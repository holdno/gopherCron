package daemon

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/holdno/gopherCron/config"
)

func TestProjectDaemon_WaitSignal(t *testing.T) {
	projects := []config.ProjectAuth{{
		ProjectID: 1,
	},
		{
			ProjectID: 2,
		}, {
			ProjectID: 3,
		}, {
			ProjectID: 4,
		}, {
			ProjectID: 5,
		}}

	pd := NewProjectDaemon(nil, nil)
	addProjects, _ := pd.DiffAndAddProjects(projects)
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
	pd := NewProjectDaemon(projects, nil)
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
	pd.DiffAndAddProjects(nil)
	w.Wait()
}
