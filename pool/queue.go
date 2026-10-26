package pool

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/panjf2000/ants/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"github.com/t2bot/matrix-media-repo/common"
	"github.com/t2bot/matrix-media-repo/common/logging"
	"github.com/t2bot/matrix-media-repo/metrics"
)

// At most (workers * maxWaitersPerWorker) callers may wait for a free worker; beyond that we shed load.
const maxWaitersPerWorker = 4
const scheduleRetryInterval = 25 * time.Millisecond

type Queue struct {
	name    string
	pool    *ants.Pool
	waiting atomic.Int64
}

func NewQueue(workers int, name string) (*Queue, error) {
	p, err := ants.NewPool(workers, ants.WithOptions(ants.Options{
		ExpiryDuration:   1 * time.Minute, // worker lifespan when unused
		PreAlloc:         false,
		MaxBlockingTasks: 0,
		Nonblocking:      true, // waiting is done (bounded and context-aware) in Schedule/ScheduleContext
		PanicHandler: func(err interface{}) {
			metrics.PoolPanics.With(prometheus.Labels{"pool": name}).Inc()
			logrus.Errorf("Panic from internal queue %s", name)
			logrus.Error(err)
			//goland:noinspection GoTypeAssertionOnErrors
			if e, ok := err.(error); ok {
				sentry.CaptureException(e)
			}
		},
		Logger:       &logging.SendToDebugLogger{},
		DisablePurge: false,
	}))
	if err != nil {
		return nil, err
	}
	metrics.RegisterPoolGauges(name,
		func() float64 { return float64(p.Running()) },
		func() float64 { return float64(p.Cap()) },
	)
	return &Queue{name: name, pool: p}, nil
}

func (p *Queue) overloaded() error {
	metrics.PoolOverloads.With(prometheus.Labels{"pool": p.name}).Inc()
	return common.ErrPoolOverloaded
}

// Schedule waits (without bound) until a worker accepts the task, preserving the pre-existing semantics for
// background tasks. Request paths must use ScheduleContext instead.
func (p *Queue) Schedule(task func()) error {
	for {
		err := p.pool.Submit(task)
		if !errors.Is(err, ants.ErrPoolOverload) {
			return err // nil, or ErrPoolClosed during a reload
		}
		time.Sleep(scheduleRetryInterval)
	}
}

// ScheduleContext waits for a free worker until ctx ends. If too many callers are already waiting it fails
// immediately. Both failure modes return an error satisfying errors.Is(err, common.ErrPoolOverloaded).
func (p *Queue) ScheduleContext(ctx context.Context, task func()) error {
	err := p.pool.Submit(task)
	if !errors.Is(err, ants.ErrPoolOverload) {
		return err
	}

	if p.waiting.Add(1) > int64(p.pool.Cap()*maxWaitersPerWorker) {
		p.waiting.Add(-1)
		return p.overloaded()
	}
	gauge := metrics.PoolWaiting.With(prometheus.Labels{"pool": p.name})
	gauge.Inc()
	defer func() {
		p.waiting.Add(-1)
		gauge.Dec()
	}()

	delay := 10 * time.Millisecond
	timer := time.NewTimer(delay)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("%w: gave up waiting for a free worker in %s: %v", p.overloaded(), p.name, ctx.Err())
		case <-timer.C:
			err = p.pool.Submit(task)
			if !errors.Is(err, ants.ErrPoolOverload) {
				return err
			}
			if delay < 200*time.Millisecond {
				delay *= 2
			}
			timer.Reset(delay)
		}
	}
}
