package daemon

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestProjectDaemon_WaitSignal(t *testing.T) {
	projects := []int64{1, 2, 3, 4, 5}
	pd := NewProjectDaemon(nil)
	addProjects, _ := pd.DiffProjects(projects)
	for _, v := range addProjects {
		pd.addProject(v)
	}
	for {
		select {
		case <-pd.WaitRemoveSignal(6):
			t.Log("success")
		default:
			return
		}

	}
	t.Log("success")
}

func TestNewProjectDaemon(t *testing.T) {
	projects := []int64{1, 2, 3, 4, 5}
	pd := NewProjectDaemon(projects)
	ctx, _ := context.WithTimeout(context.Background(), time.Second*5)
	w := sync.WaitGroup{}
	for _, v := range projects {
		w.Add(1)
		go func(v int64) {
			for {
				select {
				case <-ctx.Done():
					fmt.Printf("timeout %d\n", v)
					return
				case <-pd.WaitRemoveSignal(v):
					fmt.Printf("%d is down\n", v)
					w.Done()
					return
				}
			}
		}(v)
	}

	time.Sleep(time.Second)
	pd.DiffProjects(nil)
	w.Wait()
}
