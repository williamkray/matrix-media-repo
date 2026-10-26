package readers

import (
	"io"
	"sync"
)

type CancellableCloser interface {
	io.ReadCloser
}

type CancelCloser struct {
	io.ReadCloser
	cancel func()
	once   sync.Once
	err    error
}

type CancelSeekCloser struct {
	io.ReadSeekCloser
	cancel func()
	once   sync.Once
	err    error
}

func NewCancelCloser(r io.ReadCloser, cancel func()) CancellableCloser {
	if rsc, ok := r.(io.ReadSeekCloser); ok {
		return &CancelSeekCloser{
			ReadSeekCloser: rsc,
			cancel:         cancel,
		}
	} else {
		return &CancelCloser{
			ReadCloser: r,
			cancel:     cancel,
		}
	}
}

// Close cancels and closes the underlying reader exactly once; later calls return the first result. This matters
// because singleflight-stream seekers panic when closed twice.
func (c *CancelCloser) Close() error {
	c.once.Do(func() {
		if c.cancel != nil {
			c.cancel()
		}
		if c.ReadCloser != nil {
			c.err = c.ReadCloser.Close()
		}
	})
	return c.err
}

func (c *CancelSeekCloser) Close() error {
	c.once.Do(func() {
		if c.cancel != nil {
			c.cancel()
		}
		if c.ReadSeekCloser != nil {
			c.err = c.ReadSeekCloser.Close()
		}
	})
	return c.err
}
