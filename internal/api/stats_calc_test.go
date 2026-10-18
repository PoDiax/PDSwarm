package api_test

import (
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/podiax/pdswarm/internal/api"
)

func TestCalculateCPUPercent(t *testing.T) {
	// 50% CPU on a 2 core machine
	prevCPU := uint64(1000)
	currCPU := uint64(1500) // delta = 500
	prevSys := uint64(10000)
	currSys := uint64(11000) // delta = 1000
	onlineCPUs := 2

	// (500 / 1000) * 2 * 100 = 100% total capacity of 1 core or 50% average across 2 cores
	pct := api.CalculateCPUPercent(prevCPU, currCPU, prevSys, currSys, onlineCPUs)
	expected := 100.0
	if pct != expected {
		t.Errorf("expected CPU %f, got %f", expected, pct)
	}
}

func TestCalculateCPUPercent_EdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		prevCPU    uint64
		currCPU    uint64
		prevSys    uint64
		currSys    uint64
		onlineCPUs int
		expected   float64
	}{
		{
			name:       "zero delta cpu",
			prevCPU:    1000,
			currCPU:    1000,
			prevSys:    1000,
			currSys:    2000,
			onlineCPUs: 2,
			expected:   0.0,
		},
		{
			name:       "currCPU less than prevCPU (underflow protection)",
			prevCPU:    2000,
			currCPU:    1000,
			prevSys:    1000,
			currSys:    2000,
			onlineCPUs: 2,
			expected:   0.0,
		},
		{
			name:       "zero delta system",
			prevCPU:    1000,
			currCPU:    2000,
			prevSys:    1000,
			currSys:    1000,
			onlineCPUs: 2,
			expected:   0.0,
		},
		{
			name:       "currSys less than prevSys",
			prevCPU:    1000,
			currCPU:    2000,
			prevSys:    2000,
			currSys:    1000,
			onlineCPUs: 2,
			expected:   0.0,
		},
		{
			name:       "zero online cpus",
			prevCPU:    1000,
			currCPU:    2000,
			prevSys:    1000,
			currSys:    2000,
			onlineCPUs: 0,
			expected:   0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := api.CalculateCPUPercent(tt.prevCPU, tt.currCPU, tt.prevSys, tt.currSys, tt.onlineCPUs)
			if got != tt.expected {
				t.Errorf("expected %f, got %f", tt.expected, got)
			}
		})
	}
}

func TestCalculateStatsSnapshot(t *testing.T) {
	t.Run("nil stats", func(t *testing.T) {
		got := api.CalculateStatsSnapshot(nil)
		if got.CPUPercent != 0 || got.MemoryUsage != 0 {
			t.Errorf("expected empty snapshot for nil, got %+v", got)
		}
	})

	t.Run("complete stats response", func(t *testing.T) {
		stats := &container.StatsResponse{
			Stats: container.Stats{
				CPUStats: container.CPUStats{
					CPUUsage: container.CPUUsage{
						TotalUsage: 2500,
					},
					SystemUsage: 20000,
					OnlineCPUs:  4,
				},
				PreCPUStats: container.CPUStats{
					CPUUsage: container.CPUUsage{
						TotalUsage: 2000, // delta = 500
					},
					SystemUsage: 19000, // delta = 1000
				},
				MemoryStats: container.MemoryStats{
					Usage: 104857600, // 100 MB
					Limit: 209715200, // 200 MB
					Stats: map[string]uint64{
						"inactive_file": 4194304, // 4 MB cache
					},
				},
				BlkioStats: container.BlkioStats{
					IoServiceBytesRecursive: []container.BlkioStatEntry{
						{Op: "Read", Value: 1024},
						{Op: "Write", Value: 2048},
						{Op: "Sync", Value: 512},
					},
				},
				PidsStats: container.PidsStats{
					Current: 14,
				},
			},
			Networks: map[string]container.NetworkStats{
				"eth0": {RxBytes: 5000, TxBytes: 3000},
				"eth1": {RxBytes: 2500, TxBytes: 1500},
			},
		}

		payload := api.CalculateStatsSnapshot(stats)

		// CPU: (500 / 1000) * 4 * 100 = 200.0%
		if payload.CPUPercent != 200.0 {
			t.Errorf("expected CPUPercent 200.0, got %f", payload.CPUPercent)
		}

		// Mem usage: 104857600 - 4194304 = 100663296 (96 MB)
		expectedMemUsage := uint64(104857600 - 4194304)
		if payload.MemoryUsage != expectedMemUsage {
			t.Errorf("expected MemoryUsage %d, got %d", expectedMemUsage, payload.MemoryUsage)
		}
		if payload.MemoryMB != 96.0 {
			t.Errorf("expected MemoryMB 96.0, got %f", payload.MemoryMB)
		}
		if payload.MemoryLimit != 209715200 {
			t.Errorf("expected MemoryLimit 209715200, got %d", payload.MemoryLimit)
		}
		if payload.MemoryLimitMB != 200.0 {
			t.Errorf("expected MemoryLimitMB 200.0, got %f", payload.MemoryLimitMB)
		}
		// MemoryPercent: (96 / 200) * 100 = 48.0%
		if payload.MemoryPercent != 48.0 {
			t.Errorf("expected MemoryPercent 48.0, got %f", payload.MemoryPercent)
		}

		// Networks: Rx: 7500, Tx: 4500
		if payload.NetworkRx != 7500 {
			t.Errorf("expected NetworkRx 7500, got %d", payload.NetworkRx)
		}
		if payload.NetworkTx != 4500 {
			t.Errorf("expected NetworkTx 4500, got %d", payload.NetworkTx)
		}

		// Blkio: Read: 1024, Write: 2048
		if payload.BlockRead != 1024 {
			t.Errorf("expected BlockRead 1024, got %d", payload.BlockRead)
		}
		if payload.BlockWrite != 2048 {
			t.Errorf("expected BlockWrite 2048, got %d", payload.BlockWrite)
		}

		// PIDs: 14
		if payload.PIDs != 14 {
			t.Errorf("expected PIDs 14, got %d", payload.PIDs)
		}
	})

	t.Run("cgroup v1 cache fallback and percpu usage fallback", func(t *testing.T) {
		stats := &container.StatsResponse{
			Stats: container.Stats{
				CPUStats: container.CPUStats{
					CPUUsage: container.CPUUsage{
						TotalUsage:  2000,
						PercpuUsage: []uint64{1000, 1000}, // 2 cpus
					},
					SystemUsage: 20000,
					OnlineCPUs:  0, // not set
				},
				PreCPUStats: container.CPUStats{
					CPUUsage: container.CPUUsage{
						TotalUsage: 1000,
					},
					SystemUsage: 10000,
				},
				MemoryStats: container.MemoryStats{
					Usage: 52428800, // 50 MB
					Limit: 104857600,
					Stats: map[string]uint64{
						"total_inactive_file": 2097152, // 2 MB
					},
				},
			},
		}

		payload := api.CalculateStatsSnapshot(stats)
		// (1000 / 10000) * 2 * 100 = 20%
		if payload.CPUPercent != 20.0 {
			t.Errorf("expected CPUPercent 20.0, got %f", payload.CPUPercent)
		}
		expectedMem := uint64(52428800 - 2097152)
		if payload.MemoryUsage != expectedMem {
			t.Errorf("expected MemoryUsage %d, got %d", expectedMem, payload.MemoryUsage)
		}
	})

	t.Run("missing system usage uses time delta fallback", func(t *testing.T) {
		now := time.Now()
		stats := &container.StatsResponse{
			Stats: container.Stats{
				Read:    now,
				PreRead: now.Add(-1 * time.Second), // 1 second = 1e9 ns
				CPUStats: container.CPUStats{
					CPUUsage: container.CPUUsage{
						TotalUsage: 500000000, // 0.5s = 500ms
					},
					SystemUsage: 0, // Not populated (Windows / cgroup v2)
					OnlineCPUs:  2,
				},
				PreCPUStats: container.CPUStats{
					CPUUsage: container.CPUUsage{
						TotalUsage: 0,
					},
					SystemUsage: 0,
				},
			},
		}

		payload := api.CalculateStatsSnapshot(stats)
		// (0.5s / 1.0s) * 2 cores * 100 = 100.0%
		if payload.CPUPercent != 100.0 {
			t.Errorf("expected CPUPercent 100.0, got %f", payload.CPUPercent)
		}
	})
}

