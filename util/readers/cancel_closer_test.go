package readers

import (
	"bytes"
	"errors"
	"io"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

type countingCloser struct {
	io.Reader
	closes atomic.Int32
	err    error
}

func (c *countingCloser) Close() error {
	c.closes.Add(1)
	return c.err
}

type countingSeekCloser struct {
	*bytes.Reader
	closes atomic.Int32
	err    error
}

func (c *countingSeekCloser) Close() error {
	c.closes.Add(1)
	return c.err
}

func TestCancelCloserCloseOnce(t *testing.T) {
	inner := &countingCloser{Reader: bytes.NewReader(nil), err: errors.New("first")}
	var cancels int
	c := NewCancelCloser(inner, func() { cancels++ })
	assert.IsType(t, &CancelCloser{}, c)
	assert.EqualError(t, c.Close(), "first")
	assert.EqualError(t, c.Close(), "first")
	assert.EqualValues(t, 1, inner.closes.Load())
	assert.Equal(t, 1, cancels)
}

func TestCancelSeekCloserCloseOnce(t *testing.T) {
	inner := &countingSeekCloser{Reader: bytes.NewReader([]byte("abc")), err: errors.New("first")}
	var cancels int
	c := NewCancelCloser(inner, func() { cancels++ })
	assert.IsType(t, &CancelSeekCloser{}, c)
	assert.EqualError(t, c.Close(), "first")
	assert.EqualError(t, c.Close(), "first")
	assert.EqualValues(t, 1, inner.closes.Load())
	assert.Equal(t, 1, cancels)
}

func TestCancelCloserNilsDoNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		c := &CancelCloser{}
		assert.NoError(t, c.Close())
		s := &CancelSeekCloser{}
		assert.NoError(t, s.Close())
	})
}
