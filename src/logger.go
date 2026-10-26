package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	logMu    sync.Mutex
	logFiles []*os.File
	LogPath  string
)

func InitLogger() {
	// 1. Try exe directory
	exePath, err := os.Executable()
	var primaryPath string
	if err == nil {
		exeDir := filepath.Dir(exePath)
		primaryPath = filepath.Join(exeDir, "save_migrator.log")
	} else {
		primaryPath = "save_migrator.log"
	}

	logMu.Lock()
	f1, err1 := os.OpenFile(primaryPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err1 == nil {
		logFiles = append(logFiles, f1)
		LogPath = primaryPath
	}

	// 2. Also write to Temp dir as fallback
	tempLog := filepath.Join(os.TempDir(), "save_migrator.log")
	if tempLog != primaryPath {
		f2, err2 := os.OpenFile(tempLog, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err2 == nil {
			logFiles = append(logFiles, f2)
			if LogPath == "" {
				LogPath = tempLog
			}
		}
	}
	logMu.Unlock()

	Log("=======================================================")
	Log("Checkpoint started at %s", time.Now().Format(time.RFC3339))
	Log("Log file active at: %s", LogPath)
	Log("=======================================================")
}

func Log(format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...)
	ts := time.Now().Format("15:04:05")
	line := fmt.Sprintf("[%s] %s\n", ts, msg)

	logMu.Lock()
	defer logMu.Unlock()

	// Print to stdout
	fmt.Print(line)

	// Write to all active log files
	for _, f := range logFiles {
		io.WriteString(f, line)
		f.Sync()
	}
}

func GetLogFilePath() string {
	logMu.Lock()
	defer logMu.Unlock()
	if LogPath != "" {
		return LogPath
	}
	return filepath.Join(os.TempDir(), "save_migrator.log")
}
