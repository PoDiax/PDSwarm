package api

import (
	"net/http"

	"github.com/podiax/pdswarm/internal/config"
	"github.com/podiax/pdswarm/internal/docker"
)

type SystemInfoDTO struct {
	OS                  string  `json:"os"`
	OperatingSystem     string  `json:"OperatingSystem,omitempty"`
	KernelVersion       string  `json:"kernel_version"`
	Version             string  `json:"version"`
	AppVersion          string  `json:"app_version"`
	PDSwarmVersion      string  `json:"pdswarm_version"`
	Containers          int     `json:"containers"`
	ContainersPascal    int     `json:"Containers,omitempty"`
	ContainersRunning   int     `json:"containers_running"`
	ContainersRunningP  int     `json:"ContainersRunning,omitempty"`
	ContainersPaused    int     `json:"containers_paused"`
	ContainersPausedP   int     `json:"ContainersPaused,omitempty"`
	ContainersStopped   int     `json:"containers_stopped"`
	ContainersStoppedP  int     `json:"ContainersStopped,omitempty"`
	Images              int     `json:"images"`
	ImagesPascal        int     `json:"Images,omitempty"`
	CPUs                int     `json:"cpus"`
	NCPU                int     `json:"NCPU,omitempty"`
	MemoryBytes         int64   `json:"memory_bytes"`
	MemTotal            int64   `json:"MemTotal,omitempty"`
	CPUPercent          float64 `json:"cpu_percent"`
	MemoryUsedBytes     int64   `json:"memory_used_bytes"`
	MemoryPercent       float64 `json:"memory_percent"`
	ServerVersion       string  `json:"server_version"`
	EngineType          string  `json:"engine_type"`
	EngineName          string  `json:"engine_name"`
	EngineVersion       string  `json:"engine_version"`
	// Provide PascalCase alias for frontend SystemInfo interface
	ServerVersionPascal string  `json:"ServerVersion,omitempty"`
	Driver              string  `json:"driver,omitempty"`
	DriverPascal        string  `json:"Driver,omitempty"`
	MemoryLimit         bool    `json:"memory_limit"`
	MemoryLimitPascal   bool    `json:"MemoryLimit"`
	SwapLimit           bool    `json:"swap_limit"`
	SwapLimitPascal     bool    `json:"SwapLimit"`
	DockerRootDir       string  `json:"docker_root_dir,omitempty"`
	DockerRootDirPascal string  `json:"DockerRootDir,omitempty"`
	CgroupVersion       string  `json:"cgroup_version,omitempty"`
	CgroupVersionPascal string  `json:"CgroupVersion,omitempty"`
	CgroupDriver        string  `json:"cgroup_driver,omitempty"`
	CgroupDriverPascal  string  `json:"CgroupDriver,omitempty"`
}

// HandleSystemInfo returns system and Docker daemon information.
func HandleSystemInfo(ds *docker.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ds = dockerFromContext(r.Context(), ds)
		if ds == nil {
			respondJSONError(w, http.StatusServiceUnavailable, "docker service not available")
			return
		}

		info, err := ds.SystemInfo(r.Context())
		if err != nil {
			respondJSONError(w, http.StatusInternalServerError, "failed to get system info: "+err.Error())
			return
		}

		engine := ds.GetEngineInfo(r.Context())
		cpuPercent, memUsed, memPercent := SampleHostTelemetry()

		memTotal := info.MemTotal
		if memTotal <= 0 && globalHostStats.memTotalBytes > 0 {
			memTotal = globalHostStats.memTotalBytes
		}

		ncpu := info.NCPU
		if ncpu <= 0 {
			ncpu = 1
		}

		dto := SystemInfoDTO{
			OS:                  info.OperatingSystem,
			OperatingSystem:     info.OperatingSystem,
			KernelVersion:       info.KernelVersion,
			Version:             config.Version,
			AppVersion:          config.Version,
			PDSwarmVersion:      config.Version,
			Containers:          info.Containers,
			ContainersPascal:    info.Containers,
			ContainersRunning:   info.ContainersRunning,
			ContainersRunningP:  info.ContainersRunning,
			ContainersPaused:    info.ContainersPaused,
			ContainersPausedP:   info.ContainersPaused,
			ContainersStopped:   info.ContainersStopped,
			ContainersStoppedP:  info.ContainersStopped,
			Images:              info.Images,
			ImagesPascal:        info.Images,
			CPUs:                ncpu,
			NCPU:                ncpu,
			MemoryBytes:         memTotal,
			MemTotal:            memTotal,
			CPUPercent:          cpuPercent,
			MemoryUsedBytes:     memUsed,
			MemoryPercent:       memPercent,
			ServerVersion:       engine.ServerVersion,
			EngineType:          engine.Type,
			EngineName:          engine.DisplayName,
			EngineVersion:       engine.Version,
			ServerVersionPascal: engine.ServerVersion,
			Driver:              info.Driver,
			DriverPascal:        info.Driver,
			MemoryLimit:         info.MemoryLimit,
			MemoryLimitPascal:   info.MemoryLimit,
			SwapLimit:           info.SwapLimit,
			SwapLimitPascal:     info.SwapLimit,
			DockerRootDir:       info.DockerRootDir,
			DockerRootDirPascal: info.DockerRootDir,
			CgroupVersion:       info.CgroupVersion,
			CgroupVersionPascal: info.CgroupVersion,
			CgroupDriver:        info.CgroupDriver,
			CgroupDriverPascal:  info.CgroupDriver,
		}

		respondJSON(w, http.StatusOK, dto)
	}
}

// HandleSystemPing returns the health status of the API server and Docker daemon.
func HandleSystemPing(ds *docker.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ds = dockerFromContext(r.Context(), ds)
		if ds != nil {
			if err := ds.Ping(r.Context()); err != nil {
				respondJSONError(w, http.StatusServiceUnavailable, "docker daemon ping failed: "+err.Error())
				return
			}
		}
		respondJSON(w, http.StatusOK, map[string]string{
			"status":  "ok",
			"version": config.Version,
		})
	}
}

// HandleSystemVersion returns the version of the running PDSwarm backend.
func HandleSystemVersion() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		respondJSON(w, http.StatusOK, map[string]string{
			"version": config.Version,
			"name":    "PDSwarm",
		})
	}
}
