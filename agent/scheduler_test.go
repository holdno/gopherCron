package agent

import (
	"testing"
	"time"
)

func TestSchedulerLatency(t *testing.T) {
	for i := 0; i < 10; i++ {
		t.Log("weight 100", getSchedulerLatency(100, int32(i)))
		t.Log("weight 50", getSchedulerLatency(50, int32(i)))
	}
	for i := 0; i < 10; i++ {
		t.Log(getSchedulerLatency(100, int32(i)) < getSchedulerLatency(50, 1))
	}
}

func TestSchedulerLatencyWithRunningCounter(t *testing.T) {
	l50 := func() time.Duration {
		return getSchedulerLatency(50, 0)
	}

	l100 := func() time.Duration {
		return getSchedulerLatency(100, 0)
	}

	var (
		sCounter = 0
		bCounter = 0
	)

	for i := 0; i < 100; i++ {
		s := l50()
		b := l100()

		if s < b {
			sCounter++
		} else {
			bCounter++
		}
	}

	t.Log("l50:", sCounter)
	t.Log("l100:", bCounter)
}
