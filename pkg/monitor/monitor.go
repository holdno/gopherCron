package monitor

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/pkg/etcd"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
)

func Monitor() {

	var (
		cpuPercent []float64
		memStat    *mem.VirtualMemoryStat
		info       []byte
		err        error
	)
	for {
		if cpuPercent, err = cpu.Percent(0, false); err != nil {
			goto RETRY
		}

		if memStat, err = mem.VirtualMemory(); err != nil {
			goto RETRY
		}

		if info, err = json.Marshal(&common.MonitorInfo{
			IP:            common.LocalIP,
			CpuPercent:    fmt.Sprintf("%.2f", cpuPercent[0]),
			MemoryPercent: fmt.Sprintf("%.2f", memStat.UsedPercent),
			MemoryTotal:   memStat.Total / 1024,
			MemoryFree:    memStat.Free / 1024,
		}); err != nil {
			goto RETRY
		}

		err = etcd.Manager.SaveMonitor(common.LocalIP, info)

	RETRY:
		if err != nil {
			err = nil
		}
		time.Sleep(time.Duration(common.MonitorFrequency) * time.Second)
	}

}
