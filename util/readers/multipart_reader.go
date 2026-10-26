package readers

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/textproto"
	"net/url"

	"github.com/alioygur/is"
)

type MultipartPart struct {
	ContentType string
	FileName    string
	Location    string
	Reader      io.ReadCloser
}

func NewMultipartReader(boundary string, parts ...*MultipartPart) io.ReadCloser {
	r, w := io.Pipe()
	go func() {
		// Parts [closedUpTo:] have not been closed yet. Whatever way we exit (error, consumer closed the pipe,
		// success), release them all so e.g. S3 objects don't leak.
		closedUpTo := 0
		defer func() {
			for _, p := range parts[closedUpTo:] {
				if p != nil && p.Reader != nil {
					_ = p.Reader.Close()
				}
			}
		}()

		mpw := multipart.NewWriter(w)
		err := mpw.SetBoundary(boundary)
		if err != nil {
			_ = w.CloseWithError(err)
			return
		}

		for i, part := range parts {
			headers := textproto.MIMEHeader{}
			if part.ContentType != "" {
				headers.Set("Content-Type", part.ContentType)
			}
			if part.FileName != "" {
				if is.ASCII(part.FileName) {
					headers.Set("Content-Disposition", "attachment; filename="+url.QueryEscape(part.FileName))
				} else {
					headers.Set("Content-Disposition", "attachment; filename*=utf-8''"+url.QueryEscape(part.FileName))
				}
			}
			if part.Location != "" {
				headers.Set("Location", part.Location)
				if part.Reader != nil {
					_ = part.Reader.Close() // we won't read it, so don't orphan it
				}
				part.Reader = io.NopCloser(bytes.NewReader(make([]byte, 0)))
			}
			if part.Reader == nil {
				part.Reader = io.NopCloser(bytes.NewReader(make([]byte, 0)))
			}

			partW, err := mpw.CreatePart(headers)
			if err != nil {
				_ = w.CloseWithError(err)
				return
			}
			if _, err = io.Copy(partW, part.Reader); err != nil {
				_ = w.CloseWithError(err)
				return // deferred cleanup closes parts[i:]
			}
			closedUpTo = i + 1
			_ = part.Reader.Close()
		}

		if err := mpw.Close(); err != nil {
			_ = w.CloseWithError(err)
		}
		_ = w.Close()
	}()
	return MakeCloser(r)
}
