package datastore_op

import (
	"errors"
	"io"
	"sync"

	"github.com/t2bot/matrix-media-repo/common/config"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/database"
	"github.com/t2bot/matrix-media-repo/datastores"
	"github.com/t2bot/matrix-media-repo/pipelines/pipeline_upload"
)

type downloadResult struct {
	r           io.ReadCloser
	filename    string
	contentType string
	err         error
}

type uploadResult struct {
	m   *database.DbMedia
	err error
}

type bufferTempFn = func(datastore config.DatastoreConfig, contents io.ReadCloser) (string, int64, io.ReadCloser, error)
type uploadExecuteFn = func(ctx rcontext.RequestContext, origin string, mediaId string, r io.ReadCloser, contentType string, fileName string, userId string, kind datastores.Kind) (*database.DbMedia, error)

func PutAndReturnStream(ctx rcontext.RequestContext, origin string, mediaId string, input io.ReadCloser, contentType string, fileName string, kind datastores.Kind) (*database.DbMedia, io.ReadCloser, error) {
	// We own `input` for the whole call, whatever happens (including a failure to pick a datastore).
	defer func() {
		if input != nil {
			_ = input.Close()
		}
	}()

	dsConf, err := datastores.Pick(ctx, kind)
	if err != nil {
		return nil, nil, err
	}
	return putAndReturnStream(ctx, dsConf, origin, mediaId, input, contentType, fileName, kind, datastores.BufferTemp, pipeline_upload.Execute)
}

func putAndReturnStream(ctx rcontext.RequestContext, dsConf config.DatastoreConfig, origin string, mediaId string, input io.ReadCloser, contentType string, fileName string, kind datastores.Kind, bufferTemp bufferTempFn, uploadExecute uploadExecuteFn) (*database.DbMedia, io.ReadCloser, error) {
	pr, pw := io.Pipe()
	tee := io.TeeReader(input, pw)
	defer func(pw *io.PipeWriter, err error) {
		_ = pw.CloseWithError(err)
	}(pw, errors.New("failed to finish write"))

	wg := new(sync.WaitGroup)
	wg.Add(2)

	bufferCh := make(chan downloadResult)
	uploadCh := make(chan uploadResult)
	defer close(bufferCh)
	defer close(uploadCh)

	upstreamClose := func() error { return pw.Close() }

	go func(dsConf config.DatastoreConfig, pr io.ReadCloser, bufferCh chan downloadResult) {
		_, _, retReader, err2 := bufferTemp(dsConf, pr)
		if err2 != nil {
			// Nobody reads pr any more. Close it so the tee's pw.Write fails instead of blocking the upload
			// goroutine (and therefore wg.Wait() and the singleflight key) forever.
			_ = pr.Close()
		}
		// async the channel update to avoid deadlocks
		go func(bufferCh chan downloadResult, err2 error, retReader io.ReadCloser) {
			bufferCh <- downloadResult{err: err2, r: retReader}
		}(bufferCh, err2, retReader)
		wg.Done()
	}(dsConf, pr, bufferCh)

	go func(ctx rcontext.RequestContext, origin string, mediaId string, r io.ReadCloser, upstreamClose func() error, contentType string, fileName string, kind datastores.Kind, uploadCh chan uploadResult) {
		m, err2 := uploadExecute(ctx, origin, mediaId, r, contentType, fileName, "", kind)
		// async the channel update to avoid deadlocks
		go func(uploadCh chan uploadResult, err2 error, m *database.DbMedia) {
			uploadCh <- uploadResult{err: err2, m: m}
		}(uploadCh, err2, m)
		if err3 := upstreamClose(); err3 != nil {
			ctx.Log.Warn("Failed to close non-tee writer during remote download: ", err3)
		}
		wg.Done()
	}(ctx, origin, mediaId, io.NopCloser(tee), upstreamClose, contentType, fileName, kind, uploadCh)

	wg.Wait()
	bufferRes := <-bufferCh
	uploadRes := <-uploadCh
	if bufferRes.err != nil {
		return nil, nil, bufferRes.err
	}
	if uploadRes.err != nil {
		if bufferRes.r != nil {
			defer bufferRes.r.Close()
		}
		return nil, nil, uploadRes.err
	}

	return uploadRes.m, bufferRes.r, nil
}
