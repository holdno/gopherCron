package agent

import "testing"

func TestSchedulerLatency(t *testing.T) {
	for i := 0; i < 10; i++ {
		t.Log("weight 100", getSchedulerLatency(100, int32(i)))
		t.Log("weight 50", getSchedulerLatency(50, int32(i)))
	}
	for i := 0; i < 10; i++ {
		t.Log(getSchedulerLatency(100, int32(i)) < getSchedulerLatency(50, 1))
	}
}

func TestChanSend(t *testing.T) {
	c1 := make(chan struct{})
	c1 <- struct{}{}
	t.Log("success")
}
