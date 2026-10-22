package syswin

import (
	"runtime"
)

type DiskSpace struct {
	Drive          string  `json:"drive"`
	FreeBytes      uint64  `json:"free_bytes"`
	TotalBytes     uint64  `json:"total_bytes"`
	UsedBytes      uint64  `json:"used_bytes"`
	AvailableBytes uint64  `json:"available_bytes"`
	UsedPercent    float64 `json:"used_percent"`
}

type MemStats struct {
	AllocMB      float64 `json:"alloc_mb"`
	TotalAllocMB float64 `json:"total_alloc_mb"`
	SysMB        float64 `json:"sys_mb"`
	NumGC        uint32  `json:"num_gc"`
	Goroutines   int     `json:"goroutines"`
}

func GetMemStats() MemStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return MemStats{
		AllocMB:      float64(m.Alloc) / (1024 * 1024),
		TotalAllocMB: float64(m.TotalAlloc) / (1024 * 1024),
		SysMB:        float64(m.Sys) / (1024 * 1024),
		NumGC:        m.NumGC,
		Goroutines:   runtime.NumGoroutine(),
	}
}
