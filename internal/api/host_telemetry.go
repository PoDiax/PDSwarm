package api

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type hostStatsSampler struct {
	mu             sync.Mutex
	lastTotal      uint64
	lastIdle       uint64
	lastSampleTime time.Time
	cpuPercent     float64
	memUsedBytes   int64
	memTotalBytes  int64
	memPercent     float64
}

var globalHostStats = &hostStatsSampler{}

func init() {
	// Prime the CPU sampler so initial calls don't return 0
	go func() {
		globalHostStats.sample()
		time.Sleep(100 * time.Millisecond)
		globalHostStats.sample()
	}()
}

func (h *hostStatsSampler) sample() (float64, int64, float64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now()
	// Cache for 400ms to prevent excessive I/O while still delivering live responsive telemetry
	if !h.lastSampleTime.IsZero() && now.Sub(h.lastSampleTime) < 400*time.Millisecond && h.lastTotal > 0 {
		return h.cpuPercent, h.memUsedBytes, h.memPercent
	}

	// 1. Read /proc/stat on Linux systems
	if statFile, err := os.Open("/proc/stat"); err == nil {
		scanner := bufio.NewScanner(statFile)
		if scanner.Scan() {
			fields := strings.Fields(scanner.Text())
			if len(fields) >= 5 && fields[0] == "cpu" {
				var total uint64
				var idle uint64
				for i := 1; i < len(fields); i++ {
					val, _ := strconv.ParseUint(fields[i], 10, 64)
					total += val
					if i == 4 || i == 5 { // idle + iowait
						idle += val
					}
				}

				if h.lastTotal > 0 && total > h.lastTotal {
					totalDelta := float64(total - h.lastTotal)
					idleDelta := float64(idle - h.lastIdle)
					if totalDelta > 0 {
						pct := (1.0 - (idleDelta / totalDelta)) * 100.0
						if pct < 0 {
							pct = 0
						} else if pct > 100 {
							pct = 100
						}
						h.cpuPercent = pct
					}
				}
				h.lastTotal = total
				h.lastIdle = idle
				h.lastSampleTime = now
			}
		}
		_ = statFile.Close()
	}

	// 2. Read /proc/meminfo on Linux systems
	if memFile, err := os.Open("/proc/meminfo"); err == nil {
		scanner := bufio.NewScanner(memFile)
		var memTotal, memAvailable int64
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "MemTotal:") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					kb, _ := strconv.ParseInt(parts[1], 10, 64)
					memTotal = kb * 1024
				}
			} else if strings.HasPrefix(line, "MemAvailable:") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					kb, _ := strconv.ParseInt(parts[1], 10, 64)
					memAvailable = kb * 1024
				}
			}
			if memTotal > 0 && memAvailable > 0 {
				break
			}
		}
		_ = memFile.Close()

		if err := scanner.Err(); err != nil {
			return h.cpuPercent, h.memUsedBytes, h.memPercent
		}

		if memTotal > 0 {
			h.memTotalBytes = memTotal
			used := memTotal - memAvailable
			if used < 0 {
				used = 0
			}
			h.memUsedBytes = used
			h.memPercent = (float64(used) / float64(memTotal)) * 100.0
		}
	}

	return h.cpuPercent, h.memUsedBytes, h.memPercent
}

// SampleHostTelemetry returns current host CPU percentage, memory used bytes, and memory percentage.
func SampleHostTelemetry() (float64, int64, float64) {
	return globalHostStats.sample()
}
