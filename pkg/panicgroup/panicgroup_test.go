package panicgroup

import (
	"fmt"
	"sync"
	"testing"
)

func TestPanicgroup_Go(t *testing.T) {
	pg := NewPanicGroup(func(err error) {
		fmt.Println(err)
	})

	w := sync.WaitGroup{}
	for i := 0; i < 10; i++ {
		w.Add(1)
		pg.Go(func(a ...interface{}) {
			fmt.Println(a[0])
			w.Done()
		})(i)
	}

	w.Wait()
}
