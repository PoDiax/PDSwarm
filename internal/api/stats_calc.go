package api

import (
	"runtime"
	"strings"

	"github.com/docker/docker/api/types/container"
)

// CalculateCPUPercent computes the CPU usage percentage across the given online CPUs.
func CalculateCPUPercent(prevCPU, currCPU, prevSys, currSys uint64, onlineCPUs int) float64 {
	if currCPU <= prevCPU || onlineCPUs <= 0 {
		return 0.0
	}
	cpuDelta := float64(currCPU - prevCPU)
	if currSys > prevSys {
		sysDelta := float64(currSys - prevSys)
		if sysDelta > 0.0 && cpuDelta > 0.0 {
			return (cpuDelta / sysDelta) * float64(onlineCPUs) * 100.0
		}
	}
	return 0.0
}

// ContainerStatsPayload represents structured real-time container metrics sent to WebSockets.
type ContainerStatsPayload struct {
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryUsage   uint64  `json:"memory_usage"`
	MemoryLimit   uint64  `json:"memory_limit"`
	MemoryMB      float64 `json:"memory_mb"`
	MemoryLimitMB float64 `json:"memory_limit_mb"`
	MemoryPercent float64 `json:"memory_percent"`
	NetworkRx     uint64  `json:"network_rx"`
	NetworkTx     uint64  `json:"network_tx"`
	BlockRead     uint64  `json:"block_read"`
	BlockWrite    uint64  `json:"block_write"`
	PIDs          uint64  `json:"pids"`
}

// CalculateStatsSnapshot converts a Docker StatsResponse into a ContainerStatsPayload.
func CalculateStatsSnapshot(stats *container.StatsResponse) ContainerStatsPayload {
	if stats == nil {
		return ContainerStatsPayload{}
	}

	// 1. CPU
	onlineCPUs := int(stats.CPUStats.OnlineCPUs)
	if onlineCPUs == 0 {
		onlineCPUs = len(stats.CPUStats.CPUUsage.PercpuUsage)
	}
	if onlineCPUs == 0 {
		onlineCPUs = runtime.NumCPU()
	}
	if onlineCPUs <= 0 {
		onlineCPUs = 1
	}

	var cpuPercent float64
	currCPU := stats.CPUStats.CPUUsage.TotalUsage
	prevCPU := stats.PreCPUStats.CPUUsage.TotalUsage
	currSys := stats.CPUStats.SystemUsage
	prevSys := stats.PreCPUStats.SystemUsage

	if currCPU > prevCPU {
		cpuDelta := float64(currCPU - prevCPU)
		if currSys > prevSys {
			sysDelta := float64(currSys - prevSys)
			if sysDelta > 0 {
				cpuPercent = (cpuDelta / sysDelta) * float64(onlineCPUs) * 100.0
			}
		} else if !stats.Read.IsZero() && !stats.PreRead.IsZero() {
			// Fallback for Windows, cgroups v2, or environments where SystemUsage is not populated
			timeDelta := float64(stats.Read.Sub(stats.PreRead).Nanoseconds())
			if timeDelta > 0 {
				if stats.NumProcs > 0 {
					// Windows container: TotalUsage is 100ns units
					cpuPercent = ((cpuDelta * 100.0) / timeDelta) * 100.0
				} else {
					cpuPercent = (cpuDelta / timeDelta) * float64(onlineCPUs) * 100.0
				}
			}
		}
	}

	// 2. Memory
	memUsage := stats.MemoryStats.Usage
	if stats.MemoryStats.Stats != nil {
		if inactiveFile, ok := stats.MemoryStats.Stats["inactive_file"]; ok && inactiveFile < memUsage {
			memUsage -= inactiveFile
		} else if totalInactiveFile, ok := stats.MemoryStats.Stats["total_inactive_file"]; ok && totalInactiveFile < memUsage {
			memUsage -= totalInactiveFile
		}
	}

	memLimit := stats.MemoryStats.Limit
	var memPercent float64
	if memLimit > 0 {
		memPercent = (float64(memUsage) / float64(memLimit)) * 100.0
	}

	memMB := float64(memUsage) / (1024.0 * 1024.0)
	memLimitMB := float64(memLimit) / (1024.0 * 1024.0)

	// 3. Network I/O
	var netRx, netTx uint64
	for _, n := range stats.Networks {
		netRx += n.RxBytes
		netTx += n.TxBytes
	}

	// 4. Block I/O
	var blkRead, blkWrite uint64
	for _, entry := range stats.BlkioStats.IoServiceBytesRecursive {
		op := strings.ToLower(entry.Op)
		if strings.HasPrefix(op, "r") {
			blkRead += entry.Value
		} else if strings.HasPrefix(op, "w") {
			blkWrite += entry.Value
		}
	}

	// 5. PIDs
	pids := stats.PidsStats.Current

	return ContainerStatsPayload{
		CPUPercent:    cpuPercent,
		MemoryUsage:   memUsage,
		MemoryLimit:   memLimit,
		MemoryMB:      memMB,
		MemoryLimitMB: memLimitMB,
		MemoryPercent: memPercent,
		NetworkRx:     netRx,
		NetworkTx:     netTx,
		BlockRead:     blkRead,
		BlockWrite:    blkWrite,
		PIDs:          pids,
	}
}
