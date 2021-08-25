package retry

import (
	"context"
	"math/rand"
	"time"
)

type Retry func(context.Context) <-chan struct{}

func Fixed(d time.Duration) Retry {
	return func(ctx context.Context) <-chan struct{} {
		sig := make(chan struct{})
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				default:
					time.Sleep(d)
					sig <- struct{}{}
				}
			}
		}()
		return sig
	}
}

func ExponentialBackoff(base, randomHighLevel, max time.Duration) Retry {
	return func(ctx context.Context) <-chan struct{} {
		if base <= time.Millisecond {
			base = time.Millisecond
		}
		if max > 64*time.Second {
			max = 64 * time.Second
		}
		sig := make(chan struct{})
		go func() {
			var (
				cnt  int64
				last int64
				r    = rand.NewSource(time.Now().UnixNano())
			)
			for {
				select {
				case <-ctx.Done():
					return
				default:
					if last < int64(max) {
						cnt++
						last = ((1 << cnt) - 1) / 2
					}
					if last > int64(max) {
						last = int64(max)
					} else {
						st := time.Duration(int64(base)*last + r.Int63()%int64(randomHighLevel))
						if st > max {
							st = max
						}
						time.Sleep(st)
						sig <- struct{}{}
					}
				}
			}
		}()
		return sig
	}
}
