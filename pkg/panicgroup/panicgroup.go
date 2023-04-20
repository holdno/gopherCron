package panicgroup

import (
	"fmt"
	"sync"
)

type PanicGroup interface {
	Go(f func())
	Close()
}

type panicgroup struct {
	err   chan error
	close bool
}

func (p *panicgroup) Close() {
	p.close = true
}

func (p *panicgroup) Go(f func()) {

	go func() {
		for {
			wait := &sync.WaitGroup{}
			func() {
				wait.Add(1)
				defer func() {
					if r := recover(); r != nil {
						p.err <- fmt.Errorf("%+v", r)
					}
					wait.Done()
				}()
				f()
				wait.Wait()
			}()
			if p.close {
				return
			}
		}
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
