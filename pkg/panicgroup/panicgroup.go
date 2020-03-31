package panicgroup

import (
	"fmt"
)

type PanicGroup interface {
	Go(f func(a ...interface{})) func(a ...interface{})
}

type panicgroup struct {
	err chan error
}

func (p *panicgroup) Go(f func(a ...interface{})) func(a ...interface{}) {
	return func(a ...interface{}) {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					p.err <- fmt.Errorf("%+v", r)
				}
			}()
			f(a...)
		}()
	}
}

func NewPanicGroup(errhandle func(err error)) PanicGroup {
	pg := &panicgroup{
		err: make(chan error, 5),
	}

	pg.Go(func(a ...interface{}) {
		for {
			select {
			case err := <-pg.err:
				errhandle(err)
			}
		}
	})

	return pg
}
