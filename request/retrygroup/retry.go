package retrygroup

import (
	"context"
	"errors"
	"sync"
)

var ErrKilled = errors.New("task has done")

type RetryGroup struct {
	task func() (interface{}, error)

	succOnce sync.Once
	done     chan struct{}
	cancel   func()

	val interface{}
	err error

	opt option
}

type option struct {
	ehandler func(error)
}

type Option func(*option)

func WithErrHandler(handler func(error)) Option {
	return func(o *option) {
		o.ehandler = handler
	}
}

func NewRetryGroup(ctx context.Context, task func() (interface{}, error), opts ...Option) (*RetryGroup, context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	ret := &RetryGroup{cancel: cancel, task: task, done: make(chan struct{})}
	for _, apply := range opts {
		apply(&ret.opt)
	}
	return ret, ctx
}

func (g *RetryGroup) Go() {
	go func() {
		if g.task == nil {
			panic("no task in retry group")
		}

		done := false
		rst, err := g.task()
		g.succOnce.Do(func() {
			g.val = rst
			g.err = err

			select {
			case <-g.done:
				break
			default:
				close(g.done)
				if g.cancel != nil {
					g.cancel()
				}
			}

			done = true
		})

		if done {
			return
		}

		if g.opt.ehandler != nil {
			g.opt.ehandler(err)
		}
	}()
}

func (g *RetryGroup) Wait() (interface{}, error) {
	if g.done == nil {
		panic("invoke Go func first")
	}

	<-g.done
	return g.val, g.err
}

func (g *RetryGroup) Kill() error {
	if g.done == nil {
		panic("invoke Go func first")
	}

	select {
	case <-g.done:
		return ErrKilled
	default:
		close(g.done)
		if g.cancel != nil {
			g.cancel()
		}
	}

	return nil
}
