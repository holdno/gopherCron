package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/holdno/gopherCron/common"
)

func TestTaskUnmarshal(t *testing.T) {
	raw := `{"task_id": "12313123"}`

	var data common.TaskWithOperator

	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		t.Fatal(err)
	}

	t.Log(data.TaskInfo)
}

func fatal(format string, args ...interface{}) {
	fmt.Println(fmt.Sprintf(format, args...))
	os.Exit(1)
}

func TestApp_ExecuteTask(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	// var output bytes.Buffer
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", "sleep 8 && echo hello world")
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		fatal("no pipe: %v", err)
	}
	start := time.Now()

	if err = cmd.Start(); err != nil {
		fatal("start failed: %v", err)
	}
	var stdout strings.Builder
	go func() {
		buf := bufio.NewReader(stdoutPipe)
		for {
			line, err := buf.ReadString('\n')
			if err != nil {
				return
			}
			if len(line) > 0 {
				stdout.WriteString(line)
				stdout.WriteString("\n")
			}
		}
	}()

	err = cmd.Wait()
	d := time.Since(start)

	if err != nil {
		exiterr := err.(*exec.ExitError)
		status := exiterr.Sys().(syscall.WaitStatus)
		if status.ExitStatus() != 0 {
			t.Logf("wrong exit status: %v, %v", status.ExitStatus(), cmd.ProcessState.ExitCode())
		}
	}

	if d.Seconds() >= 3 {
		t.Logf("Cancelation took too long: %v", d)
	}

	time.Sleep(time.Second * 10)
	fmt.Println("Success!", stdout.String())
}
