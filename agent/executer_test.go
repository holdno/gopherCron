package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/holdno/gopherCron/common"
	"github.com/spacegrower/watermelon/infra/wlog"
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
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()

	go func() {
		time.Sleep(time.Second * 3)
		cancel()
		fmt.Println("canceled")
	}()

	std, err := execute(ctx, "/bin/sh", "echo hello world", wlog.With())
	if err != nil {
		t.Fatal(err)
	}

	if std != nil {
		fmt.Println("got stdout/error", std.String())
	}
}

func TestOffCounter(t *testing.T) {
	offCounter := &atomic.Bool{}

	if !offCounter.CompareAndSwap(false, true) {
		t.Fatal("")
	}

	if offCounter.CompareAndSwap(false, true) {
		t.Fatal("")
	}
}

func TestResultSub(t *testing.T) {
	keyword := "啊"
	var output strings.Builder
	for i := 0; i < 5000; i++ {
		output.WriteString(keyword)
	}

	runeStr := []rune(strings.TrimSuffix(output.String(), "\n"))
	if len(runeStr) > 5000 {
		runeStr = runeStr[:5000]
	}

	result := string(runeStr)

	fmt.Println(len(keyword), len(result), len(runeStr))
}
