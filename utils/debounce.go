package utils

import "time"

func NewDebounce(t time.Duration, f func()) func() {
	timer := time.AfterFunc(t, f)
	return func() {
		timer.Reset(t)
	}
}
