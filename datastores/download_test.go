package datastores

import (
	"bytes"
	"io"
	"sync/atomic"
	"testing"

	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/t2bot/matrix-media-repo/metrics"
)

type fakeRSC struct {
	*bytes.Reader
	closes atomic.Int32
}

func (f *fakeRSC) Close() error {
	f.closes.Add(1)
	return nil
}

func gaugeValue(t *testing.T) float64 {
	t.Helper()
	m := &dto.Metric{}
	assert.NoError(t, metrics.OpenS3Objects.Write(m))
	return m.GetGauge().GetValue()
}

func TestTrackedObject(t *testing.T) {
	base := gaugeValue(t)
	fake := &fakeRSC{Reader: bytes.NewReader([]byte("hello"))}
	cancels := 0
	obj := newTrackedObject(fake, func() { cancels++ })
	assert.Equal(t, base+1, gaugeValue(t))

	b := make([]byte, 2)
	_, err := io.ReadFull(obj, b)
	assert.NoError(t, err)
	assert.Equal(t, "he", string(b))
	_, err = obj.Seek(0, io.SeekStart)
	assert.NoError(t, err)

	assert.NoError(t, obj.Close())
	assert.NoError(t, obj.Close())
	assert.Equal(t, base, gaugeValue(t))
	assert.EqualValues(t, 1, fake.closes.Load())
	assert.Equal(t, 1, cancels)
}
