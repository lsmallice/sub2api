package service

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
)

const (
	openAIImagesPublicPathPrefix = "/images/smallice/"
	openAIImagesPublicURLTTL     = time.Hour
	openAIImagesCleanupInterval  = 15 * time.Minute
	openAIImagesStoreDir         = "generated-images"
)

type openAIImagesPublicStore struct {
	rootDir       string
	signingKey    []byte
	publicBaseURL string
	mu            sync.Mutex
	lastCleanup   time.Time
}

func newOpenAIImagesPublicStore(cfg *config.Config) *openAIImagesPublicStore {
	secret := ""
	publicBaseURL := strings.TrimSpace(os.Getenv("OPENAI_IMAGES_PUBLIC_BASE_URL"))
	if cfg != nil {
		secret = strings.TrimSpace(cfg.JWT.Secret)
		if publicBaseURL == "" {
			publicBaseURL = strings.TrimSpace(cfg.Gateway.OpenAIImagesPublicBaseURL)
		}
	}
	if secret == "" {
		secret = "sub2api-image-url-development-secret"
	}
	return &openAIImagesPublicStore{
		rootDir:       filepath.Join(resolveOpenAIImagesDataDir(), "images", "smallice"),
		signingKey:    []byte(secret),
		publicBaseURL: normalizeOpenAIImagesPublicBaseURL(publicBaseURL),
	}
}

func (s *openAIImagesPublicStore) saveBase64(c *gin.Context, b64 string, outputFormat string, createdAt time.Time) (string, error) {
	if s == nil {
		return "", fmt.Errorf("image public store is not configured")
	}
	s.cleanupExpired()
	normalized := normalizeOpenAIImageBase64(b64)
	if normalized == "" {
		return "", fmt.Errorf("empty image base64")
	}
	data, err := base64.StdEncoding.DecodeString(normalized)
	if err != nil {
		return "", fmt.Errorf("decode image base64: %w", err)
	}

	ext := openAIImageOutputExtension(outputFormat, data)
	name := createdAt.UTC().Format("20060102-150405") + "-" + randomPublicImageSuffix() + "." + ext
	relPath := filepath.ToSlash(filepath.Join(openAIImagesStoreDir, name))
	fullPath := filepath.Join(s.rootDir, relPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return "", fmt.Errorf("create image directory: %w", err)
	}
	if err := os.WriteFile(fullPath, data, 0644); err != nil {
		return "", fmt.Errorf("write image file: %w", err)
	}
	return s.publicURL(c, relPath, time.Now().Add(openAIImagesPublicURLTTL)), nil
}

func (s *openAIImagesPublicStore) serve(c *gin.Context) {
	if s == nil {
		c.Status(http.StatusNotFound)
		return
	}
	s.cleanupExpired()
	relPath := strings.TrimPrefix(c.Param("path"), "/")
	if relPath == "" || !s.validSignature(relPath, c.Query("expires"), c.Query("sig")) {
		c.Status(http.StatusNotFound)
		return
	}
	clean := filepath.Clean(relPath)
	if clean == "." || strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
		c.Status(http.StatusNotFound)
		return
	}
	fullPath := filepath.Join(s.rootDir, clean)
	info, err := os.Stat(fullPath)
	if err != nil || info.IsDir() {
		c.Status(http.StatusNotFound)
		return
	}
	c.Header("Cache-Control", "private, max-age=3600")
	c.File(fullPath)
}

func (s *openAIImagesPublicStore) cleanupExpired() {
	if s == nil {
		return
	}
	now := time.Now()
	s.mu.Lock()
	if !s.lastCleanup.IsZero() && now.Sub(s.lastCleanup) < openAIImagesCleanupInterval {
		s.mu.Unlock()
		return
	}
	s.lastCleanup = now
	rootDir := s.rootDir
	s.mu.Unlock()

	_ = filepath.WalkDir(rootDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry == nil || entry.IsDir() {
			return nil
		}
		info, statErr := entry.Info()
		if statErr != nil {
			return nil
		}
		if now.Sub(info.ModTime()) > openAIImagesPublicURLTTL {
			_ = os.Remove(path)
		}
		return nil
	})
	_ = removeEmptyOpenAIImageDirs(filepath.Join(rootDir, openAIImagesStoreDir), rootDir)
}

func removeEmptyOpenAIImageDirs(dir string, stop string) error {
	dir = filepath.Clean(dir)
	stop = filepath.Clean(stop)
	if dir == "." || dir == stop || !strings.HasPrefix(dir, stop) {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	for _, entry := range entries {
		if entry.IsDir() {
			_ = removeEmptyOpenAIImageDirs(filepath.Join(dir, entry.Name()), stop)
		}
	}
	entries, err = os.ReadDir(dir)
	if err != nil || len(entries) > 0 {
		return nil
	}
	_ = os.Remove(dir)
	parent := filepath.Dir(dir)
	if parent == dir || parent == stop {
		return nil
	}
	return removeEmptyOpenAIImageDirs(parent, stop)
}

func (s *openAIImagesPublicStore) publicURL(c *gin.Context, relPath string, expiresAt time.Time) string {
	path := openAIImagesPublicPathPrefix + strings.TrimPrefix(filepath.ToSlash(relPath), "/")
	expires := strconv.FormatInt(expiresAt.Unix(), 10)
	u := url.URL{Path: path}
	q := u.Query()
	q.Set("expires", expires)
	q.Set("sig", s.sign(path, expires))
	u.RawQuery = q.Encode()
	base := s.publicBaseURL
	if base == "" {
		base = resolveOpenAIImagesPublicBaseURL(c)
	}
	if base == "" {
		return u.String()
	}
	return strings.TrimRight(base, "/") + u.String()
}

func (s *openAIImagesPublicStore) validSignature(relPath string, expiresRaw string, sig string) bool {
	path := openAIImagesPublicPathPrefix + strings.TrimPrefix(filepath.ToSlash(relPath), "/")
	expires, err := strconv.ParseInt(strings.TrimSpace(expiresRaw), 10, 64)
	if err != nil || expires <= time.Now().Unix() {
		return false
	}
	got, err := hex.DecodeString(strings.TrimSpace(sig))
	if err != nil {
		return false
	}
	want, err := hex.DecodeString(s.sign(path, strconv.FormatInt(expires, 10)))
	if err != nil {
		return false
	}
	return hmac.Equal(got, want)
}

func (s *openAIImagesPublicStore) sign(path string, expires string) string {
	mac := hmac.New(sha256.New, s.signingKey)
	_, _ = mac.Write([]byte(path))
	_, _ = mac.Write([]byte("\n"))
	_, _ = mac.Write([]byte(expires))
	return hex.EncodeToString(mac.Sum(nil))
}

func resolveOpenAIImagesDataDir() string {
	if dir := strings.TrimSpace(os.Getenv("DATA_DIR")); dir != "" {
		return dir
	}
	if info, err := os.Stat("/app/data"); err == nil && info.IsDir() {
		return "/app/data"
	}
	return "."
}

func resolveOpenAIImagesPublicBaseURL(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return ""
	}
	proto := firstForwardedHeaderValue(c.GetHeader("X-Forwarded-Proto"))
	if proto == "" {
		if c.Request.TLS != nil {
			proto = "https"
		} else {
			proto = "http"
		}
	}
	host := firstForwardedHeaderValue(c.GetHeader("X-Forwarded-Host"))
	if host == "" {
		host = strings.TrimSpace(c.Request.Host)
	}
	if host == "" || proto == "" {
		return ""
	}
	return strings.ToLower(proto) + "://" + host
}

func normalizeOpenAIImagesPublicBaseURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return ""
	}
	parsed.Scheme = scheme
	parsed.Path = strings.TrimRight(parsed.EscapedPath(), "/")
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return strings.TrimRight(parsed.String(), "/")
}

func firstForwardedHeaderValue(value string) string {
	value = strings.TrimSpace(value)
	if idx := strings.Index(value, ","); idx >= 0 {
		value = strings.TrimSpace(value[:idx])
	}
	return value
}

func randomPublicImageSuffix() string {
	var b [5]byte
	if _, err := rand.Read(b[:]); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return strings.ToUpper(hex.EncodeToString(b[:]))
}

func openAIImageOutputExtension(outputFormat string, data []byte) string {
	format := strings.ToLower(strings.TrimSpace(outputFormat))
	format = strings.TrimPrefix(format, "image/")
	switch format {
	case "png", "webp", "jpg":
		return format
	case "jpeg":
		return "jpg"
	}
	if len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return "webp"
	}
	if len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff {
		return "jpg"
	}
	if len(data) >= 8 && string(data[:8]) == "\x89PNG\r\n\x1a\n" {
		return "png"
	}
	return "png"
}
