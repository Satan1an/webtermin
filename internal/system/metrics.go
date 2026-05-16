package system

import (
	"context"
	"os"
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
)

type Info struct {
	Hostname        string    `json:"hostname"`
	OS              string    `json:"os"`
	Platform        string    `json:"platform"`
	PlatformVersion string    `json:"platform_version"`
	KernelVersion   string    `json:"kernel_version"`
	Arch            string    `json:"arch"`
	CPUModel        string    `json:"cpu_model"`
	CPUCores        int       `json:"cpu_cores"`
	BootTime        time.Time `json:"boot_time"`
	Uptime          int64     `json:"uptime_sec"`
}

type Metrics struct {
	Time      time.Time     `json:"time"`
	CPU       float64       `json:"cpu_pct"`
	CPUPerCPU []float64     `json:"cpu_pct_per_cpu"`
	Load      [3]float64    `json:"load"`
	Mem       MemMetrics    `json:"mem"`
	Swap      SwapMetrics   `json:"swap"`
	Disks     []DiskMetrics `json:"disks"`
	Network   []NetMetrics  `json:"network"`
}

type MemMetrics struct {
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	Free        uint64  `json:"free"`
	UsedPercent float64 `json:"used_pct"`
}

type SwapMetrics struct {
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	Free        uint64  `json:"free"`
	UsedPercent float64 `json:"used_pct"`
}

type DiskMetrics struct {
	Device      string  `json:"device"`
	Mount       string  `json:"mount"`
	FSType      string  `json:"fstype"`
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	UsedPercent float64 `json:"used_pct"`
}

type NetMetrics struct {
	Interface  string `json:"iface"`
	BytesSent  uint64 `json:"bytes_sent"`
	BytesRecv  uint64 `json:"bytes_recv"`
	PacketsOut uint64 `json:"packets_out"`
	PacketsIn  uint64 `json:"packets_in"`
}

func GetInfo() (*Info, error) {
	h, err := host.Info()
	if err != nil {
		return nil, err
	}
	cores, _ := cpu.Counts(true)
	cpuInfo, _ := cpu.Info()
	model := "unknown"
	if len(cpuInfo) > 0 {
		model = cpuInfo[0].ModelName
	}
	hostname, _ := os.Hostname()
	return &Info{
		Hostname:        hostname,
		OS:              h.OS,
		Platform:        h.Platform,
		PlatformVersion: h.PlatformVersion,
		KernelVersion:   h.KernelVersion,
		Arch:            runtime.GOARCH,
		CPUModel:        model,
		CPUCores:        cores,
		BootTime:        time.Unix(int64(h.BootTime), 0),
		Uptime:          int64(h.Uptime),
	}, nil
}

// GetMetrics returns a snapshot. interval is the CPU sampling window; pass 0 for instant.
func GetMetrics(ctx context.Context, interval time.Duration) (*Metrics, error) {
	m := &Metrics{Time: time.Now()}
	if perCPU, err := cpu.PercentWithContext(ctx, interval, true); err == nil {
		m.CPUPerCPU = perCPU
		var sum float64
		for _, v := range perCPU {
			sum += v
		}
		if len(perCPU) > 0 {
			m.CPU = sum / float64(len(perCPU))
		}
	}
	if l, err := load.Avg(); err == nil {
		m.Load = [3]float64{l.Load1, l.Load5, l.Load15}
	}
	if v, err := mem.VirtualMemory(); err == nil {
		m.Mem = MemMetrics{Total: v.Total, Used: v.Used, Free: v.Free, UsedPercent: v.UsedPercent}
	}
	if v, err := mem.SwapMemory(); err == nil {
		m.Swap = SwapMetrics{Total: v.Total, Used: v.Used, Free: v.Free, UsedPercent: v.UsedPercent}
	}
	if parts, err := disk.Partitions(false); err == nil {
		for _, p := range parts {
			u, err := disk.Usage(p.Mountpoint)
			if err != nil {
				continue
			}
			m.Disks = append(m.Disks, DiskMetrics{
				Device: p.Device, Mount: p.Mountpoint, FSType: p.Fstype,
				Total: u.Total, Used: u.Used, UsedPercent: u.UsedPercent,
			})
		}
	}
	if stats, err := net.IOCounters(true); err == nil {
		for _, s := range stats {
			if s.Name == "lo" {
				continue
			}
			m.Network = append(m.Network, NetMetrics{
				Interface: s.Name,
				BytesSent: s.BytesSent, BytesRecv: s.BytesRecv,
				PacketsOut: s.PacketsSent, PacketsIn: s.PacketsRecv,
			})
		}
	}
	return m, nil
}
