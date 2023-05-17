package utils

import (
	"errors"
	"testing"
	"time"
)

func TestGetDateFromNow(t *testing.T) {
	t.Log(GetDateFromNow(-0).Format("2006-01-02"))
}

func TestCloseChan(t *testing.T) {
	c := make(chan error, 10)

	c <- errors.New("1")
	c <- errors.New("2")
	c <- errors.New("3")
	c <- errors.New("4")

	close(c)

	timer := time.NewTimer(time.Second * 2)
	for {
		select {
		case <-timer.C:
			t.Log("finished")
			return
		case err := <-c:
			t.Log("got error", err)
		}
	}
}

func TestSignalChannel(t *testing.T) {
	sc := NewSignalChannel[error]()

	sc.Send(errors.New("1"))
	sc.Send(errors.New("2"))
	sc.Send(errors.New("3"))
	sc.Send(errors.New("4"))

	if err := sc.WaitOne(); err != nil {
		if err.Error() != "1" {
			t.Fatal("got unexcept error", err)
		}
	}

	scTwo := NewSignalChannel[error]()
	scTwo.Close()

	if err := sc.WaitOne(); err != nil {
		t.Fatal("got unexcept error", err)
	}
}

func TestNewReasonWriter(t *testing.T) {
	r := NewReasonWriter()
	r.WriteString("test1")
	r.WriteStringPrefix("test2")

	if r.String() != "test2,test1" {
		t.Fatal("got unexcept string", r.String())
	}
}
