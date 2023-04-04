package utils

import (
	"testing"
	"time"
)

func TestNewDebounce(t *testing.T) {
	now := time.Now()
	de := NewDebounce(time.Second, func() {
		t.Log("after", time.Since(now))
	})

	for i := 0; i < 10; i++ {
		de()
		time.Sleep(500 * time.Millisecond)
	}

	time.Sleep(time.Second)
	de()

	time.Sleep(time.Second * 2)
}
