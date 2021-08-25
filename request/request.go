package request

import (
	"context"
	"errors"
	"time"

	"github.com/Saner-Lee/backup-request/request/retrygroup"
	"github.com/Saner-Lee/backup-request/retry"
)

var ErrNoAccess = errors.New("have no access to retry")

type Request struct {
	retry *retrygroup.RetryGroup
	ctx   context.Context
	opt   option
}

type option struct {
	event  func(context.Context) <-chan struct{}
	access func(context.Context) bool
}

type Option func(*option)

func WithEvent(fn func(context.Context) <-chan struct{}) Option {
	return func(o *option) {
		o.event = fn
	}
}

func WithAccess(fn func(context.Context) bool) Option {
	return func(o *option) {
		o.access = fn
	}
}

func NewRequest(ctx context.Context, retrygroup *retrygroup.RetryGroup, opts ...Option) (*Request, error) {
	if ctx == nil {
		panic("empty context in new request")
	}

	ret := &Request{
		ctx:   ctx,
		retry: retrygroup,
	}
	for _, apply := range opts {
		apply(&ret.opt)
	}
	if ret.opt.event == nil {
		retry.Fixed(time.Second)
	}
	return ret, nil
}

func (r *Request) Do() (interface{}, error) {
	var (
		first = make(chan struct{}, 1)
		err   = make(chan error, 1)
		event = r.opt.event(r.ctx)
	)

	go func() {
		defer func() {
			close(first)
			close(err)
		}()

		for {
			select {
			case <-r.ctx.Done():
				return
			case first <- struct{}{}:
				r.retry.Go()
			case <-event:
				for i := 0; i < 1; i++ {
					if r.opt.access == nil {
						break
					}

					if !r.opt.access(r.ctx) {
						err <- r.retry.Kill()
						return
					}
				}
				r.retry.Go()
			}
		}
	}()

	ret, rerr := r.retry.Wait()
	errgroup, ok := <-err
	if ok && errgroup == nil {
		return nil, ErrNoAccess
	}
	return ret, rerr
}

func IsKilled(err error) bool {
	return errors.Is(err, ErrNoAccess)
}
