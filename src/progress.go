package main

import (
	"sync"
)

type ProgressInfo struct {
	Active    bool    `json:"active"`
	Operation string  `json:"operation"` // "scan", "backup", "verify", "restore", "idle"
	Percent   float64 `json:"percent"`   // 0.0 to 100.0
	Status    string  `json:"status"`    // Current human-readable phase
	Current   int     `json:"current"`   // Current step or file
	Total     int     `json:"total"`     // Total steps or files
	Detail    string  `json:"detail"`    // Extra contextual detail (file path, throughput, etc.)
}

var (
	progressMu   sync.RWMutex
	currProgress ProgressInfo
)

func SetProgress(op string, percent float64, status string, current, total int, detail string) {
	progressMu.Lock()
	defer progressMu.Unlock()
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	currProgress = ProgressInfo{
		Active:    true,
		Operation: op,
		Percent:   percent,
		Status:    status,
		Current:   current,
		Total:     total,
		Detail:    detail,
	}
}

func GetProgress() ProgressInfo {
	progressMu.RLock()
	defer progressMu.RUnlock()
	return currProgress
}

func ClearProgress(status string) {
	progressMu.Lock()
	defer progressMu.Unlock()
	currProgress = ProgressInfo{
		Active:    false,
		Operation: "idle",
		Percent:   100,
		Status:    status,
		Current:   0,
		Total:     0,
		Detail:    "",
	}
}
