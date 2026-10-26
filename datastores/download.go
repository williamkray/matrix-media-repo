package datastores

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"sync"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/t2bot/matrix-media-repo/common/config"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/metrics"
)

// s3ObjectMaxLifetime is a safety net: even if a consumer stalls, the object's requests are aborted after this long.
const s3ObjectMaxLifetime = 2 * time.Hour

func Download(ctx rcontext.RequestContext, ds config.DatastoreConfig, dsFileName string) (io.ReadSeekCloser, error) {
	if ds.Type == "s3" {
		s3c, err := getS3(ds)
		if err != nil {
			return nil, err
		}

		metrics.S3Operations.With(prometheus.Labels{"operation": "GetObject"}).Inc()

		// The returned object is shared between singleflight followers and keeps using this context for later
		// seeks, so it must not die with the leader's request. Its lifetime is governed by Close() instead.
		objCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx.Context), s3ObjectMaxLifetime)
		obj, err := s3c.client.GetObject(objCtx, s3c.bucket, dsFileName, minio.GetObjectOptions{})
		if err != nil {
			cancel()
			if obj != nil {
				_ = obj.Close()
			}
			return nil, err // explicit nil: never return a typed-nil *minio.Object inside the interface
		}
		return newTrackedObject(obj, cancel), nil
	} else if ds.Type == "file" {
		basePath := ds.Options["path"]
		f, err := os.Open(path.Join(basePath, dsFileName))
		if err != nil {
			return nil, err
		}
		return f, nil
	}
	return nil, errors.New("unknown datastore type - contact developer")
}

// trackedObject counts open S3 objects (metrics.OpenS3Objects) and makes Close idempotent. minio-go's Object holds
// a goroutine until Close() and returns an error on a second Close.
type trackedObject struct {
	io.ReadSeekCloser
	cancel context.CancelFunc
	once   sync.Once
	err    error
}

func newTrackedObject(rsc io.ReadSeekCloser, cancel context.CancelFunc) *trackedObject {
	metrics.OpenS3Objects.Inc()
	return &trackedObject{ReadSeekCloser: rsc, cancel: cancel}
}

func (t *trackedObject) Close() error {
	t.once.Do(func() {
		t.err = t.ReadSeekCloser.Close()
		if t.cancel != nil {
			t.cancel()
		}
		metrics.OpenS3Objects.Dec()
	})
	return t.err
}

func DownloadOrRedirect(ctx rcontext.RequestContext, ds config.DatastoreConfig, dsFileName string) (io.ReadSeekCloser, error) {
	if ds.Type != "s3" {
		return Download(ctx, ds, dsFileName)
	}

	s3c, err := getS3(ds)
	if err != nil {
		return nil, err
	}

	if s3c.publicBaseUrl != "" {
		metrics.S3Operations.With(prometheus.Labels{"operation": "RedirectGetObject"}).Inc()
		return nil, redirect(fmt.Sprintf("%s%s", s3c.publicBaseUrl, dsFileName))
	}

	return Download(ctx, ds, dsFileName)
}

func WouldRedirectWhenCached(ctx rcontext.RequestContext, ds config.DatastoreConfig) (bool, error) {
	if ds.Type != "s3" {
		return false, nil
	}

	s3c, err := getS3(ds)
	if err != nil {
		return false, err
	}

	return s3c.redirectWhenCached && s3c.publicBaseUrl != "", nil
}
