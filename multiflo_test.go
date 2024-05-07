package multiflo_test

import (
	"context"
	"errors"
	"math/rand"
	"multiflo"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func Test(t *testing.T) {
	t.Run("if nothing is erroring, everything is admissible", func(t *testing.T) {
		errs := make(chan error, 1)
		defer close(errs)

		flo := multiflo.New()

		allJobsStarted := sync.WaitGroup{}
		allJobsStarted.Add(1000)
		numCompleted := atomic.Int64{}

		for i := 0; i < 1000; i++ {
			go func() {
				ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Millisecond))
				defer cancel()

				release, err := flo.Claim(ctx, map[string]string{
					"key": pick("a", "b", "c"),
				})
				if err != nil {
					errs <- err
					return
				}

				allJobsStarted.Done()
				time.Sleep(time.Second)
				release(nil)
				numCompleted.Add(1)
			}()
		}

		allJobsStarted.Wait()
		flo.Close()

		if numCompleted.Load() != 1000 {
			t.Errorf("Expected 1000 completions, got %d", numCompleted.Load())
		}

		select {
		case err := <-errs:
			t.Errorf("Unexpected error: %v", err)
		case <-time.After(time.Second):
		}
	})

	t.Run("if a single key is failing, it's limited in the next frame", func(t *testing.T) {
		admissed, rejected, pass, fail, noop := harness(t)

		ctx := context.Background()

		nextFrame := make(chan time.Time)
		flo := multiflo.New(multiflo.WithNextFramer(nextFrame))

		admissed(fail(
			flo.Claim(ctx, map[string]string{
				"ip": "192.168.1.1",
			}),
		))

		admissed(fail(
			flo.Claim(ctx, map[string]string{
				"ip": "192.168.1.1",
			}),
		))

		nextFrame <- time.Now()

		rejected(noop(
			flo.Claim(ctx, map[string]string{
				"ip":   "192.168.1.1",
				"user": "jdoe",
			}),
		))

		admissed(pass(
			flo.Claim(ctx, map[string]string{
				"user": "jdoe",
			}),
		))

		nextFrame <- time.Now()

		admissed(pass(
			flo.Claim(ctx, map[string]string{
				"ip":   "192.168.1.1",
				"user": "jdoe",
			}),
		))
	})
}

func harness(t *testing.T) (admissed, rejected func(error), pass, fail, noop func(release multiflo.ReleaseFunc, err error) error) {

	admissed = func(err error) {
		if err != nil {
			t.Errorf("Expected the claim to be admissed, but got rejection: %v", err)
		}
	}

	rejected = func(err error) {
		if err == nil {
			t.Errorf("Expected the claim to be rejected, but it was admissed")
		}
	}

	pass = func(release multiflo.ReleaseFunc, err error) error {
		release(nil)

		return err
	}

	fail = func(release multiflo.ReleaseFunc, err error) error {
		release(errors.New("failed"))

		return err
	}

	noop = func(release multiflo.ReleaseFunc, err error) error {
		return err
	}

	return
}

func pick(s ...string) string {
	return s[rand.Intn(len(s))]
}
