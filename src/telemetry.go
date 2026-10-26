package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"regexp"
	"runtime"
	"strings"
	"time"
)

type TelemetryReport struct {
	Timestamp   string              `json:"timestamp"`
	OS          string              `json:"os"`
	TotalTitles int                 `json:"total_titles"`
	Games       []SanitizedGameItem `json:"games"`
	OnlineGames []string            `json:"online_games,omitempty"`
}

type SanitizedGameItem struct {
	Name           string `json:"name"`
	AppID          string `json:"appid,omitempty"`
	StorageType      string `json:"storage_type"`
	SanitizedPath  string `json:"sanitized_path"`
	TargetRelative string `json:"target_relative"`
	FileCount      int    `json:"file_count"`
	SizeBytes      int64  `json:"size_bytes"`
	ProtonTier     string `json:"proton_tier,omitempty"`
}

var userProfileRegex = regexp.MustCompile(`(?i)[a-zA-Z]:\\Users\\[^\\]+`)
var homeUserRegex = regexp.MustCompile(`^/home/[^/]+`)

// Strip usernames and machine paths before sending
func SanitizePath(p string) string {
	if p == "" {
		return ""
	}
	s := p
	s = userProfileRegex.ReplaceAllString(s, "%USERPROFILE%")
	s = homeUserRegex.ReplaceAllString(s, "/home/<user>")

	s = strings.ReplaceAll(s, "%USERPROFILE%\\AppData\\LocalLow", "%USERPROFILE%\\AppData\\LocalLow")
	s = strings.ReplaceAll(s, "%USERPROFILE%\\AppData\\Local", "%LOCALAPPDATA%")
	s = strings.ReplaceAll(s, "%USERPROFILE%\\AppData\\Roaming", "%APPDATA%")
	s = strings.ReplaceAll(s, "%USERPROFILE%\\Documents", "%DOCUMENTS%")
	s = strings.ReplaceAll(s, "%USERPROFILE%\\OneDrive\\Documents", "%DOCUMENTS%")
	s = strings.ReplaceAll(s, "%USERPROFILE%\\Saved Games", "%SAVEDGAMES%")

	return s
}

func BuildSanitizedTelemetryReport(games []DetectedGame, onlineGames []OnlineGame) TelemetryReport {
	sanitizedGames := make([]SanitizedGameItem, 0, len(games))
	for _, g := range games {
		sanitizedGames = append(sanitizedGames, SanitizedGameItem{
			Name:           g.Name,
			AppID:          g.AppID,
			StorageType:      g.StorageType,
			SanitizedPath:  SanitizePath(g.SourcePath),
			TargetRelative: g.TargetRelativePath,
			FileCount:      g.FileCount,
			SizeBytes:      g.SizeBytes,
			ProtonTier:     g.ProtonTier,
		})
	}

	onlineNames := make([]string, 0, len(onlineGames))
	for _, og := range onlineGames {
		onlineNames = append(onlineNames, og.Name)
	}

	return TelemetryReport{
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		OS:          runtime.GOOS,
		TotalTitles: len(games),
		Games:       sanitizedGames,
		OnlineGames: onlineNames,
	}
}

func DispatchTelemetryReport(games []DetectedGame, onlineGames []OnlineGame) {
	cfg := LoadConfig()
	if !cfg.TelemetryEnabled {
		return
	}

	go func() {
		report := BuildSanitizedTelemetryReport(games, onlineGames)
		data, err := json.Marshal(report)
		if err != nil {
			return
		}

		client := &http.Client{Timeout: 4 * time.Second}
		req, err := http.NewRequest("POST", "https://reports.cryptori.al/api/report", bytes.NewBuffer(data))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "Checkpoint-Telemetry/0.9")

		resp, err := client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
		}
	}()
}
