package service

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type updateCacheStub struct {
	data string
}

func (s *updateCacheStub) GetUpdateInfo(ctx context.Context) (string, error) {
	if s.data == "" {
		return "", errors.New("cache miss")
	}
	return s.data, nil
}

func (s *updateCacheStub) SetUpdateInfo(ctx context.Context, data string, ttl time.Duration) error {
	s.data = data
	return nil
}

type updateGitHubClientStub struct {
	release *GitHubRelease
	err     error
}

func (s *updateGitHubClientStub) FetchLatestRelease(ctx context.Context, repo string) (*GitHubRelease, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.release, nil
}

func (s *updateGitHubClientStub) DownloadFile(ctx context.Context, url, dest string, maxSize int64) error {
	return errors.New("unexpected DownloadFile call")
}

func (s *updateGitHubClientStub) FetchChecksumFile(ctx context.Context, url string) ([]byte, error) {
	return nil, errors.New("unexpected FetchChecksumFile call")
}

func TestUpdateServiceCheckUpdateUsesBaseVersionForCustomBuild(t *testing.T) {
	tests := []struct {
		name       string
		latestTag  string
		wantUpdate bool
	}{
		{
			name:       "same upstream version does not update",
			latestTag:  "v0.1.132",
			wantUpdate: false,
		},
		{
			name:       "newer upstream version updates",
			latestTag:  "v0.1.133",
			wantUpdate: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewUpdateService(&updateCacheStub{}, &updateGitHubClientStub{
				release: &GitHubRelease{TagName: tt.latestTag},
			}, "0.1.132", "imgcap-0.1.132", "custom")

			info, err := svc.CheckUpdate(context.Background(), true)
			require.NoError(t, err)
			require.Equal(t, "0.1.132", info.CurrentVersion)
			require.Equal(t, "0.1.132", info.BaseVersion)
			require.Equal(t, "imgcap-0.1.132", info.DisplayVersion)
			require.Equal(t, "imgcap-0.1.132", info.BuildLabel)
			require.Equal(t, "custom", info.BuildType)
			require.Equal(t, tt.wantUpdate, info.HasUpdate)
		})
	}
}

func TestUpdateServiceCheckUpdateCachedUsesBaseVersionForCustomBuild(t *testing.T) {
	cache := &updateCacheStub{
		data: `{"latest":"0.1.132","release_info":{"name":"v0.1.132"},"timestamp":` + strconv.FormatInt(time.Now().Unix(), 10) + `}`,
	}
	svc := NewUpdateService(cache, &updateGitHubClientStub{
		err: errors.New("network should not be used"),
	}, "0.1.132", "imgcap-0.1.132", "custom")

	info, err := svc.CheckUpdate(context.Background(), false)
	require.NoError(t, err)
	require.True(t, info.Cached)
	require.False(t, info.HasUpdate)
	require.Equal(t, "0.1.132", info.CurrentVersion)
	require.Equal(t, "imgcap-0.1.132", info.DisplayVersion)
	require.Equal(t, "imgcap-0.1.132", info.BuildLabel)
}
