package pool

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/t2bot/matrix-media-repo/common"
	"github.com/t2bot/matrix-media-repo/metrics"
)

func waitingGauge(t *testing.T, name string) float64 {
	m := &dto.Metric{}
	require.NoError(t, metrics.PoolWaiting.With(prometheus.Labels{"pool": name}).Write(m))
	return m.GetGauge().GetValue()
}

func TestQueueOverload(t *testing.T) {
	q, err := NewQueue(1, "test")
	require.NoError(t, err)

	release := make(chan struct{})
	started := make(chan struct{})
	require.NoError(t, q.Schedule(func() { close(started); <-release }))
	<-started

	// Schedule blocks while busy, then succeeds after release
	schedDone := make(chan error, 1)
	ran := make(chan struct{})
	go func() { schedDone <- q.Schedule(func() { close(ran) }) }()
	select {
	case <-schedDone:
		t.Fatal("Schedule returned while worker was busy")
	case <-time.After(100 * time.Millisecond):
	}

	// (b) ScheduleContext times out
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	start := time.Now()
	err = q.ScheduleContext(ctx, func() {})
	cancel()
	assert.True(t, errors.Is(err, common.ErrPoolOverloaded))
	assert.GreaterOrEqual(t, time.Since(start), 90*time.Millisecond)
	assert.Less(t, time.Since(start), 2*time.Second)

	// (c) fill waiters; next fails fast
	longCtx, longCancel := context.WithCancel(context.Background())
	results := make(chan error, maxWaitersPerWorker)
	for i := 0; i < maxWaitersPerWorker; i++ {
		go func() { results <- q.ScheduleContext(longCtx, func() {}) }()
	}
	require.Eventually(t, func() bool { return waitingGauge(t, "test") == float64(maxWaitersPerWorker) }, 2*time.Second, 5*time.Millisecond)
	start = time.Now()
	err = q.ScheduleContext(context.Background(), func() {})
	assert.True(t, errors.Is(err, common.ErrPoolOverloaded))
	assert.Less(t, time.Since(start), 50*time.Millisecond)

	// cancel the waiters; they all fail and the gauge returns to 0
	longCancel()
	for i := 0; i < maxWaitersPerWorker; i++ {
		select {
		case e := <-results:
			assert.True(t, errors.Is(e, common.ErrPoolOverloaded))
		case <-time.After(2 * time.Second):
			t.Fatal("waiter did not return")
		}
	}
	assert.Equal(t, float64(0), waitingGauge(t, "test"))

	// (d) release; blocked Schedule succeeds, and ScheduleContext succeeds too
	close(release)
	select {
	case err = <-schedDone:
		assert.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Schedule did not return after release")
	}
	select {
	case <-ran:
	case <-time.After(2 * time.Second):
		t.Fatal("scheduled task did not run")
	}
	done := make(chan struct{})
	ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel2()
	require.NoError(t, q.ScheduleContext(ctx2, func() { close(done) }))
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("task did not run")
	}
	assert.Equal(t, float64(0), waitingGauge(t, "test"))
}
