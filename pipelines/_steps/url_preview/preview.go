package url_preview

import (
	"errors"
	"fmt"

	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/pool"
	"github.com/t2bot/matrix-media-repo/url_previewing/m"
	"github.com/t2bot/matrix-media-repo/url_previewing/p"
)

type generateResult struct {
	preview m.PreviewResult
	err     error
}

func Preview(ctx rcontext.RequestContext, targetUrl *m.UrlPayload, languageHeader string) (m.PreviewResult, error) {
	ch := make(chan generateResult, 1) // buffered and never closed
	fn := func() {
		defer func() {
			if rec := recover(); rec != nil {
				select {
				case ch <- generateResult{err: fmt.Errorf("panic while generating preview: %v", rec)}:
				default:
				}
				panic(rec) // let the pool's PanicHandler count/report it
			}
		}()
		var preview m.PreviewResult
		err := m.ErrPreviewUnsupported

		// Try oEmbed first
		if ctx.Config.UrlPreviews.OEmbed {
			ctx.Log.Debug("Trying oEmbed previewer")
			preview, err = p.GenerateOEmbedPreview(targetUrl, languageHeader, ctx)
		}

		// Try OpenGraph if that failed
		if errors.Is(err, m.ErrPreviewUnsupported) {
			ctx.Log.Debug("Trying OpenGraph previewer")
			preview, err = p.GenerateOpenGraphPreview(targetUrl, languageHeader, ctx)
		}

		// Try scraping if that failed
		if errors.Is(err, m.ErrPreviewUnsupported) {
			ctx.Log.Debug("Trying built-in previewer")
			preview, err = p.GenerateCalculatedPreview(targetUrl, languageHeader, ctx)
		}

		select {
		case ch <- generateResult{preview: preview, err: err}:
		default:
		}
	}

	if err := pool.UrlPreviewQueue.ScheduleContext(ctx.Context, fn); err != nil {
		return m.PreviewResult{}, err
	}
	var res generateResult
	select {
	case res = <-ch:
	case <-ctx.Context.Done():
		return m.PreviewResult{}, ctx.Context.Err() // worker result (no streams) is dropped into the buffer
	}
	return res.preview, res.err
}
