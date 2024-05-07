package multiflo

import (
	"context"
	"errors"
	"sync"
	"time"
)

type MultiFlo struct {
	claimAttempts chan claimAttempt

	flushFrame <-chan time.Time

	// buckets["username"]["joerodriguez"] tracks tokens for a specific user
	buckets map[string]map[string]bucket

	empty  sync.WaitGroup
	finish chan struct{}

	defaultNextFramer *time.Ticker
}

func New(opts ...Option) *MultiFlo {
	ticker := time.NewTicker(time.Second)

	flo := &MultiFlo{
		claimAttempts: make(chan claimAttempt),

		flushFrame: ticker.C,
		buckets:    map[string]map[string]bucket{},

		finish: make(chan struct{}),

		defaultNextFramer: ticker,
	}

	for _, opt := range opts {
		opt(flo)
	}

	go flo.Start()

	return flo
}

type ReleaseFunc func(error)

type bucket struct {
	desiredClaimLimit int64
	actualClaims      int64

	currentFrame frame
	lastFrame    frame

	// An aggregate of all frames
	baselineFrame frame
}

type frame struct {
	attempts int64
	failures int64
	latency  time.Duration
}

func (f *MultiFlo) Start() {
	for {
		select {
		case <-f.finish:
			return
		case <-f.flushFrame:
			// TODO: flush frames, update claim limits
		case attempt := <-f.claimAttempts:
			f.empty.Add(1)

			attempt.answer <- func(err error) {
				f.empty.Done()
			}
		}
	}
}

var ErrInadmissible = errors.New("inadmissible")

type claimAttempt struct {
	fairnessAttributes map[string]string
	answer             chan ReleaseFunc
}

func (f *MultiFlo) Claim(ctx context.Context, fairnessAttributes map[string]string) (ReleaseFunc, error) {

	answer := make(chan ReleaseFunc)
	defer close(answer)

	f.claimAttempts <- claimAttempt{
		fairnessAttributes: fairnessAttributes,
		answer:             answer,
	}

	select {

	case <-ctx.Done():
		return nil, ctx.Err()

	case release := <-answer:
		if release == nil {
			return nil, ErrInadmissible
		}

		return release, nil
	}
}

func (f *MultiFlo) Close() {
	f.empty.Wait()
	f.defaultNextFramer.Stop()
	f.finish <- struct{}{}

	close(f.claimAttempts)
	close(f.finish)
}

type Option func(*MultiFlo)

func WithNextFramer(nextFrame <-chan time.Time) Option {
	return func(f *MultiFlo) {
		f.flushFrame = nextFrame
	}
}
