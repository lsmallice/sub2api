package service

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type updateServiceCacheStub struct {
	data string
}

func (s *updateServiceCacheStub) GetUpdateInfo(context.Context) (string, error) {
	if s.data == "" {
		return "", errors.New("cache miss")
	}
	return s.data, nil
}

func (s *updateServiceCacheStub) SetUpdateInfo(_ context.Context, data string, _ time.Duration) error {
	s.data = data
	return nil
}

type updateServiceGitHubClientStub struct {
	release *GitHubRelease
	err     error
}

func (s *updateServiceGitHubClientStub) FetchLatestRelease(context.Context, string) (*GitHubRelease, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.release, nil
}

func (s *updateServiceGitHubClientStub) DownloadFile(context.Context, string, string, int64) error {
	panic("DownloadFile should not be called when no update is available")
}

func (s *updateServiceGitHubClientStub) FetchChecksumFile(context.Context, string) ([]byte, error) {
	panic("FetchChecksumFile should not be called when no update is available")
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
			svc := NewUpdateService(&updateServiceCacheStub{}, &updateServiceGitHubClientStub{
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
	cache := &updateServiceCacheStub{
		data: `{"latest":"0.1.132","release_info":{"name":"v0.1.132"},"timestamp":` + strconv.FormatInt(time.Now().Unix(), 10) + `}`,
	}
	svc := NewUpdateService(cache, &updateServiceGitHubClientStub{
		err: errors.New("network should not be used"),
	}, "0.1.132", "imgcap-0.1.132", "custom")

	info, err := svc.CheckUpdate(context.Background(), false)
	require.NoError(t, err)
	require.True(t, info.Cached)
	require.False(t, info.HasUpdate)
	require.Equal(t, "0.1.132", info.CurrentVersion)
	require.Equal(t, "0.1.132", info.BaseVersion)
	require.Equal(t, "imgcap-0.1.132", info.DisplayVersion)
	require.Equal(t, "imgcap-0.1.132", info.BuildLabel)
	require.Equal(t, "custom", info.BuildType)
}

func TestUpdateServicePerformUpdateNoUpdateReturnsSentinel(t *testing.T) {
	svc := NewUpdateService(
		&updateServiceCacheStub{},
		&updateServiceGitHubClientStub{
			release: &GitHubRelease{
				TagName: "v0.1.132",
				Name:    "v0.1.132",
			},
		},
		"0.1.132",
		"",
		"release",
	)

	err := svc.PerformUpdate(context.Background())

	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNoUpdateAvailable))
	require.ErrorIs(t, err, ErrNoUpdateAvailable)
}
