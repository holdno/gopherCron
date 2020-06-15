package panicgroup

import (
	"fmt"
)

type PanicGroup interface {
	Go(f func())
}

type panicgroup struct {
	err chan error
}

func (p *panicgroup) Go(f func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				p.err <- fmt.Errorf("%+v", r)
			}
		}()
		f()
	}()
}

func NewPanicGroup(errhandle func(err error)) PanicGroup {
	pg := &panicgroup{
		err: make(chan error, 5),
	}

	pg.Go(func() {
		for {
			select {
			case err := <-pg.err:
				errhandle(err)
			}
		}
	})

	return pg
}
