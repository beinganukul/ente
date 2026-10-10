package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/ente/museum/pkg/utils/config"
	"github.com/ente/museum/pkg/utils/database"
	"github.com/ente/museum/pkg/utils/s3config"
	spacerepo "github.com/ente/museum/space/repo"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

type cleanupSubrequestLimit struct {
	base   http.RoundTripper
	active atomic.Bool
	count  atomic.Int32
}

func (c *cleanupSubrequestLimit) RoundTrip(request *http.Request) (*http.Response, error) {
	if c.active.Load() && c.count.Add(1) > 50 {
		return nil, fmt.Errorf("Workers Free subrequest limit exceeded")
	}
	return c.base.RoundTrip(request)
}

func TestWorkerCleanupDrainsBacklogWithinSubrequestLimit(t *testing.T) {
	_, repos, ctx := setupPostsControllerTest(t)
	if !database.IsTurso(repos.Assets.DB) {
		t.Skip("HTTP subrequest accounting requires disposable libSQL")
	}
	viper.Reset()
	t.Cleanup(viper.Reset)
	require.NoError(t, config.ConfigureViper("local"))
	var deleted atomic.Int32
	storage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodDelete {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		deleted.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(storage.Close)
	viper.Set("s3.b2-eu-cen.key", "test-key")
	viper.Set("s3.b2-eu-cen.secret", "test-secret")
	viper.Set("s3.b2-eu-cen.endpoint", storage.URL)
	viper.Set("s3.b2-eu-cen.region", "us-east-1")
	viper.Set("s3.b2-eu-cen.bucket", "test-bucket")
	viper.Set("s3.b2-eu-cen.use_path_style_urls", true)
	transport := &cleanupSubrequestLimit{base: http.DefaultTransport}
	original := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = original })
	repos.Assets.S3Config = s3config.NewS3Config()
	for i := 0; i < 30; i++ {
		require.NoError(t, repos.Assets.AddTempObject(ctx, spacerepo.SpaceTempObjectRecord{
			ObjectKey: fmt.Sprintf("worker-cleanup-backlog-%d", i), Purpose: spacerepo.TempObjectPurposePost,
			BucketID: "b2-eu-cen", ExpiresAt: 1, ExpectedSize: 123,
		}))
	}
	cleanup := &CleanupController{AssetsRepo: repos.Assets}
	for remaining := 30; remaining > 0; remaining -= spaceTempObjectWorkerCleanupBatchSize {
		transport.count.Store(0)
		transport.active.Store(true)
		removed := cleanup.removeUnreportedObjectsBatch(ctx, spaceTempObjectWorkerCleanupBatchSize)
		transport.active.Store(false)
		require.Equal(t, spaceTempObjectWorkerCleanupBatchSize, removed)
		require.LessOrEqual(t, transport.count.Load(), int32(50))
		t.Logf("cleanup step: %d HTTP subrequests", transport.count.Load())
		var count int
		require.NoError(t, repos.Assets.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM space_temp_objects`).Scan(&count))
		require.Equal(t, remaining-spaceTempObjectWorkerCleanupBatchSize, count)
	}
	require.Equal(t, int32(30), deleted.Load())
}
