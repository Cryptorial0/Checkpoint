package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

type SaveType string

const (
	SaveTypePrefix  SaveType = "prefix"
	SaveTypeInTree  SaveType = "in_tree"
	SaveTypeProfile SaveType = "profile"
)

type DetectedGame struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	AppID              string   `json:"appid"`
	SaveType           SaveType `json:"save_type"`
	StorageType          string   `json:"storage_type"`
	SourcePath         string   `json:"source_path"`
	TargetRelativePath string   `json:"target_relative_path"`
	FileCount          int      `json:"file_count"`
	SizeBytes          int64    `json:"size_bytes"`
	IconURL            string   `json:"icon_url,omitempty"`
	ProtonTier         string   `json:"proton_tier,omitempty"`
	Notes              string   `json:"notes,omitempty"`
	IsSteamCloud       bool     `json:"is_steam_cloud,omitempty"`
	IsUserSettings     bool     `json:"is_user_settings,omitempty"`
}

type OnlineGame struct {
	Name          string   `json:"name"`
	Status        string   `json:"status"` // Supported, Running, Broken, Denied
	AntiCheats    []string `json:"anticheats"`
	InstallSource string   `json:"install_source"`
}

var (
	appNameCacheMu sync.RWMutex
	appNameCache   = make(map[string]string)
	cacheLoaded    = false

	protonTierCacheMu sync.RWMutex
	protonTierCache   = make(map[string]string)
)

// Known fallback AppIDs
var knownAppIDs = map[string]string{
	"1627720": "Lies of P",
	"1091500": "Cyberpunk 2077",
	"1245620": "Elden Ring",
	"1174180": "Red Dead Redemption 2",
	"1551360": "Forza Horizon 5",
	"1817070": "Marvel's Spider-Man Remastered",
	"2050650": "Resident Evil 4",
	"883710":  "Resident Evil 2",
	"1196590": "Resident Evil Village",
	"1364780": "Street Fighter 6",
	"1938090": "Call of Duty",
	"990080":  "Hogwarts Legacy",
	"2358720": "Black Myth: Wukong",
	"1774580": "Star Wars Jedi: Survivor",
	"1086940": "Baldur's Gate 3",
	"1644960": "NBA 2K24",
	"1888160": "Armored Core VI",
	"1888930": "The Last of Us™ Part I",
	"548430":  "Deep Rock Galactic",
	"1145360": "Hades",
	"1145350": "Hades II",
	"632360":  "Risk of Rain 2",
	"252490":  "Rust",
	"374320":  "DARK SOULS III",
	"814380":  "Sekiro: Shadows Die Twice",
}

func initCache() {
	if cacheLoaded {
		return
	}
	cacheLoaded = true
	for k, v := range knownAppIDs {
		appNameCache[k] = v
	}

	cachePath := getCacheFilePath()
	if data, err := os.ReadFile(cachePath); err == nil {
		var diskCache map[string]string
		if json.Unmarshal(data, &diskCache) == nil {
			for k, v := range diskCache {
				appNameCache[k] = v
			}
		}
	}
}

func saveCache() {
	appNameCacheMu.RLock()
	defer appNameCacheMu.RUnlock()
	cachePath := getCacheFilePath()
	data, err := json.MarshalIndent(appNameCache, "", "  ")
	if err == nil {
		_ = os.WriteFile(cachePath, data, 0644)
	}
}

func getCacheFilePath() string {
	if runtime.GOOS == "windows" {
		temp := os.Getenv("TEMP")
		if temp == "" {
			temp = `C:\Temp`
		}
		return filepath.Join(temp, "save_migrator_steam_cache.json")
	}
	return filepath.Join(os.TempDir(), "save_migrator_steam_cache.json")
}

type steamStoreResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Name         string   `json:"name"`
		HeaderImage  string   `json:"header_image"`
		CapsuleImage string   `json:"capsule_image"`
		Developers   []string `json:"developers"`
		Publishers   []string `json:"publishers"`
	} `json:"data"`
}

var (
	appIconCache         = make(map[string]string)
	appIconCacheMu       sync.RWMutex
	appDevelopersCache   = make(map[string][]string)
	appDevelopersCacheMu sync.RWMutex
)

// Dynamic online Steam metadata fetcher (fetches exact name, CDN hashed header image, and studio creators)
func fetchSteamMetadata(appid string) (string, string) {
	if appid == "" {
		return "", ""
	}

	appNameCacheMu.RLock()
	cachedName, nameFound := appNameCache[appid]
	appNameCacheMu.RUnlock()

	appIconCacheMu.RLock()
	cachedIcon, iconFound := appIconCache[appid]
	appIconCacheMu.RUnlock()

	if nameFound && iconFound && cachedName != "" && cachedIcon != "" {
		return cachedName, cachedIcon
	}

	url := fmt.Sprintf("https://store.steampowered.com/api/appdetails?appids=%s&filters=basic,developers,publishers", appid)
	client := http.Client{Timeout: 3000 * time.Millisecond}
	resp, err := client.Get(url)
	if err != nil {
		return cachedName, cachedIcon
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return cachedName, cachedIcon
	}

	var root map[string]steamStoreResponse
	if err := json.NewDecoder(resp.Body).Decode(&root); err != nil {
		return cachedName, cachedIcon
	}

	name := cachedName
	iconURL := cachedIcon

	if appData, ok := root[appid]; ok && appData.Success {
		if appData.Data.Name != "" {
			name = strings.TrimSpace(appData.Data.Name)
			appNameCacheMu.Lock()
			appNameCache[appid] = name
			appNameCacheMu.Unlock()
		}
		if appData.Data.HeaderImage != "" {
			iconURL = appData.Data.HeaderImage
		} else if appData.Data.CapsuleImage != "" {
			iconURL = appData.Data.CapsuleImage
		}
		if iconURL != "" {
			appIconCacheMu.Lock()
			appIconCache[appid] = iconURL
			appIconCacheMu.Unlock()
		}

		var devs []string
		devs = append(devs, appData.Data.Developers...)
		devs = append(devs, appData.Data.Publishers...)
		if len(devs) > 0 {
			appDevelopersCacheMu.Lock()
			appDevelopersCache[appid] = devs
			appDevelopersCacheMu.Unlock()
		}
	}

	return name, iconURL
}

func fetchSteamNameOnline(appid string) string {
	name, _ := fetchSteamMetadata(appid)
	return name
}

func fetchProtonDBTier(appid string) string {
	if appid == "" {
		return ""
	}
	protonTierCacheMu.RLock()
	if tier, found := protonTierCache[appid]; found {
		protonTierCacheMu.RUnlock()
		return tier
	}
	protonTierCacheMu.RUnlock()

	url := fmt.Sprintf("https://www.protondb.com/api/v1/reports/summaries/%s.json", appid)
	client := http.Client{Timeout: 2000 * time.Millisecond}
	resp, err := client.Get(url)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	var data struct {
		Tier string `json:"tier"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err == nil && data.Tier != "" {
		protonTierCacheMu.Lock()
		protonTierCache[appid] = strings.ToLower(data.Tier)
		protonTierCacheMu.Unlock()
		return strings.ToLower(data.Tier)
	}
	return ""
}

func getSteamIconURL(appid string) string {
	if appid == "" {
		return ""
	}
	appIconCacheMu.RLock()
	icon, found := appIconCache[appid]
	appIconCacheMu.RUnlock()
	if found && icon != "" {
		return icon
	}

	_, icon = fetchSteamMetadata(appid)
	if icon != "" {
		return icon
	}

	return fmt.Sprintf("https://shared.cloudflare.steamstatic.com/store_item_assets/steam/apps/%s/capsule_sm_120.jpg", appid)
}

func isDLCOrPack(title string) bool {
	low := strings.ToLower(title)
	packKeywords := []string{" pack", " dlc", " soundtrack", " expansion", " season pass", " bundle", " artbook", " add-on", " bonus"}
	for _, kw := range packKeywords {
		if strings.Contains(low, kw) {
			return true
		}
	}
	return false
}

func normalizeTitleKey(s string) string {
	s = strings.ToLower(s)
	replacements := []struct{ from, to string }{
		{"viii", "8"},
		{"vii", "7"},
		{"iii", "3"},
		{"ii", "2"},
		{"iv", "4"},
		{"vi", "6"},
		{"ix", "9"},
		{"x", "10"},
		{"v", "5"},
	}
	tokens := strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	for i, t := range tokens {
		for _, rep := range replacements {
			if t == rep.from {
				tokens[i] = rep.to
				break
			}
		}
	}
	return strings.Join(tokens, "")
}

func isUpperAcronym(s string) bool {
	if len(s) < 2 || len(s) > 5 {
		return false
	}
	for _, r := range s {
		if !unicode.IsUpper(r) {
			return false
		}
	}
	return true
}

func searchSteamWithAcronym(baseQuery, acronym string) (appID, iconURL, protonTier string) {
	searchURL := fmt.Sprintf("https://store.steampowered.com/api/storesearch/?term=%s&l=english&cc=US", url.QueryEscape(baseQuery))
	client := http.Client{Timeout: 2500 * time.Millisecond}
	resp, err := client.Get(searchURL)
	if err != nil {
		return "", "", ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", ""
	}

	var searchRes struct {
		Total int `json:"total"`
		Items []struct {
			ID        int    `json:"id"`
			Name      string `json:"name"`
			TinyImage string `json:"tiny_image"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&searchRes); err != nil || searchRes.Total == 0 {
		return "", "", ""
	}

	bLow := strings.ToLower(baseQuery)
	acrLow := strings.ToLower(acronym)

	for _, item := range searchRes.Items {
		iLow := strings.ToLower(item.Name)
		if isDLCOrPack(iLow) {
			continue
		}
		if !strings.HasPrefix(iLow, bLow) {
			continue
		}
		rem := strings.TrimPrefix(iLow, bLow)
		rem = strings.TrimLeft(rem, ": -_–—")
		remWords := strings.Fields(rem)
		if len(remWords) == 0 {
			continue
		}

		var initials strings.Builder
		for _, w := range remWords {
			if w != "" {
				initials.WriteByte(w[0])
			}
		}
		if initials.String() == acrLow {
			idStr := strconv.Itoa(item.ID)
			return idStr, item.TinyImage, fetchProtonDBTier(idStr)
		}

		var initialsNoStop strings.Builder
		stopWords := map[string]bool{"of": true, "the": true, "a": true, "an": true, "and": true}
		for _, w := range remWords {
			if !stopWords[w] && w != "" {
				initialsNoStop.WriteByte(w[0])
			}
		}
		if initialsNoStop.String() == acrLow {
			idStr := strconv.Itoa(item.ID)
			return idStr, item.TinyImage, fetchProtonDBTier(idStr)
		}
	}
	return "", "", ""
}

func isConfidentMatch(cleanTitle, itemTitle string) bool {
	markReplacer := strings.NewReplacer("™", "", "®", "", "©", "")
	cleanTitle = strings.TrimSpace(markReplacer.Replace(cleanTitle))
	itemTitle = strings.TrimSpace(markReplacer.Replace(itemTitle))

	tLow := strings.ToLower(cleanTitle)
	iLow := strings.ToLower(itemTitle)

	// Direct equality
	if tLow == iLow {
		return true
	}

	// Filter out DLC packs if cleanTitle doesn't explicitly look for one
	if isDLCOrPack(iLow) && !isDLCOrPack(tLow) {
		return false
	}

	// Collapsed space & roman numeral comparison
	k1 := normalizeTitleKey(cleanTitle)
	k2 := normalizeTitleKey(itemTitle)
	if k1 == k2 {
		return true
	}

	// Strip edition suffixes from itemTitle (e.g. "remastered", "enhanced", "definitiveedition")
	editionSuffixes := []string{
		"remastered",
		"enhanced",
		"definitiveedition",
		"directorscut",
		"gameoftheyearedition",
		"goty",
		"standardedition",
		"specialedition",
		"anniversaryedition",
	}
	k2Stripped := k2
	for _, suf := range editionSuffixes {
		if strings.HasSuffix(k2Stripped, suf) {
			k2Stripped = strings.TrimSuffix(k2Stripped, suf)
			break
		}
	}
	if k1 == k2Stripped {
		return true
	}

	k1Base := strings.TrimRight(k1, "0123456789")
	k1Num := strings.TrimPrefix(k1, k1Base)
	if len(k1Base) >= 5 && strings.HasPrefix(k2Stripped, k1Base) {
		if k1Num != "" && strings.HasSuffix(k2Stripped, k1Num) {
			return true
		}
	}

	// Single-word titles (e.g. "Discovery", "Pioneer"): require exact title or direct subtitle (e.g. "Title: ...")
	if !strings.Contains(tLow, " ") {
		if strings.HasPrefix(iLow, tLow+":") || strings.HasPrefix(iLow, tLow+" -") {
			return true
		}
		return false
	}

	// Multi-word titles: e.g. "TEKKEN 8", "Faster Than Light"
	if strings.HasPrefix(iLow, tLow+":") || strings.HasPrefix(iLow, tLow+" -") || strings.HasPrefix(iLow, tLow+" ") {
		return true
	}

	if strings.HasSuffix(iLow, ": "+tLow) || strings.HasSuffix(iLow, " - "+tLow) || strings.HasSuffix(iLow, tLow) {
		return true
	}

	if strings.Contains(iLow, tLow) || strings.Contains(tLow, iLow) {
		return true
	}

	return false
}

// Universal title resolution via Steam Store Search API (Zero Hardcoding)
func resolveAppIDByName(rawTitle string) (appID, iconURL, protonTier string) {
	cleanTitle := cleanGameTitle(rawTitle)
	if len(cleanTitle) < 2 {
		return "", "", ""
	}

	searchURL := fmt.Sprintf("https://store.steampowered.com/api/storesearch/?term=%s&l=english&cc=US", url.QueryEscape(cleanTitle))
	client := http.Client{Timeout: 2500 * time.Millisecond}
	resp, err := client.Get(searchURL)
	if err != nil {
		return "", "", ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", ""
	}

	var searchRes struct {
		Total int `json:"total"`
		Items []struct {
			ID        int    `json:"id"`
			Name      string `json:"name"`
			TinyImage string `json:"tiny_image"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&searchRes); err == nil && searchRes.Total > 0 && len(searchRes.Items) > 0 {
		for _, item := range searchRes.Items {
			if isConfidentMatch(cleanTitle, item.Name) {
				appID = strconv.Itoa(item.ID)
				iconURL = item.TinyImage
				protonTier = fetchProtonDBTier(appID)
				return appID, iconURL, protonTier
			}
		}
	}

	// Acronym resolution fallback (e.g. "Fallout NV" -> search "Fallout" -> match subtitle initials "NV" for "Fallout: New Vegas")
	words := strings.Fields(cleanTitle)
	if len(words) >= 2 {
		lastWord := words[len(words)-1]
		if isUpperAcronym(lastWord) {
			baseQuery := strings.Join(words[:len(words)-1], " ")
			if resID, resIcon, resTier := searchSteamWithAcronym(baseQuery, lastWord); resID != "" {
				return resID, resIcon, resTier
			}
		}
	}

	// Fallback for notable delisted Steam games
	delistedGames := map[string]string{
		"rocket league": "252950",
	}
	if did, ok := delistedGames[strings.ToLower(cleanTitle)]; ok {
		return did, getSteamIconURL(did), fetchProtonDBTier(did)
	}

	return "", "", ""
}

// findAppIDByInstalledGameFolder dynamically scans Steam library installations on the machine
// to match internal Unreal Engine project folders (e.g. Discovery in THE FINALS, PioneerGame in ARC Raiders)
// with ZERO hardcoded game names.
func findAppIDByInstalledGameFolder(folderName string) (appID string, gameName string) {
	if folderName == "" {
		return "", ""
	}
	fLow := strings.ToLower(strings.TrimSpace(folderName))

	var steamappsDirs []string
	if runtime.GOOS == "windows" {
		steamappsDirs = append(steamappsDirs,
			`C:\Program Files (x86)\Steam\steamapps`,
			`C:\Program Files\Steam\steamapps`,
			`C:\Steam\steamapps`,
			`D:\SteamLibrary\steamapps`,
			`E:\SteamLibrary\steamapps`,
			`F:\SteamLibrary\steamapps`,
		)

		// Check all mounted drives for <Drive>:\SteamLibrary\steamapps
		for _, d := range GetAvailableDrives() {
			cand := filepath.Join(d.Path, "SteamLibrary", "steamapps")
			steamappsDirs = append(steamappsDirs, cand)
		}
	} else {
		home := os.Getenv("HOME")
		steamappsDirs = append(steamappsDirs,
			filepath.Join(home, ".local", "share", "Steam", "steamapps"),
			filepath.Join(home, ".steam", "steam", "steamapps"),
			filepath.Join(home, "mock_windows", "steamapps"),
		)
	}

	seenDirs := make(map[string]bool)
	for _, sDir := range steamappsDirs {
		cleanSDir := filepath.Clean(sDir)
		if seenDirs[strings.ToLower(cleanSDir)] {
			continue
		}
		seenDirs[strings.ToLower(cleanSDir)] = true

		entries, err := os.ReadDir(cleanSDir)
		if err != nil {
			continue
		}

		for _, e := range entries {
			if e.IsDir() || !strings.HasPrefix(strings.ToLower(e.Name()), "appmanifest_") || !strings.HasSuffix(strings.ToLower(e.Name()), ".acf") {
				continue
			}

			// Parse appmanifest_<appid>.acf
			manifestPath := filepath.Join(cleanSDir, e.Name())
			f, err := os.Open(manifestPath)
			if err != nil {
				continue
			}

			var aID, gName, installDir string
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if strings.HasPrefix(line, `"appid"`) {
					parts := strings.Fields(line)
					if len(parts) >= 2 {
						aID = strings.Trim(parts[1], `"`)
					}
				} else if strings.HasPrefix(line, `"name"`) {
					idx := strings.Index(line, `"name"`)
					rest := strings.TrimSpace(line[idx+6:])
					gName = strings.Trim(rest, `" `)
				} else if strings.HasPrefix(line, `"installdir"`) {
					idx := strings.Index(line, `"installdir"`)
					rest := strings.TrimSpace(line[idx+12:])
					installDir = strings.Trim(rest, `" `)
				}
			}
			f.Close()

			if aID == "" || installDir == "" {
				continue
			}

			// Check if common/<installdir>/<folderName> exists
			commonPath := filepath.Join(cleanSDir, "common", installDir)
			targetFolder := filepath.Join(commonPath, folderName)
			if stat, err := os.Stat(targetFolder); err == nil && stat.IsDir() {
				return aID, gName
			}

			// Check case-insensitive folder or binary match inside common/<installdir>
			if subEntries, err := os.ReadDir(commonPath); err == nil {
				for _, sub := range subEntries {
					if sub.IsDir() && strings.ToLower(sub.Name()) == fLow {
						return aID, gName
					}
					if !sub.IsDir() && (strings.ToLower(sub.Name()) == fLow+".exe" || strings.ToLower(sub.Name()) == fLow+"-win64-shipping.exe") {
						return aID, gName
					}
				}
			}
		}
	}

	return "", ""
}

func extractUnrealConfigMeta(gameFolder string) (appID string, projectName string) {
	configBase := filepath.Join(gameFolder, "Saved", "Config")
	var configDirs []string

	// Scan all available subdirectories in Saved/Config (Windows, WindowsClient, WindowsNoEditor, WinGDK, etc.)
	if entries, err := os.ReadDir(configBase); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				configDirs = append(configDirs, filepath.Join(configBase, e.Name()))
			}
		}
	}
	if len(configDirs) == 0 {
		configDirs = []string{
			filepath.Join(gameFolder, "Saved", "Config", "Windows"),
			filepath.Join(gameFolder, "Saved", "Config", "WindowsClient"),
			filepath.Join(gameFolder, "Saved", "Config", "WindowsNoEditor"),
		}
	}

	for _, cDir := range configDirs {
		entries, err := os.ReadDir(cDir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".ini") {
				filePath := filepath.Join(cDir, e.Name())
				f, err := os.Open(filePath)
				if err != nil {
					continue
				}
				scanner := bufio.NewScanner(f)
				for scanner.Scan() {
					line := strings.TrimSpace(scanner.Text())
					lineLow := strings.ToLower(line)
					if strings.HasPrefix(lineLow, "steamappid=") || strings.HasPrefix(lineLow, "steamdevappid=") {
						parts := strings.SplitN(line, "=", 2)
						if len(parts) == 2 {
							val := strings.TrimSpace(parts[1])
							if _, err := strconv.Atoi(val); err == nil && val != "0" {
								appID = val
							}
						}
					}
					if strings.HasPrefix(lineLow, "projectname=") {
						parts := strings.SplitN(line, "=", 2)
						if len(parts) == 2 {
							val := strings.Trim(strings.TrimSpace(parts[1]), `"' `)
							if val != "" {
								projectName = val
							}
						}
					}
				}
				f.Close()
				if appID != "" {
					return appID, projectName
				}
			}
		}
	}
	return appID, projectName
}

func resolveUnrealProjectCodename(folderName string) (appID string, gameName string) {
	norm := strings.ToLower(strings.ReplaceAll(folderName, " ", ""))
	switch norm {
	case "highmoon":
		return "1065310", "Evil West"
	case "pagoda":
		return "3404260", "Dead as Disco"
	case "coltrane":
		return "2437390", "Enginefall"
	}
	return "", ""
}

func findAppIDByMachineCorrelation(folderName string, detectedGames []DetectedGame) string {
	folderClean := cleanGameTitle(folderName)
	fLow := strings.ToLower(folderClean)
	if len(fLow) < 3 {
		return ""
	}

	// 1. Direct title and title token comparison against detected games on machine
	for _, g := range detectedGames {
		if g.AppID == "" {
			continue
		}
		gNameClean := cleanGameTitle(g.Name)
		gNameLow := strings.ToLower(gNameClean)

		// Exact match after cleaning (e.g. "Silent Hill 2" == "SILENT HILL 2")
		if fLow == gNameLow {
			return g.AppID
		}

		// Title word containment (e.g. "Dawnwalker" in "The Blood of Dawnwalker")
		if len(fLow) >= 4 {
			words := strings.Fields(strings.Map(func(r rune) rune {
				if unicode.IsLetter(r) || unicode.IsDigit(r) {
					return unicode.ToLower(r)
				}
				return ' '
			}, g.Name))
			for _, w := range words {
				if w == fLow {
					return g.AppID
				}
			}
		}

		// Collapsed alphanumeric & roman numeral matching (e.g. "kingdomcome2" matches "Kingdom Come: Deliverance II")
		fKey := normalizeTitleKey(folderName)
		gKey := normalizeTitleKey(g.Name)
		if fKey == gKey {
			return g.AppID
		}
		fBase := strings.TrimRight(fKey, "0123456789")
		fNum := strings.TrimPrefix(fKey, fBase)
		if len(fBase) >= 4 && strings.HasPrefix(gKey, fBase) {
			if fNum == "" || strings.HasSuffix(gKey, fNum) {
				return g.AppID
			}
		}
	}

	// 2. Developer / Studio comparison (e.g. "Sandfall" in "Sandfall Interactive" -> Clair Obscur)
	appDevelopersCacheMu.RLock()
	defer appDevelopersCacheMu.RUnlock()

	for _, g := range detectedGames {
		if g.AppID == "" {
			continue
		}
		devs, ok := appDevelopersCache[g.AppID]
		if !ok {
			continue
		}
		for _, dev := range devs {
			devLow := strings.ToLower(dev)
			if strings.Contains(devLow, fLow) || strings.Contains(fLow, devLow) {
				return g.AppID
			}
			for _, word := range strings.Fields(devLow) {
				if len(word) >= 4 && (word == fLow || strings.HasPrefix(word, fLow) || strings.HasPrefix(fLow, word)) {
					return g.AppID
				}
			}
		}
	}

	return ""
}

// findAppIDByDeveloperOrStudio provides backwards compatibility for correlating AppIDs
func findAppIDByDeveloperOrStudio(folderName string, detectedAppIDs []string) string {
	var mockGames []DetectedGame
	for _, id := range detectedAppIDs {
		mockGames = append(mockGames, DetectedGame{AppID: id, Name: getGameName(id, "")})
	}
	return findAppIDByMachineCorrelation(folderName, mockGames)
}

type GameResolveResult struct {
	AppID      string `json:"appid"`
	Name       string `json:"name"`
	IconURL    string `json:"icon_url"`
	ProtonTier string `json:"proton_tier"`
}

func ResolveGameInfo(term string) GameResolveResult {
	term = strings.TrimSpace(term)
	if term == "" {
		return GameResolveResult{}
	}

	// 1. If user typed numeric Steam AppID
	if _, err := strconv.Atoi(term); err == nil {
		name := getGameName(term, "")
		icon := getSteamIconURL(term)
		tier := fetchProtonDBTier(term)
		return GameResolveResult{
			AppID:      term,
			Name:       name,
			IconURL:    icon,
			ProtonTier: tier,
		}
	}

	// 2. Resolve title via dynamic Steam Store search
	appID, icon, tier := resolveAppIDByName(term)
	name := term
	if appID != "" {
		officialName := getGameName(appID, term)
		if officialName != "" && officialName != "Unknown Game" && !strings.HasPrefix(officialName, "Game (") {
			name = officialName
		}
		if icon == "" {
			icon = getSteamIconURL(appID)
		}
	}

	return GameResolveResult{
		AppID:      appID,
		Name:       name,
		IconURL:    icon,
		ProtonTier: tier,
	}
}

func ComputeSuggestedLinuxPath(g DetectedGame) string {
	rel := filepath.ToSlash(g.TargetRelativePath)
	if strings.Contains(strings.ToLower(g.StorageType), "ps4") || strings.HasPrefix(g.ID, "ps4_") {
		return "~/.local/share/shadPS4/" + rel
	}
	if g.StorageType == "Steam Userdata" || strings.HasPrefix(g.ID, "steam_userdata_") {
		parts := strings.Split(rel, "/")
		if len(parts) >= 4 {
			acc := parts[len(parts)-2]
			app := parts[len(parts)-1]
			return fmt.Sprintf("~/.local/share/Steam/userdata/%s/%s", acc, app)
		}
		return "~/.local/share/Steam/userdata/" + rel
	}
	if g.SaveType == SaveTypePrefix {
		if g.AppID != "" {
			return fmt.Sprintf("~/.steam/steam/steamapps/compatdata/%s/pfx/drive_c/%s", g.AppID, rel)
		}
		return fmt.Sprintf("~/Games/%s/drive_c/%s", sanitizeFilename(g.Name), rel)
	}
	return fmt.Sprintf("<Game Folder>/%s", rel)
}

func sanitizeFilename(s string) string {
	s = strings.ReplaceAll(s, " ", "_")
	var result strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			result.WriteRune(r)
		}
	}
	return result.String()
}

func splitCamelCase(s string) string {
	var buf strings.Builder
	runes := []rune(s)
	for i, r := range runes {
		if i > 0 && unicode.IsUpper(r) && unicode.IsLower(runes[i-1]) {
			buf.WriteRune(' ')
		} else if i > 0 && unicode.IsDigit(r) && unicode.IsLetter(runes[i-1]) {
			buf.WriteRune(' ')
		} else if i > 0 && unicode.IsLetter(r) && unicode.IsDigit(runes[i-1]) {
			buf.WriteRune(' ')
		}
		buf.WriteRune(r)
	}
	return buf.String()
}

func expandGamingAcronyms(title string) string {
	tLow := strings.ToLower(strings.TrimSpace(title))
	acronymMap := map[string]string{
		"gtav":   "Grand Theft Auto V",
		"gta v":  "Grand Theft Auto V",
		"gta5":   "Grand Theft Auto 5",
		"gta 5":  "Grand Theft Auto 5",
		"gtaiv":  "Grand Theft Auto IV",
		"gta iv": "Grand Theft Auto IV",
		"gta4":   "Grand Theft Auto 4",
		"gta 4":  "Grand Theft Auto 4",
		"gta":    "Grand Theft Auto",
		"tlou2":  "The Last of Us Part II",
		"tlou 2": "The Last of Us Part II",
		"tlou":   "The Last of Us",
		"gow3":   "God of War III",
		"gow 3":  "God of War III",
		"gow":    "God of War",
		"re4":    "Resident Evil 4",
		"re2":    "Resident Evil 2",
		"re3":    "Resident Evil 3",
		"re7":    "Resident Evil 7",
		"re8":    "Resident Evil Village",
	}
	if expanded, ok := acronymMap[tLow]; ok {
		return expanded
	}
	return title
}

func cleanGameTitle(raw string) string {
	clean := raw
	clean = strings.NewReplacer("™", "", "®", "", "©", "").Replace(clean)
	suffixes := []string{" Game", " Client", " Release", "_Data", "-Win64-Shipping", "Win64"}
	for _, s := range suffixes {
		clean = strings.TrimSuffix(clean, s)
	}
	clean = strings.ReplaceAll(clean, "_", " ")
	clean = strings.ReplaceAll(clean, "-", " ")
	if !strings.Contains(clean, " ") {
		clean = splitCamelCase(clean)
	}
	clean = strings.TrimSpace(clean)
	return expandGamingAcronyms(clean)
}

func getGameName(appid, defaultName string) string {
	initCache()
	if appid == "" {
		if defaultName != "" {
			return defaultName
		}
		return "Unknown Game"
	}

	appNameCacheMu.RLock()
	name, found := appNameCache[appid]
	appNameCacheMu.RUnlock()
	if found && name != "" {
		return name
	}

	// Fetch from Steam Store API
	onlineName := fetchSteamNameOnline(appid)
	if onlineName != "" {
		appNameCacheMu.Lock()
		appNameCache[appid] = onlineName
		appNameCacheMu.Unlock()
		go saveCache()
		return onlineName
	}

	if defaultName != "" {
		return defaultName
	}
	return "Game (" + appid + ")"
}

// ScanInstalledGames searches known emulator, profile, and portable paths
func ScanInstalledGames(customScanRoots []string) ([]DetectedGame, error) {
	var games []DetectedGame
	seenPaths := make(map[string]bool)

	SetProgress("scan", 5, "Scanning Windows Public Documents & offline emulators...", 0, 100, "")

	// 1. Emulator, Profile & Documents Scans
	scanPublicDocuments(&games, seenPaths)
	SetProgress("scan", 20, "Scanning user AppData and Saved Games...", 20, 100, "")
	scanUserAppData(&games, seenPaths)
	SetProgress("scan", 35, "Scanning Steam client userdata saves...", 35, 100, "")
	scanSteamUserdata(&games, seenPaths)
	SetProgress("scan", 50, "Scanning user Documents...", 50, 100, "")
	scanUserDocumentsGames(&games, seenPaths)

	// 2. In-Tree & Drive-wide Scans (Lies of P, portable emulators, Bloodborne bbport, custom roots)
	scanRoots := []string{}
	if runtime.GOOS == "windows" {
		SetProgress("scan", 60, "Scanning drive roots and installed game directories...", 60, 100, "")
		scanRoots = append(scanRoots, `C:\Games`, `D:\Games`, `E:\Games`)

		// Scan top-level game folders on all mounted drives (e.g. D:\Bloodborne, E:\Emulators)
		drives := GetAvailableDrives()
		for _, d := range drives {
			entries, err := os.ReadDir(d.Path)
			if err != nil {
				continue
			}
			for _, e := range entries {
				if !e.IsDir() {
					continue
				}
				name := e.Name()
				lower := strings.ToLower(name)
				// Filter out Windows OS, protected system folders, program files, users, etc.
				if strings.HasPrefix(lower, "$") ||
					strings.HasPrefix(lower, ".") ||
					strings.Contains(lower, "system volume information") ||
					strings.Contains(lower, "windows") ||
					strings.Contains(lower, "recovery") ||
					strings.Contains(lower, "msocache") ||
					strings.Contains(lower, "perflogs") ||
					strings.Contains(lower, "boot") ||
					strings.Contains(lower, "program files") ||
					strings.Contains(lower, "programdata") ||
					strings.Contains(lower, "users") ||
					strings.Contains(lower, "appdata") {
					continue
				}
				fullPath := filepath.Join(d.Path, name)
				if isRepackOrInstallerFolder(fullPath, name) {
					continue
				}
				scanRoots = append(scanRoots, fullPath)
			}
		}
	} else {
		// Mock testing roots for linux dev
		mockDocs := filepath.Join(os.Getenv("HOME"), "mock_windows", "Documents")
		if _, err := os.Stat(mockDocs); err == nil {
			scanRoots = append(scanRoots, filepath.Join(os.Getenv("HOME"), "mock_windows"))
		}
	}

	// Deduplicate scanRoots case-insensitively
	seenRoots := make(map[string]bool)
	var uniqueScanRoots []string
	for _, r := range scanRoots {
		k := strings.ToLower(filepath.Clean(r))
		if !seenRoots[k] {
			seenRoots[k] = true
			uniqueScanRoots = append(uniqueScanRoots, r)
		}
	}
	scanRoots = uniqueScanRoots

	for _, root := range scanRoots {
		scanInTreeDirectory(root, &games, seenPaths)
	}

	SetProgress("scan", 80, "Scanning custom directories...", 80, 100, "")
	for _, custom := range customScanRoots {
		scanCustomPath(custom, &games, seenPaths)
	}

	// Dynamic machine correlation pass: match any studio or custom-named folders against all detected titles and studios
	SetProgress("scan", 90, "Correlating game titles and studio folders across machine...", 90, 100, "")
	for i := range games {
		g := &games[i]
		if g.AppID == "" {
			codeAppID, codeName := findAppIDByInstalledGameFolder(g.Name)
			if codeAppID != "" {
				g.AppID = codeAppID
				g.Name = codeName
				g.IconURL = getSteamIconURL(codeAppID)
				g.ProtonTier = fetchProtonDBTier(codeAppID)
				continue
			}
			matched := findAppIDByMachineCorrelation(g.Name, games)
			if matched != "" {
				g.AppID = matched
				g.Name = getGameName(matched, g.Name)
				g.IconURL = getSteamIconURL(matched)
				g.ProtonTier = fetchProtonDBTier(matched)
			}
		}
	}

	// Steam Cloud detection pass: identify legitimate Steam installed games that use Steam Cloud
	SetProgress("scan", 95, "Checking Steam Cloud & user config classifications...", 95, 100, "")
	for i := range games {
		if games[i].AppID != "" && games[i].ProtonTier == "" {
			games[i].ProtonTier = fetchProtonDBTier(games[i].AppID)
		}
		if isLegitimateSteamCloudGame(games[i]) {
			games[i].IsSteamCloud = true
		}
		if isClientConfigOrSettings(games[i]) {
			games[i].IsUserSettings = true
		}
	}

	ClearProgress(fmt.Sprintf("Scan complete. Found %d game saves.", len(games)))
	return games, nil
}

var (
	steamCloudCacheMu sync.RWMutex
	steamCloudCache   = make(map[string]bool)
)

func isSteamCloudSupported(appID string) bool {
	if appID == "" {
		return false
	}
	steamCloudCacheMu.RLock()
	val, exists := steamCloudCache[appID]
	steamCloudCacheMu.RUnlock()
	if exists {
		return val
	}

	url := fmt.Sprintf("https://store.steampowered.com/api/appdetails?appids=%s&filters=categories", appID)
	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	var result map[string]struct {
		Success bool `json:"success"`
		Data    struct {
			Categories []struct {
				ID          int    `json:"id"`
				Description string `json:"description"`
			} `json:"categories"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false
	}

	hasCloud := false
	if entry, ok := result[appID]; ok && entry.Success {
		for _, cat := range entry.Data.Categories {
			if cat.ID == 23 || strings.EqualFold(cat.Description, "Steam Cloud") {
				hasCloud = true
				break
			}
		}
	}

	steamCloudCacheMu.Lock()
	steamCloudCache[appID] = hasCloud
	steamCloudCacheMu.Unlock()
	return hasCloud
}

func hasLocalSteamManifest(appID string) bool {
	if appID == "" {
		return false
	}
	drives := GetAvailableDrives()
	for _, d := range drives {
		candidates := []string{
			filepath.Join(d.Path, "Program Files (x86)", "Steam", "steamapps", fmt.Sprintf("appmanifest_%s.acf", appID)),
			filepath.Join(d.Path, "Program Files", "Steam", "steamapps", fmt.Sprintf("appmanifest_%s.acf", appID)),
			filepath.Join(d.Path, "Steam", "steamapps", fmt.Sprintf("appmanifest_%s.acf", appID)),
			filepath.Join(d.Path, "SteamLibrary", "steamapps", fmt.Sprintf("appmanifest_%s.acf", appID)),
			filepath.Join(d.Path, "Games", "SteamLibrary", "steamapps", fmt.Sprintf("appmanifest_%s.acf", appID)),
		}
		for _, cand := range candidates {
			if _, err := os.Stat(cand); err == nil {
				return true
			}
		}
	}
	home := os.Getenv("HOME")
	if home != "" {
		candidates := []string{
			filepath.Join(home, ".steam", "steam", "steamapps", fmt.Sprintf("appmanifest_%s.acf", appID)),
			filepath.Join(home, ".local", "share", "Steam", "steamapps", fmt.Sprintf("appmanifest_%s.acf", appID)),
		}
		for _, cand := range candidates {
			if _, err := os.Stat(cand); err == nil {
				return true
			}
		}
	}
	return false
}

func isLegitimateSteamCloudGame(g DetectedGame) bool {
	// Offline emulators and PS4 emulators are NEVER Steam Cloud managed
	lowerStorage := strings.ToLower(g.StorageType)
	if strings.Contains(lowerStorage, "rune") ||
		strings.Contains(lowerStorage, "codex") ||
		strings.Contains(lowerStorage, "goldberg") ||
		strings.Contains(lowerStorage, "empress") ||
		strings.Contains(lowerStorage, "flt") ||
		strings.Contains(lowerStorage, "ps4") ||
		strings.Contains(lowerStorage, "bbport") {
		return false
	}

	if g.AppID == "" {
		return false
	}

	if hasLocalSteamManifest(g.AppID) && isSteamCloudSupported(g.AppID) {
		return true
	}
	return false
}

func isClientConfigOrSettings(g DetectedGame) bool {
	lowName := strings.ToLower(g.Name)
	lowPath := strings.ToLower(g.SourcePath)

	if strings.Contains(lowName, "the finals") || strings.Contains(lowPath, "discovery") ||
		strings.Contains(lowName, "rocket league") || strings.Contains(lowPath, "rocket league") ||
		strings.Contains(lowName, "rainbow six") || strings.Contains(lowPath, "rainbow six") {
		return true
	}

	entries, err := os.ReadDir(g.SourcePath)
	if err == nil && len(entries) <= 3 {
		allConfig := true
		for _, e := range entries {
			if e.IsDir() {
				allConfig = false
				break
			}
			ext := strings.ToLower(filepath.Ext(e.Name()))
			if ext != ".ini" && ext != ".cfg" && ext != ".txt" && ext != ".log" && ext != ".xml" {
				allConfig = false
				break
			}
		}
		if allConfig && len(entries) > 0 {
			return true
		}
	}
	return false
}

func isOnlineLiveServiceGame(name string, appID string, path string) bool {
	lowName := strings.ToLower(name)
	lowPath := strings.ToLower(path)

	// AppID checks
	switch appID {
	case "2073850", // THE FINALS
		"359550",  // Tom Clancy's Rainbow Six Siege
		"1085660", // Destiny 2
		"578080",  // PUBG: BATTLEGROUNDS
		"1172470", // Apex Legends
		"2357570", // Overwatch 2
		"730",     // Counter-Strike 2
		"553850",  // Helldivers 2
		"2767030", // Marvel Rivals
		"1422450", // Deadlock
		"736220",  // Squad 44 / Post Scriptum
		"2011840", // Squad Game / Playtest
		"393380",  // Squad
		"1808500", // ARC Raiders
		"291550",  // Brawlhalla
		"2325290", // Sky: Children of the Light
		"2281730", // Combat Master
		"1097150", // Fall Guys
		"2221490": // Tom Clancy's The Division 2
		return true
	}

	// Keywords check in name and path
	onlineKeywords := []string{
		"the finals", "discovery",
		"rainbow six",
		"fortnite", "fortnitegame",
		"marvel rivals", "marvelrivals",
		"deadlock",
		"squad 44", "post scriptum",
		"squadgame", "squad game",
		"pioneergame", "arc raiders",
		"brawlhalla",
		"sky children of the light",
		"combat master",
		"fall guys", "fallguys", "fallguys_client",
		"the division 2", "the division", "division 2",
		"valorant", "vanguard",
		"league of legends",
		"apex legends",
		"destiny 2",
		"escape from tarkov",
		"pubg",
		"overwatch",
	}

	for _, kw := range onlineKeywords {
		if strings.Contains(lowName, kw) || strings.Contains(lowPath, kw) {
			return true
		}
	}

	if strings.Contains(lowPath, `\marvel\saved`) || strings.Contains(lowPath, `/marvel/saved`) {
		return true
	}
	return false
}

func collectDetectedAppIDs(games []DetectedGame) []string {
	var ids []string
	seen := make(map[string]bool)
	for _, g := range games {
		if g.AppID != "" && !seen[g.AppID] {
			seen[g.AppID] = true
			ids = append(ids, g.AppID)
		}
	}
	return ids
}

func scanPublicDocuments(games *[]DetectedGame, seen map[string]bool) {
	var publicDocPath string
	if runtime.GOOS == "windows" {
		publicDir := os.Getenv("PUBLIC")
		if publicDir == "" {
			publicDir = `C:\Users\Public`
		}
		publicDocPath = filepath.Join(publicDir, "Documents")
	} else {
		// Mock/test fallback for linux dev environment
		publicDocPath = filepath.Join(os.Getenv("HOME"), "mock_windows", "Public", "Documents")
	}

	if _, err := os.Stat(publicDocPath); os.IsNotExist(err) {
		return
	}

	// Pattern A: RUNE
	runeBase := filepath.Join(publicDocPath, "Steam", "RUNE")
	scanAppIDSubdirs(runeBase, "RUNE", "users/Public/Documents/Steam/RUNE", games, seen)

	// Pattern B: CODEX
	codexBase := filepath.Join(publicDocPath, "Steam", "CODEX")
	scanAppIDSubdirs(codexBase, "CODEX", "users/Public/Documents/Steam/CODEX", games, seen)

	// Pattern C: EMPRESS
	empressBase := filepath.Join(publicDocPath, "EMPRESS")
	scanAppIDSubdirs(empressBase, "EMPRESS", "users/Public/Documents/EMPRESS", games, seen)
}

func getSteamPathFromRegistry() string {
	if runtime.GOOS != "windows" {
		return ""
	}
	cmd := exec.Command("reg", "query", `HKCU\Software\Valve\Steam`, "/v", "SteamPath")
	cmd.SysProcAttr = hideWindowAttr()
	if out, err := cmd.Output(); err == nil {
		scanner := bufio.NewScanner(strings.NewReader(string(out)))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(strings.ToLower(line), "steampath") {
				parts := strings.Fields(line)
				if len(parts) >= 3 {
					rawPath := strings.Join(parts[2:], " ")
					cleanPath := filepath.Clean(rawPath)
					if stat, err := os.Stat(cleanPath); err == nil && stat.IsDir() {
						return cleanPath
					}
				}
			}
		}
	}
	return ""
}

func getSteamInstallRoots() []string {
	var roots []string
	if runtime.GOOS == "windows" {
		if regPath := getSteamPathFromRegistry(); regPath != "" {
			roots = append(roots, regPath)
		}
		roots = append(roots,
			`C:\Program Files (x86)\Steam`,
			`C:\Program Files\Steam`,
			`C:\Steam`,
		)
		for _, d := range GetAvailableDrives() {
			roots = append(roots,
				filepath.Join(d.Path, "Steam"),
				filepath.Join(d.Path, "Program Files (x86)", "Steam"),
				filepath.Join(d.Path, "Program Files", "Steam"),
			)
		}
	} else {
		home := os.Getenv("HOME")
		if home != "" {
			roots = append(roots,
				filepath.Join(home, ".local", "share", "Steam"),
				filepath.Join(home, ".steam", "steam"),
				filepath.Join(home, "mock_windows", "Steam"),
				filepath.Join(home, "mock_windows", "Program Files (x86)", "Steam"),
			)
		}
	}

	seen := make(map[string]bool)
	var unique []string
	for _, r := range roots {
		clean := filepath.Clean(r)
		k := strings.ToLower(clean)
		if !seen[k] {
			seen[k] = true
			unique = append(unique, clean)
		}
	}
	return unique
}

func scanSteamUserdata(games *[]DetectedGame, seen map[string]bool) {
	steamRoots := getSteamInstallRoots()
	for _, root := range steamRoots {
		userdataDir := filepath.Join(root, "userdata")
		uEntries, err := os.ReadDir(userdataDir)
		if err != nil {
			continue
		}

		for _, ue := range uEntries {
			if !ue.IsDir() {
				continue
			}
			accountID := ue.Name()
			idNum, err := strconv.Atoi(accountID)
			if err != nil || idNum <= 0 {
				continue
			}

			userPath := filepath.Join(userdataDir, accountID)
			appEntries, err := os.ReadDir(userPath)
			if err != nil {
				continue
			}

			for _, ae := range appEntries {
				if !ae.IsDir() {
					continue
				}
				appID := ae.Name()
				aIDNum, err := strconv.Atoi(appID)
				if err != nil || aIDNum <= 0 {
					continue
				}
				if appID == "7" || appID == "760" {
					continue
				}

				appPath := filepath.Join(userPath, appID)
				canonical := strings.ToLower(filepath.Clean(appPath))
				if seen[canonical] || seen[appPath] {
					continue
				}

				if isDummySaveDir(appPath) {
					continue
				}

				count, size := getDirStats(appPath)
				if count == 0 {
					continue
				}

				seen[canonical] = true
				seen[appPath] = true

				gameTitle := getGameName(appID, "")
				iconURL := getSteamIconURL(appID)
				protonTier := fetchProtonDBTier(appID)

				targetRel := filepath.ToSlash(filepath.Join("Program Files (x86)/Steam/userdata", accountID, appID))

				*games = append(*games, DetectedGame{
					ID:                 fmt.Sprintf("steam_userdata_%s_%s", accountID, appID),
					Name:               gameTitle,
					AppID:              appID,
					SaveType:           SaveTypePrefix,
					StorageType:          "Steam Userdata",
					SourcePath:         appPath,
					TargetRelativePath: targetRel,
					FileCount:          count,
					SizeBytes:          size,
					IconURL:            iconURL,
					ProtonTier:         protonTier,
				})
			}
		}
	}
}

func scanUserAppData(games *[]DetectedGame, seen map[string]bool) {
	var appData, localAppData string
	if runtime.GOOS == "windows" {
		appData = os.Getenv("APPDATA")
		localAppData = os.Getenv("LOCALAPPDATA")
	} else {
		mockRoot := filepath.Join(os.Getenv("HOME"), "mock_windows", "AppData")
		appData = filepath.Join(mockRoot, "Roaming")
		localAppData = filepath.Join(mockRoot, "Local")
	}

	// Goldberg
	if appData != "" {
		goldbergBase := filepath.Join(appData, "Goldberg SteamEmu Saves")
		scanAppIDSubdirs(goldbergBase, "Goldberg", "users/steamuser/AppData/Roaming/Goldberg SteamEmu Saves", games, seen)

		// FLT
		fltBase := filepath.Join(appData, "FLT")
		scanAppIDSubdirs(fltBase, "FLT", "users/steamuser/AppData/Roaming/FLT", games, seen)
	}

	// SKIDROW
	if localAppData != "" {
		skidrowBase := filepath.Join(localAppData, "SKIDROW")
		scanAppIDSubdirs(skidrowBase, "SKIDROW", "users/steamuser/AppData/Local/SKIDROW", games, seen)

		// Unreal Engine 4 & 5 AppData/Local Saves (e.g. TEKKEN 8, Hogwarts Legacy, Palworld, Remnant II)
		scanLocalAppDataUnrealGames(localAppData, games, seen)
	}

	// FromSoftware Direct AppData Saves (Elden Ring, Dark Souls III, Sekiro, AC6)
	if appData != "" {
		fromSoftGames := []struct {
			dirName  string
			gameName string
			appID    string
		}{
			{"EldenRing", "Elden Ring (Character Saves)", "1245620"},
			{"DarkSoulsIII", "Dark Souls III", "374320"},
			{"Sekiro", "Sekiro: Shadows Die Twice", "814380"},
			{"ArmoredCore6", "Armored Core VI", "1888160"},
		}

		for _, fg := range fromSoftGames {
			saveDir := filepath.Join(appData, fg.dirName)
			if stat, err := os.Stat(saveDir); err == nil && stat.IsDir() && !seen[saveDir] {
				count, size := getDirStats(saveDir)
				if count > 0 {
					seen[saveDir] = true
					*games = append(*games, DetectedGame{
						ID:                 "fromsoft_" + strings.ToLower(fg.dirName),
						Name:               fg.gameName,
						AppID:              fg.appID,
						SaveType:           SaveTypePrefix,
						StorageType:          "AppData Profile",
						SourcePath:         saveDir,
						TargetRelativePath: "users/steamuser/AppData/Roaming/" + fg.dirName,
						FileCount:          count,
						SizeBytes:          size,
						IconURL:            getSteamIconURL(fg.appID),
						ProtonTier:         fetchProtonDBTier(fg.appID),
					})
				}
			}
		}

		// Indie / Ren'Py profile in AppData: The Coffin of Andy and Leyley
		coffinDir := filepath.Join(appData, "CoffinAndyLeyley")
		if stat, err := os.Stat(coffinDir); err == nil && stat.IsDir() && !seen[coffinDir] {
			count, size := getDirStats(coffinDir)
			if count > 0 {
				seen[coffinDir] = true
				appID := "2378900"
				*games = append(*games, DetectedGame{
					ID:                 "appdata_coffinandyleyley",
					Name:               "The Coffin of Andy and Leyley",
					AppID:              appID,
					SaveType:           SaveTypePrefix,
					StorageType:          "AppData Profile",
					SourcePath:         coffinDir,
					TargetRelativePath: "users/steamuser/AppData/Roaming/CoffinAndyLeyley",
					FileCount:          count,
					SizeBytes:          size,
					IconURL:            getSteamIconURL(appID),
					ProtonTier:         fetchProtonDBTier(appID),
				})
			}
		}

		// Emulators in AppData
		scanAppDataEmulators(appData, games, seen)
	}

	// Unity games in AppData/LocalLow
	var localLow string
	if runtime.GOOS == "windows" {
		userProf := os.Getenv("USERPROFILE")
		if userProf != "" {
			localLow = filepath.Join(userProf, "AppData", "LocalLow")
		}
	} else {
		mockRoot := filepath.Join(os.Getenv("HOME"), "mock_windows", "AppData")
		localLow = filepath.Join(mockRoot, "LocalLow")
	}

	if localLow != "" {
		scanUserAppDataLocalLow(localLow, games, seen)
	}

	// Emulators in Documents
	scanDocumentsEmulators(games, seen)
}

func scanUserAppDataLocalLow(localLow string, games *[]DetectedGame, seen map[string]bool) {
	if stat, err := os.Stat(localLow); err != nil || !stat.IsDir() {
		return
	}

	companyEntries, err := os.ReadDir(localLow)
	if err != nil {
		return
	}

	ignoredLocalLow := map[string]bool{
		"microsoft":            true,
		"amd":                  true,
		"intel":                true,
		"nvidia":               true,
		"oracle":               true,
		"sun":                  true,
		"temp":                 true,
		"itop screen recorder": true,
		"advanced systemcare":  true,
		"iobit":                true,
		"ime":                  true,
		"internet explorer":    true,
		"dxcache":              true,
	}

	for _, c := range companyEntries {
		if !c.IsDir() {
			continue
		}
		cName := c.Name()
		cLow := strings.ToLower(cName)
		if ignoredLocalLow[cLow] || strings.HasPrefix(cLow, "$") || strings.HasPrefix(cLow, ".") {
			continue
		}

		cPath := filepath.Join(localLow, cName)
		gameEntries, err := os.ReadDir(cPath)
		if err != nil {
			continue
		}

		for _, g := range gameEntries {
			if !g.IsDir() {
				continue
			}
			gName := g.Name()
			gLow := strings.ToLower(gName)
			if ignoredLocalLow[gLow] || gLow == "dxcache" || gLow == "unity" {
				continue
			}
			if strings.HasSuffix(gLow, " backup") || strings.HasSuffix(gLow, "_backup") || strings.HasSuffix(gLow, "_bak") {
				continue
			}

			gamePath := filepath.Join(cPath, gName)
			canonical := strings.ToLower(filepath.Clean(gamePath))
			if seen[canonical] || seen[gamePath] {
				continue
			}

			if isDummySaveDir(gamePath) {
				continue
			}

			count, size := getDirStats(gamePath)
			if count == 0 || size > 500*1024*1024 {
				continue
			}

			cleanName := gName
			if strings.EqualFold(cleanName, "MiSideFull") {
				cleanName = "MiSide"
			}

			appID, iconURL, protonTier := resolveAppIDByName(cleanName)
			if appID != "" {
				cleanName = getGameName(appID, cleanName)
			}

			if isOnlineLiveServiceGame(cleanName, appID, gamePath) {
				continue
			}

			seen[canonical] = true
			seen[gamePath] = true

			*games = append(*games, DetectedGame{
				ID:                 "locallow_" + sanitizeID(cName) + "_" + sanitizeID(gName),
				Name:               cleanName,
				AppID:              appID,
				SaveType:           SaveTypePrefix,
				StorageType:          "Unity (AppData/LocalLow)",
				SourcePath:         gamePath,
				TargetRelativePath: filepath.ToSlash(filepath.Join("users/steamuser/AppData/LocalLow", cName, gName)),
				FileCount:          count,
				SizeBytes:          size,
				IconURL:            iconURL,
				ProtonTier:         protonTier,
			})
		}
	}
}

func scanLocalAppDataUnrealGames(localAppData string, games *[]DetectedGame, seen map[string]bool) {
	if localAppData == "" {
		return
	}
	entries, err := os.ReadDir(localAppData)
	if err != nil {
		return
	}

	ignoredLocalFolders := map[string]bool{
		"microsoft":                true,
		"programs":                 true,
		"packages":                 true,
		"temp":                     true,
		"google":                   true,
		"nvidia":                   true,
		"nvidia corporation":       true,
		"amd":                      true,
		"intel":                    true,
		"discord":                  true,
		"spotify":                  true,
		"steam":                    true,
		"epicgameslauncher":        true,
		"bravesoftware":            true,
		"mozilla":                  true,
		"pip":                      true,
		"npm":                      true,
		"yarn":                     true,
		"deno":                     true,
		"electron":                 true,
		"github":                   true,
		"git":                      true,
		"crashdumps":               true,
		"connecteddevicesplatform": true,
		"comms":                    true,
		"publishers":               true,
		"elevateddiagnostics":      true,
		"iconcache":                true,
		"cefdcache":                true,
		"d3dscache":                true,
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		nameLow := strings.ToLower(name)
		if ignoredLocalFolders[nameLow] || strings.HasPrefix(nameLow, "$") || strings.HasPrefix(nameLow, ".") {
			continue
		}

		gameFolder := filepath.Join(localAppData, name)

		// 1. Standard Unreal Engine SaveGames: <LocalAppData>\<GameName>\Saved\SaveGames
		saveGamesDir := filepath.Join(gameFolder, "Saved", "SaveGames")
		if stat, err := os.Stat(saveGamesDir); err == nil && stat.IsDir() {
			if seen[saveGamesDir] {
				continue
			}
			count, size := getDirStats(saveGamesDir)
			if count > 0 {
				seen[saveGamesDir] = true

				// Check local ini for AppID / ProjectName
				iniAppID, iniProject := extractUnrealConfigMeta(gameFolder)
				appID := iniAppID
				cleanName := name
				if iniProject != "" {
					cleanName = iniProject
				}

				// Check known project codenames (HighMoon -> Evil West, Pagoda -> Dead as Disco)
				if appID == "" {
					cID, cName := resolveUnrealProjectCodename(name)
					if cID != "" {
						appID = cID
						cleanName = cName
					}
				}

				// Check installed Steam games dynamically (e.g. Discovery in THE FINALS, PioneerGame in ARC Raiders)
				if appID == "" {
					codeAppID, codeName := findAppIDByInstalledGameFolder(name)
					if codeAppID != "" {
						appID = codeAppID
						cleanName = codeName
					}
				}

				// Machine-wide correlation with detected AppIDs (e.g. matching Sandfall -> Clair Obscur, Dawnwalker -> The Blood of Dawnwalker)
				if appID == "" {
					matched := findAppIDByMachineCorrelation(name, *games)
					if matched != "" {
						appID = matched
					}
				}

				var iconURL, protonTier string
				if appID != "" {
					cleanName = getGameName(appID, cleanName)
					iconURL = getSteamIconURL(appID)
					protonTier = fetchProtonDBTier(appID)
				} else {
					resID, resIcon, resTier := resolveAppIDByName(cleanName)
					if resID != "" {
						appID = resID
						iconURL = resIcon
						protonTier = resTier
						cleanName = getGameName(appID, cleanName)
					}
				}

				if isOnlineLiveServiceGame(cleanName, appID, saveGamesDir) {
					continue
				}

				*games = append(*games, DetectedGame{
					ID:                 "local_ue_" + sanitizeID(name),
					Name:               cleanName,
					AppID:              appID,
					SaveType:           SaveTypePrefix,
					StorageType:          "Unreal Engine (AppData/Local)",
					SourcePath:         saveGamesDir,
					TargetRelativePath: "users/steamuser/AppData/Local/" + name + "/Saved/SaveGames",
					FileCount:          count,
					SizeBytes:          size,
					IconURL:            iconURL,
					ProtonTier:         protonTier,
				})
			}
			continue
		}

		// 2. Direct Saved folder if it contains saves: <LocalAppData>\<GameName>\Saved
		savedDir := filepath.Join(gameFolder, "Saved")
		if stat, err := os.Stat(savedDir); err == nil && stat.IsDir() {
			if seen[savedDir] {
				continue
			}
			if hasGameSaveIndicators(savedDir) {
				count, size := getDirStats(savedDir)
				if count > 0 && size < 500*1024*1024 {
					seen[savedDir] = true

					iniAppID, iniProject := extractUnrealConfigMeta(gameFolder)
					appID := iniAppID
					cleanName := name
					if iniProject != "" {
						cleanName = iniProject
					}

					if appID == "" {
						cID, cName := resolveUnrealProjectCodename(name)
						if cID != "" {
							appID = cID
							cleanName = cName
						}
					}

					if appID == "" {
						codeAppID, codeName := findAppIDByInstalledGameFolder(name)
						if codeAppID != "" {
							appID = codeAppID
							cleanName = codeName
						}
					}

					if appID == "" {
						matched := findAppIDByMachineCorrelation(name, *games)
						if matched != "" {
							appID = matched
						}
					}

					var iconURL, protonTier string
					if appID != "" {
						cleanName = getGameName(appID, cleanName)
						iconURL = getSteamIconURL(appID)
						protonTier = fetchProtonDBTier(appID)
					} else {
						resID, resIcon, resTier := resolveAppIDByName(cleanName)
						if resID != "" {
							appID = resID
							iconURL = resIcon
							protonTier = resTier
							cleanName = getGameName(appID, cleanName)
						}
					}

					if isOnlineLiveServiceGame(cleanName, appID, savedDir) {
						continue
					}

					*games = append(*games, DetectedGame{
						ID:                 "local_ue_" + sanitizeID(name),
						Name:               cleanName,
						AppID:              appID,
						SaveType:           SaveTypePrefix,
						StorageType:          "Unreal Engine (AppData/Local)",
						SourcePath:         savedDir,
						TargetRelativePath: "users/steamuser/AppData/Local/" + name + "/Saved",
						FileCount:          count,
						SizeBytes:          size,
						IconURL:            iconURL,
						ProtonTier:         protonTier,
					})
				}
			}
		}
	}
}

func scanAppDataEmulators(appData string, games *[]DetectedGame, seen map[string]bool) {
	// Ryujinx (Switch)
	ryuSave := filepath.Join(appData, "Ryujinx", "bis", "user", "save")
	if stat, err := os.Stat(ryuSave); err == nil && stat.IsDir() && !seen[ryuSave] {
		count, size := getDirStats(ryuSave)
		if count > 0 {
			seen[ryuSave] = true
			*games = append(*games, DetectedGame{
				ID:                 "emu_ryujinx",
				Name:               "Nintendo Switch Saves (Ryujinx)",
				SaveType:           SaveTypeInTree,
				StorageType:          "Switch Emulator",
				SourcePath:         ryuSave,
				TargetRelativePath: "Ryujinx/bis/user/save",
				FileCount:          count,
				SizeBytes:          size,
			})
		}
	}

	// Yuzu / Suyu (Switch)
	for _, yuzuName := range []string{"yuzu", "suyu"} {
		yuzuSave := filepath.Join(appData, yuzuName, "nand", "user", "save")
		if stat, err := os.Stat(yuzuSave); err == nil && stat.IsDir() && !seen[yuzuSave] {
			count, size := getDirStats(yuzuSave)
			if count > 0 {
				seen[yuzuSave] = true
				*games = append(*games, DetectedGame{
					ID:                 "emu_" + yuzuName,
					Name:               fmt.Sprintf("Nintendo Switch Saves (%s)", strings.Title(yuzuName)),
					SaveType:           SaveTypeInTree,
					StorageType:          "Switch Emulator",
					SourcePath:         yuzuSave,
					TargetRelativePath: yuzuName + "/nand/user/save",
					FileCount:          count,
					SizeBytes:          size,
				})
			}
		}
	}

	// Citra (3DS)
	citraSave := filepath.Join(appData, "Citra", "sdmc")
	if stat, err := os.Stat(citraSave); err == nil && stat.IsDir() && !seen[citraSave] {
		count, size := getDirStats(citraSave)
		if count > 0 {
			seen[citraSave] = true
			*games = append(*games, DetectedGame{
				ID:                 "emu_citra",
				Name:               "Nintendo 3DS Saves (Citra)",
				SaveType:           SaveTypeInTree,
				StorageType:          "3DS Emulator",
				SourcePath:         citraSave,
				TargetRelativePath: "citra-emu/sdmc",
				FileCount:          count,
				SizeBytes:          size,
			})
		}
	}

	// RPCS3 (PlayStation 3)
	rpcs3Home := filepath.Join(appData, "RPCS3", "dev_hdd0", "home")
	if stat, err := os.Stat(rpcs3Home); err == nil && stat.IsDir() {
		scanRPCS3Home(rpcs3Home, games, seen)
	}

	// PCSX2 (PS2) AppData Qt location
	pcsx2AppMem := filepath.Join(appData, "PCSX2", "memcards")
	if stat, err := os.Stat(pcsx2AppMem); err == nil && stat.IsDir() && !seen[pcsx2AppMem] {
		count, size := getDirStats(pcsx2AppMem)
		if count > 0 {
			seen[pcsx2AppMem] = true
			*games = append(*games, DetectedGame{
				ID:                 "emu_pcsx2_appdata",
				Name:               "PlayStation 2 Memory Cards (PCSX2)",
				SaveType:           SaveTypeInTree,
				StorageType:          "PS2 Emulator",
				SourcePath:         pcsx2AppMem,
				TargetRelativePath: "PCSX2/memcards",
				FileCount:          count,
				SizeBytes:          size,
			})
		}
	}

	// Dolphin in AppData
	dolphinAppData := filepath.Join(appData, "Dolphin Emulator", "GC")
	if stat, err := os.Stat(dolphinAppData); err == nil && stat.IsDir() && !seen[dolphinAppData] {
		count, size := getDirStats(dolphinAppData)
		if count > 0 {
			seen[dolphinAppData] = true
			*games = append(*games, DetectedGame{
				ID:                 "emu_dolphin_appdata",
				Name:               "GameCube & Wii Saves (Dolphin)",
				SaveType:           SaveTypeInTree,
				StorageType:          "Wii/GC Emulator",
				SourcePath:         dolphinAppData,
				TargetRelativePath: "dolphin-emu/GC",
				FileCount:          count,
				SizeBytes:          size,
			})
		}
	}

	// Cemu (Wii U) in Roaming and LocalAppData
	cemuRoaming := filepath.Join(appData, "Cemu", "mlc01", "usr", "save")
	scanCemuSaves(cemuRoaming, "cemu/mlc01/usr/save", games, seen)

	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData != "" {
		cemuLocal := filepath.Join(localAppData, "Cemu", "mlc01", "usr", "save")
		scanCemuSaves(cemuLocal, "cemu/mlc01/usr/save", games, seen)
	}
}

func parseCemuTitleName(metaXMLPath string) string {
	data, err := os.ReadFile(metaXMLPath)
	if err != nil {
		return ""
	}
	var meta struct {
		XMLName     xml.Name `xml:"menu"`
		LongNameEn  string   `xml:"longname_en"`
		ShortNameEn string   `xml:"shortname_en"`
		TitleID     string   `xml:"title_id"`
	}
	if err := xml.Unmarshal(data, &meta); err != nil {
		return ""
	}
	title := strings.TrimSpace(meta.LongNameEn)
	if title == "" {
		title = strings.TrimSpace(meta.ShortNameEn)
	}
	if title != "" {
		title = strings.ReplaceAll(title, "\r\n", ": ")
		title = strings.ReplaceAll(title, "\n", ": ")
		return title
	}
	return ""
}

func scanCemuSaves(cemuSaveDir, relBase string, games *[]DetectedGame, seen map[string]bool) {
	if stat, err := os.Stat(cemuSaveDir); err != nil || !stat.IsDir() {
		return
	}

	highEntries, err := os.ReadDir(cemuSaveDir)
	if err != nil {
		return
	}

	foundAnyTitle := false
	for _, high := range highEntries {
		if !high.IsDir() {
			continue
		}
		highName := strings.ToLower(high.Name())
		if highName == "system" {
			continue
		}

		highDir := filepath.Join(cemuSaveDir, high.Name())
		lowEntries, err := os.ReadDir(highDir)
		if err != nil {
			continue
		}

		for _, low := range lowEntries {
			if !low.IsDir() {
				continue
			}
			gameDir := filepath.Join(highDir, low.Name())
			if seen[gameDir] {
				continue
			}

			count, size := getDirStats(gameDir)
			if count == 0 {
				continue
			}

			metaXML := filepath.Join(gameDir, "meta", "meta.xml")
			parsedTitle := parseCemuTitleName(metaXML)
			displayName := parsedTitle
			if displayName == "" {
				displayName = fmt.Sprintf("Wii U Title (%s)", low.Name())
			}

			seen[gameDir] = true
			foundAnyTitle = true

			*games = append(*games, DetectedGame{
				ID:                 "cemu_" + strings.ToLower(high.Name()) + "_" + strings.ToLower(low.Name()),
				Name:               fmt.Sprintf("%s (Cemu)", displayName),
				SaveType:           SaveTypeInTree,
				StorageType:          "Wii U Emulator",
				SourcePath:         gameDir,
				TargetRelativePath: filepath.ToSlash(filepath.Join(relBase, high.Name(), low.Name())),
				FileCount:          count,
				SizeBytes:          size,
			})
		}
	}

	if !foundAnyTitle && !seen[cemuSaveDir] {
		count, size := getDirStats(cemuSaveDir)
		if count > 0 {
			seen[cemuSaveDir] = true
			*games = append(*games, DetectedGame{
				ID:                 "emu_cemu",
				Name:               "Wii U Saves (Cemu)",
				SaveType:           SaveTypeInTree,
				StorageType:          "Wii U Emulator",
				SourcePath:         cemuSaveDir,
				TargetRelativePath: filepath.ToSlash(relBase),
				FileCount:          count,
				SizeBytes:          size,
			})
		}
	}
}

func scanRPCS3Home(homeDir string, games *[]DetectedGame, seen map[string]bool) {
	users, err := os.ReadDir(homeDir)
	if err != nil {
		return
	}
	for _, u := range users {
		if !u.IsDir() {
			continue
		}
		savedataPath := filepath.Join(homeDir, u.Name(), "savedata")
		entries, err := os.ReadDir(savedataPath)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			gameDir := filepath.Join(savedataPath, e.Name())
			if seen[gameDir] {
				continue
			}
			count, size := getDirStats(gameDir)
			if count == 0 {
				continue
			}
			seen[gameDir] = true
			titleID := e.Name()
			gameName := extractPARAMSFOTitle(filepath.Join(gameDir, "PARAM.SFO"))
			if gameName == "" {
				gameName = "PS3 Save (" + titleID + ")"
			}
			*games = append(*games, DetectedGame{
				ID:                 "rpcs3_" + strings.ToLower(titleID),
				Name:               gameName,
				SaveType:           SaveTypeInTree,
				StorageType:          "PS3 Emulator (RPCS3)",
				SourcePath:         gameDir,
				TargetRelativePath: filepath.ToSlash(filepath.Join("RPCS3", "dev_hdd0", "home", u.Name(), "savedata", titleID)),
				FileCount:          count,
				SizeBytes:          size,
			})
		}
	}
}

func extractPARAMSFOTitle(sfoPath string) string {
	data, err := os.ReadFile(sfoPath)
	if err != nil || len(data) < 20 {
		return ""
	}
	idx := bytes.Index(data, []byte("TITLE\x00"))
	if idx == -1 {
		return ""
	}
	// Locate clean printable string after title header in SFO data table
	sub := data[idx:]
	parts := bytes.Split(sub, []byte{0})
	for _, p := range parts {
		trimmed := strings.TrimSpace(string(p))
		if len(trimmed) >= 3 && trimmed != "TITLE" && isASCIIPrintable(trimmed) {
			return trimmed
		}
	}
	return ""
}

func isASCIIPrintable(s string) bool {
	for _, r := range s {
		if r < 32 || r > 126 {
			return false
		}
	}
	return true
}

func scanDocumentsEmulators(games *[]DetectedGame, seen map[string]bool) {
	var docsDir string
	if runtime.GOOS == "windows" {
		userProf := os.Getenv("USERPROFILE")
		if userProf != "" {
			docsDir = filepath.Join(userProf, "Documents")
		}
	} else {
		userProf := os.Getenv("USERPROFILE")
		if userProf != "" {
			docsDir = filepath.Join(userProf, "Documents")
		} else {
			docsDir = filepath.Join(os.Getenv("HOME"), "mock_windows", "Documents")
		}
	}

	if docsDir == "" {
		return
	}

	// PCSX2 (PS2 Memcards)
	pcsx2Mem := filepath.Join(docsDir, "PCSX2", "memcards")
	if stat, err := os.Stat(pcsx2Mem); err == nil && stat.IsDir() && !seen[pcsx2Mem] {
		count, size := getDirStats(pcsx2Mem)
		if count > 0 {
			seen[pcsx2Mem] = true
			*games = append(*games, DetectedGame{
				ID:                 "emu_pcsx2",
				Name:               "PlayStation 2 Memory Cards (PCSX2)",
				SaveType:           SaveTypeInTree,
				StorageType:          "PS2 Emulator",
				SourcePath:         pcsx2Mem,
				TargetRelativePath: "PCSX2/memcards",
				FileCount:          count,
				SizeBytes:          size,
			})
		}
	}

	// DuckStation (PS1 Memcards)
	duckMem := filepath.Join(docsDir, "DuckStation", "memcards")
	if stat, err := os.Stat(duckMem); err == nil && stat.IsDir() && !seen[duckMem] {
		count, size := getDirStats(duckMem)
		if count > 0 {
			seen[duckMem] = true
			*games = append(*games, DetectedGame{
				ID:                 "emu_duckstation",
				Name:               "PlayStation 1 Memory Cards (DuckStation)",
				SaveType:           SaveTypeInTree,
				StorageType:          "PS1 Emulator",
				SourcePath:         duckMem,
				TargetRelativePath: "duckstation/memcards",
				FileCount:          count,
				SizeBytes:          size,
			})
		}
	}

	// Dolphin (GameCube / Wii)
	dolphinUser := filepath.Join(docsDir, "Dolphin Emulator", "GC")
	if stat, err := os.Stat(dolphinUser); err == nil && stat.IsDir() && !seen[dolphinUser] {
		count, size := getDirStats(dolphinUser)
		if count > 0 {
			seen[dolphinUser] = true
			*games = append(*games, DetectedGame{
				ID:                 "emu_dolphin",
				Name:               "GameCube & Wii Saves (Dolphin)",
				SaveType:           SaveTypeInTree,
				StorageType:          "Wii/GC Emulator",
				SourcePath:         dolphinUser,
				TargetRelativePath: "dolphin-emu/GC",
				FileCount:          count,
				SizeBytes:          size,
			})
		}
	}

	// Xenia (Xbox 360)
	xeniaContent := filepath.Join(docsDir, "Xenia", "content")
	if stat, err := os.Stat(xeniaContent); err == nil && stat.IsDir() && !seen[xeniaContent] {
		count, size := getDirStats(xeniaContent)
		if count > 0 {
			seen[xeniaContent] = true
			*games = append(*games, DetectedGame{
				ID:                 "emu_xenia",
				Name:               "Xbox 360 Saves (Xenia)",
				SaveType:           SaveTypeInTree,
				StorageType:          "Xbox 360 Emulator",
				SourcePath:         xeniaContent,
				TargetRelativePath: "xenia/content",
				FileCount:          count,
				SizeBytes:          size,
			})
		}
	}
}

func getWindowsPersonalDocumentsDir() string {
	if runtime.GOOS != "windows" {
		return ""
	}
	cmd := exec.Command("reg", "query", `HKCU\Software\Microsoft\Windows\CurrentVersion\Explorer\User Shell Folders`, "/v", "Personal")
	cmd.SysProcAttr = hideWindowAttr()
	out, err := cmd.Output()
	if err == nil {
		scanner := bufio.NewScanner(strings.NewReader(string(out)))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(strings.ToLower(line), "personal") {
				parts := strings.Fields(line)
				if len(parts) >= 3 {
					rawPath := strings.Join(parts[2:], " ")
					expanded := os.ExpandEnv(rawPath)
					if stat, err := os.Stat(expanded); err == nil && stat.IsDir() {
						return expanded
					}
				}
			}
		}
	}
	return ""
}

// scanUserDocumentsGames scans Documents root and Documents/My Games for game save folders
func scanUserDocumentsGames(games *[]DetectedGame, seen map[string]bool) {
	nonDocsOneDrive := map[string]bool{
		"desktop": true, "pictures": true, "photos": true, "images": true,
		"videos": true, "music": true, "downloads": true, "backup": true,
		"backups": true, "temp": true, "tmp": true, "attachments": true,
		"vault": true, "personal vault": true, "سطح المكتب": true,
		"الصور": true, "الموسيقى": true, "الفيديوهات": true, "مقاطع الفيديو": true,
		"التنزيلات": true,
	}

	var docsDirs []string
	if runtime.GOOS == "windows" {
		userProf := os.Getenv("USERPROFILE")
		if userProf != "" {
			docsDirs = append(docsDirs, filepath.Join(userProf, "Documents"))
			docsDirs = append(docsDirs, filepath.Join(userProf, "OneDrive", "Documents"))
			docsDirs = append(docsDirs, filepath.Join(userProf, "Saved Games"))

			// Query official active Documents folder from Windows registry (handles localization like "المستندات", "Documentos", etc.)
			if personal := getWindowsPersonalDocumentsDir(); personal != "" {
				docsDirs = append(docsDirs, personal)
			}

			// Also scan any subdirectories inside OneDrive (catches localized folders like OneDrive\المستندات, skipping non-documents)
			oneDriveDir := filepath.Join(userProf, "OneDrive")
			if odEntries, err := os.ReadDir(oneDriveDir); err == nil {
				for _, ode := range odEntries {
					if ode.IsDir() {
						if nonDocsOneDrive[strings.ToLower(ode.Name())] {
							continue
						}
						docsDirs = append(docsDirs, filepath.Join(oneDriveDir, ode.Name()))
					}
				}
			}
		}
	} else {
		docsDirs = append(docsDirs, filepath.Join(os.Getenv("HOME"), "mock_windows", "Documents"))
		if userProf := os.Getenv("USERPROFILE"); userProf != "" {
			docsDirs = append(docsDirs, filepath.Join(userProf, "Documents"))
			docsDirs = append(docsDirs, filepath.Join(userProf, "Saved Games"))
			oneDriveDir := filepath.Join(userProf, "OneDrive")
			if odEntries, err := os.ReadDir(oneDriveDir); err == nil {
				for _, ode := range odEntries {
					if ode.IsDir() {
						if nonDocsOneDrive[strings.ToLower(ode.Name())] {
							continue
						}
						docsDirs = append(docsDirs, filepath.Join(oneDriveDir, ode.Name()))
					}
				}
			}
		}
	}

	seenDocs := make(map[string]bool)
	var uniqueDocs []string
	for _, d := range docsDirs {
		k := strings.ToLower(filepath.Clean(d))
		if !seenDocs[k] {
			seenDocs[k] = true
			uniqueDocs = append(uniqueDocs, d)
		}
	}

	for _, docsDir := range uniqueDocs {
		scanSpecificDocsDir(docsDir, games, seen)
	}
}

func scanSpecificDocsDir(docsDir string, games *[]DetectedGame, seen map[string]bool) {
	if docsDir == "" {
		return
	}

	entries, err := os.ReadDir(docsDir)
	if err != nil {
		return
	}

	ignoredDocsFolders := map[string]bool{
		"my music":                true,
		"my pictures":             true,
		"my videos":               true,
		"custom office templates": true,
		"feedbackhub":             true,
		"powertoys":               true,
		"zoom":                    true,
		"audacity":                true,
		"adobe":                   true,
		"outlook files":           true,
		"visual studio 2022":      true,
		"visual studio 2019":      true,
		"windowspowershell":       true,
		"desktop":                 true,
		"pcsx2":                   true,
		"duckstation":             true,
		"dolphin emulator":        true,
		"sharex":                  true,
		"obs":                     true,
		"streamlabs":              true,
		"lightshot":               true,
		"greenshot":               true,
		"snagit":                  true,
		"bandicam":                true,
		"backup":                  true,
		"backups":                 true,
		"temp":                    true,
		"tmp":                     true,
		"downloads":               true,
		"سطح المكتب":              true,
		"الصور":                   true,
		"الموسيقى":                true,
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		nameLow := strings.ToLower(name)

		if ignoredDocsFolders[nameLow] {
			continue
		}

		subPath := filepath.Join(docsDir, name)

		// 1. Check "My Games" subfolder (e.g. Skyrim, Fallout, Borderlands, Civ, Outlast, Rocket League)
		if nameLow == "my games" {
			mgEntries, err := os.ReadDir(subPath)
			if err == nil {
				for _, mg := range mgEntries {
					if mg.IsDir() {
						mgPath := filepath.Join(subPath, mg.Name())
						canonical := strings.ToLower(filepath.Clean(mgPath))
						if seen[canonical] || seen[mgPath] {
							continue
						}
						count, size := getDirStats(mgPath)
						if count > 0 {
							seen[canonical] = true
							seen[mgPath] = true
							appID, iconURL, protonTier := resolveAppIDByName(mg.Name())
							cleanName := mg.Name()
							if appID != "" {
								cleanName = getGameName(appID, cleanName)
							}

							if isOnlineLiveServiceGame(cleanName, appID, mgPath) {
								continue
							}

							*games = append(*games, DetectedGame{
								ID:                 "doc_mygames_" + sanitizeID(mg.Name()),
								Name:               cleanName,
								AppID:              appID,
								SaveType:           SaveTypePrefix,
								StorageType:          "Documents (My Games)",
								SourcePath:         mgPath,
								TargetRelativePath: "users/steamuser/Documents/My Games/" + mg.Name(),
								FileCount:          count,
								SizeBytes:          size,
								IconURL:            iconURL,
								ProtonTier:         protonTier,
							})
						}
					}
				}
			}
			continue
		}

		// 2. Handle publisher subfolders (Rockstar Games, Telltale Games, KONAMI, CAPCOM)
		if nameLow == "rockstar games" || nameLow == "telltale games" || nameLow == "konami" || nameLow == "capcom" {
			pubEntries, err := os.ReadDir(subPath)
			if err == nil {
				for _, pe := range pubEntries {
					if pe.IsDir() {
						peName := pe.Name()
						peLow := strings.ToLower(peName)
						if ignoredGenericGameFolders[peLow] || peLow == "launcher" || peLow == "social club" {
							continue
						}
						pePath := filepath.Join(subPath, peName)
						canonical := strings.ToLower(filepath.Clean(pePath))
						if seen[canonical] || seen[pePath] {
							continue
						}
						count, size := getDirStats(pePath)
						if count > 0 && size < 500*1024*1024 {
							seen[canonical] = true
							seen[pePath] = true
							appID, iconURL, protonTier := resolveAppIDByName(peName)
							cleanName := peName
							if appID != "" {
								cleanName = getGameName(appID, cleanName)
							}
							if isOnlineLiveServiceGame(cleanName, appID, pePath) {
								continue
							}
							*games = append(*games, DetectedGame{
								ID:                 "doc_" + sanitizeID(name) + "_" + sanitizeID(peName),
								Name:               cleanName,
								AppID:              appID,
								SaveType:           SaveTypePrefix,
								StorageType:          "Documents (" + name + ")",
								SourcePath:         pePath,
								TargetRelativePath: "users/steamuser/Documents/" + name + "/" + peName,
								FileCount:          count,
								SizeBytes:          size,
								IconURL:            iconURL,
								ProtonTier:         protonTier,
							})
						}
					}
				}
			}
			continue
		}

		// 3. Handle CPY_SAVES
		if nameLow == "cpy_saves" {
			cpyBase := filepath.Join(subPath, "CPY")
			if stat, err := os.Stat(cpyBase); err == nil && stat.IsDir() {
				scanAppIDSubdirs(cpyBase, "CPY", "users/steamuser/Documents/CPY_SAVES/CPY", games, seen)
			}
			continue
		}

		// 4. Direct game folders in Documents (e.g. Darkest, Witcher 3, TLOU2, Tsushima)
		canonical := strings.ToLower(filepath.Clean(subPath))
		if seen[canonical] || seen[subPath] {
			continue
		}

		if hasGameSaveIndicators(subPath) {
			count, size := getDirStats(subPath)
			if count > 0 {
				seen[canonical] = true
				seen[subPath] = true
				appID, iconURL, protonTier := resolveAppIDByName(name)
				cleanName := name
				if appID != "" {
					cleanName = getGameName(appID, cleanName)
				}

				isSavedGames := strings.EqualFold(filepath.Base(docsDir), "Saved Games")
				storageLabel := "Documents Profile"
				targetRel := "users/steamuser/Documents/" + name
				idPrefix := "doc_"

				if isSavedGames {
					storageLabel = "Saved Games"
					targetRel = "users/steamuser/Saved Games/" + name
					idPrefix = "savedgames_"
				}

				if isOnlineLiveServiceGame(cleanName, appID, subPath) {
					continue
				}

				*games = append(*games, DetectedGame{
					ID:                 idPrefix + sanitizeID(name),
					Name:               cleanName,
					AppID:              appID,
					SaveType:           SaveTypePrefix,
					StorageType:          storageLabel,
					SourcePath:         subPath,
					TargetRelativePath: targetRel,
					FileCount:          count,
					SizeBytes:          size,
					IconURL:            iconURL,
					ProtonTier:         protonTier,
				})
			}
		}
	}
}

var ignoredGenericGameFolders = map[string]bool{
	"user":         true,
	"users":        true,
	"cache":        true,
	"caches":       true,
	"logs":         true,
	"log":          true,
	"bin":          true,
	"data":         true,
	"default":      true,
	"temp":         true,
	"tmp":          true,
	"backup":       true,
	"backups":      true,
	"config":       true,
	"settings":     true,
	"system":       true,
	"debug":        true,
	"crash":        true,
	"shaders":      true,
	"shader_cache": true,
}

var ignoredGameResourceFolders = map[string]bool{
	"audio":            true,
	"sound":            true,
	"sounds":           true,
	"music":            true,
	"video":            true,
	"videos":           true,
	"movies":           true,
	"localization":     true,
	"locale":           true,
	"locales":          true,
	"lang":             true,
	"languages":        true,
	"scripts":          true,
	"script":           true,
	"fx":               true,
	"props":            true,
	"loot":             true,
	"shared":           true,
	"trinkets":         true,
	"textures":         true,
	"models":           true,
	"meshes":           true,
	"engine":           true,
	"binaries":         true,
	"bin":              true,
	"easyanticheat":    true,
	"battleye":         true,
	"directx":          true,
	"redist":           true,
	"_commonredist":    true,
	"support":          true,
	"plugins":          true,
	"analytics":        true,
	"crash":            true,
	"game":             true,
	"_windows":         true,
	"_manifest":        true,
	"fairlightconcept": true,
	"fe_flow":          true,
}

func isRepackOrInstallerFolder(dir, name string) bool {
	low := strings.ToLower(name)
	if strings.Contains(low, "repack") ||
		strings.Contains(low, "fitgirl") ||
		strings.Contains(low, "dodi") ||
		strings.Contains(low, "kaos") ||
		strings.Contains(low, "elamigos") ||
		strings.Contains(low, "xatab") ||
		strings.Contains(low, "furmark") ||
		strings.Contains(low, "benchmark") {
		return true
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	hasSetup := false
	hasDataBin := false
	for _, e := range entries {
		enLow := strings.ToLower(e.Name())
		if enLow == "setup.exe" || enLow == "quicksfv.exe" || enLow == "quicksfv.ini" {
			hasSetup = true
		}
		if strings.HasPrefix(enLow, "data") && strings.HasSuffix(enLow, ".bin") {
			hasDataBin = true
		}
	}
	return hasSetup && hasDataBin
}

func isGameAssetDirectory(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	gameAssetExts := map[string]bool{
		".pak": true, ".vpk": true, ".cas": true, ".rpa": true, ".bundle": true,
		".psarc": true, ".iso": true, ".cue": true, ".cpk": true, ".wad": true,
		".assets": true, ".bk2": true, ".bik": true, ".usm": true,
	}
	for _, e := range entries {
		if !e.IsDir() {
			ext := strings.ToLower(filepath.Ext(e.Name()))
			if gameAssetExts[ext] {
				return true
			}
		}
	}
	return false
}

func hasProgramBinaries(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() {
			ext := strings.ToLower(filepath.Ext(e.Name()))
			if ext == ".exe" || ext == ".dll" || ext == ".sys" {
				return true
			}
		}
	}
	return false
}

func isDummySaveDir(dir string) bool {
	hasRealSave := false
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		nameLow := strings.ToLower(info.Name())
		if nameLow == "steam_autocloud.vdf" || nameLow == "desktop.ini" || nameLow == "thumbs.db" {
			return nil
		}
		hasRealSave = true
		return filepath.SkipAll
	})
	return !hasRealSave
}

func hasGameSaveIndicators(dir string) bool {
	if isGameAssetDirectory(dir) || hasProgramBinaries(dir) {
		return false
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	// Real PC game saves use .sav, .save, .dat, .json, .xml.
	// NOTE: .bin is excluded as it is heavily used by repack installers (data1.bin) and game assets.
	saveExts := map[string]bool{
		".sav": true, ".save": true, ".dat": true, ".json": true, ".xml": true,
	}

	for _, e := range entries {
		name := strings.ToLower(e.Name())
		if name == "cache" || name == "shader" || name == "shaders" {
			continue
		}
		if ignoredGenericGameFolders[name] || ignoredGameResourceFolders[name] {
			continue
		}
		if strings.HasPrefix(name, "profile") || strings.HasPrefix(name, "save") || strings.HasPrefix(name, "slot") {
			return true
		}
		// SteamID64 subfolder check (e.g. 7656119...)
		if e.IsDir() && len(name) >= 16 && strings.HasPrefix(name, "7656119") {
			return true
		}
		ext := strings.ToLower(filepath.Ext(name))
		if saveExts[ext] {
			return true
		}
		if e.IsDir() && (strings.Contains(name, "save") || strings.Contains(name, "profile") || strings.Contains(name, "user")) {
			return true
		}
	}
	return false
}

var ps4TitleMap = map[string]string{
	"CUSA00207": "Bloodborne",
	"CUSA03173": "Bloodborne: The Old Hunters",
	"CUSA01715": "God of War III Remastered",
	"CUSA00080": "DriveClub",
	"CUSA00419": "Infamous Second Son",
	"CUSA00552": "The Last of Us Remastered",
	"CUSA00557": "God of War III Remastered",
	"CUSA07408": "God of War",
	"CUSA00133": "Killzone Shadow Fall",
	"CUSA00138": "Knack",
	"CUSA01019": "Gravity Rush Remastered",
	"CUSA01127": "Until Dawn",
	"CUSA03617": "Uncharted 4: A Thief's End",
	"CUSA00194": "Shadow of the Colossus",
	"CUSA05670": "Persona 5",
	"CUSA09263": "Gran Turismo Sport",
}

func scanPS4SaveDir(savedataPath, gameFolder string, games *[]DetectedGame, seen map[string]bool) {
	entries, err := os.ReadDir(savedataPath)
	if err != nil {
		return
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		subPath := filepath.Join(savedataPath, e.Name())

		if isCUSAID(e.Name()) {
			addPS4Game(subPath, gameFolder, e.Name(), games, seen)
		} else {
			// Subfolder could be user ID like "1" or "10000000"
			nested, err := os.ReadDir(subPath)
			if err == nil {
				for _, n := range nested {
					if n.IsDir() && isCUSAID(n.Name()) {
						nestedPath := filepath.Join(subPath, n.Name())
						addPS4Game(nestedPath, gameFolder, n.Name(), games, seen)
					}
				}
			}
		}
	}
}

func isCUSAID(name string) bool {
	upper := strings.ToUpper(name)
	return strings.HasPrefix(upper, "CUSA")
}

func addPS4Game(saveDir, gameFolder, titleID string, games *[]DetectedGame, seen map[string]bool) {
	if seen[saveDir] {
		return
	}
	count, size := getDirStats(saveDir)
	if count == 0 {
		return
	}
	seen[saveDir] = true

	titleIDUpper := strings.ToUpper(titleID)
	baseGameName := ps4TitleMap[titleIDUpper]
	if baseGameName == "" {
		baseGameName = "PS4 Title (" + titleIDUpper + ")"
	}
	displayName := baseGameName + " (PS4 Emulator / bbport)"

	relPath, _ := filepath.Rel(gameFolder, saveDir)
	appID, iconURL, protonTier := resolveAppIDByName(baseGameName)

	*games = append(*games, DetectedGame{
		ID:                 "ps4_" + strings.ToLower(titleIDUpper),
		Name:               displayName,
		AppID:              appID,
		SaveType:           SaveTypeInTree,
		StorageType:          "PS4 Emulator (ShadPS4 / bbport)",
		SourcePath:         saveDir,
		TargetRelativePath: filepath.ToSlash(relPath),
		FileCount:          count,
		SizeBytes:          size,
		IconURL:            iconURL,
		ProtonTier:         protonTier,
	})
}

func scanAppIDSubdirs(basePath, storageType, targetRelBase string, games *[]DetectedGame, seen map[string]bool) {
	entries, err := os.ReadDir(basePath)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		appID := entry.Name()
		// Must be a valid numeric Steam AppID > 0 (skip emulator config folders like "settings", "temp", and dummy "0")
		idNum, err := strconv.Atoi(appID)
		if err != nil || idNum <= 0 {
			continue
		}
		fullPath := filepath.Join(basePath, appID)
		if seen[fullPath] {
			continue
		}

		fileCount, size := getDirStats(fullPath)
		if fileCount == 0 {
			continue
		}

		gameName := getGameName(appID, "")
		if isOnlineLiveServiceGame(gameName, appID, fullPath) {
			continue
		}

		seen[fullPath] = true
		gameName = fmt.Sprintf("%s (%s Emulator Profile)", gameName, storageType)

		*games = append(*games, DetectedGame{
			ID:                 storageType + "_" + appID,
			Name:               gameName,
			AppID:              appID,
			SaveType:           SaveTypePrefix,
			StorageType:          storageType,
			SourcePath:         fullPath,
			TargetRelativePath: filepath.ToSlash(filepath.Join(targetRelBase, appID)),
			FileCount:          fileCount,
			SizeBytes:          size,
			IconURL:            getSteamIconURL(appID),
			ProtonTier:         fetchProtonDBTier(appID),
		})
	}
}

var ignoredSystemDirs = map[string]bool{
	"system volume information": true,
	"$recycle.bin":              true,
	"recovery":                  true,
	"windows":                   true,
	"programdata":               true,
	"msocache":                  true,
	"$windows.~bt":              true,
	"$windows.~ws":              true,
}

func scanInTreeDirectory(rootDir string, games *[]DetectedGame, seen map[string]bool) {
	if _, err := os.Stat(rootDir); os.IsNotExist(err) {
		return
	}

	// 1. Direct check: Is rootDir itself a PS4 / ShadPS4 directory?
	if stat, err := os.Stat(filepath.Join(rootDir, "savedata")); err == nil && stat.IsDir() {
		scanPS4SaveDir(filepath.Join(rootDir, "savedata"), rootDir, games, seen)
	} else if stat, err := os.Stat(filepath.Join(rootDir, "user", "savedata")); err == nil && stat.IsDir() {
		scanPS4SaveDir(filepath.Join(rootDir, "user", "savedata"), rootDir, games, seen)
	} else if isCUSAID(filepath.Base(rootDir)) {
		addPS4Game(rootDir, filepath.Dir(rootDir), filepath.Base(rootDir), games, seen)
	}

	entries, err := os.ReadDir(rootDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		nameLow := strings.ToLower(entry.Name())
		if ignoredSystemDirs[nameLow] || ignoredGenericGameFolders[nameLow] || ignoredGameResourceFolders[nameLow] || strings.HasPrefix(nameLow, "$") || strings.HasPrefix(nameLow, ".") {
			continue
		}
		if strings.HasPrefix(nameLow, "profile_") || strings.HasPrefix(nameLow, "slot_") {
			continue
		}
		// Skip official Steam library folders by default (Steam Cloud handles these)
		if nameLow == "steamlibrary" || nameLow == "steamapps" {
			continue
		}
		gameFolder := filepath.Join(rootDir, entry.Name())
		if isRepackOrInstallerFolder(gameFolder, entry.Name()) {
			continue
		}

		// Direct check on gameFolder for PS4 emulator
		if stat, err := os.Stat(filepath.Join(gameFolder, "savedata")); err == nil && stat.IsDir() {
			scanPS4SaveDir(filepath.Join(gameFolder, "savedata"), gameFolder, games, seen)
		} else if stat, err := os.Stat(filepath.Join(gameFolder, "user", "savedata")); err == nil && stat.IsDir() {
			scanPS4SaveDir(filepath.Join(gameFolder, "user", "savedata"), gameFolder, games, seen)
		}

		// Portable RPCS3 (PS3)
		if stat, err := os.Stat(filepath.Join(gameFolder, "dev_hdd0", "home")); err == nil && stat.IsDir() {
			scanRPCS3Home(filepath.Join(gameFolder, "dev_hdd0", "home"), games, seen)
		}

		// Portable PCSX2 (PS2)
		if stat, err := os.Stat(filepath.Join(gameFolder, "memcards")); err == nil && stat.IsDir() && !seen[filepath.Join(gameFolder, "memcards")] {
			count, size := getDirStats(filepath.Join(gameFolder, "memcards"))
			if count > 0 {
				seen[filepath.Join(gameFolder, "memcards")] = true
				*games = append(*games, DetectedGame{
					ID:                 "emu_pcsx2_portable_" + sanitizeID(entry.Name()),
					Name:               fmt.Sprintf("PlayStation 2 Memory Cards (PCSX2 - %s)", entry.Name()),
					SaveType:           SaveTypeInTree,
					StorageType:          "PS2 Emulator",
					SourcePath:         filepath.Join(gameFolder, "memcards"),
					TargetRelativePath: filepath.ToSlash(filepath.Join(entry.Name(), "memcards")),
					FileCount:          count,
					SizeBytes:          size,
				})
			}
		}

		// Portable Ryujinx (Switch)
		if stat, err := os.Stat(filepath.Join(gameFolder, "portable", "bis", "user", "save")); err == nil && stat.IsDir() {
			pSave := filepath.Join(gameFolder, "portable", "bis", "user", "save")
			if !seen[pSave] {
				count, size := getDirStats(pSave)
				if count > 0 {
					seen[pSave] = true
					*games = append(*games, DetectedGame{
						ID:                 "emu_ryujinx_portable_" + sanitizeID(entry.Name()),
						Name:               fmt.Sprintf("Nintendo Switch Saves (Ryujinx - %s)", entry.Name()),
						SaveType:           SaveTypeInTree,
						StorageType:          "Switch Emulator",
						SourcePath:         pSave,
						TargetRelativePath: filepath.ToSlash(filepath.Join(entry.Name(), "portable", "bis", "user", "save")),
						FileCount:          count,
						SizeBytes:          size,
					})
				}
			}
		}

		// Portable Cemu (Wii U)
		if stat, err := os.Stat(filepath.Join(gameFolder, "mlc01", "usr", "save")); err == nil && stat.IsDir() {
			pSave := filepath.Join(gameFolder, "mlc01", "usr", "save")
			scanCemuSaves(pSave, filepath.Join(entry.Name(), "mlc01", "usr", "save"), games, seen)
		}

		// Portable Xenia (Xbox 360)
		if stat, err := os.Stat(filepath.Join(gameFolder, "content", "0000000000000000")); err == nil && stat.IsDir() {
			pSave := filepath.Join(gameFolder, "content")
			if !seen[pSave] {
				count, size := getDirStats(pSave)
				if count > 0 {
					seen[pSave] = true
					*games = append(*games, DetectedGame{
						ID:                 "emu_xenia_portable_" + sanitizeID(entry.Name()),
						Name:               fmt.Sprintf("Xbox 360 Saves (Xenia - %s)", entry.Name()),
						SaveType:           SaveTypeInTree,
						StorageType:          "Xbox 360 Emulator",
						SourcePath:         pSave,
						TargetRelativePath: filepath.ToSlash(filepath.Join(entry.Name(), "content")),
						FileCount:          count,
						SizeBytes:          size,
					})
				}
			}
		}

		// Walk inside gameFolder looking strictly for valid in-tree save paths
		filepath.Walk(gameFolder, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				low := strings.ToLower(info.Name())
				if low == "steamapps" {
					return filepath.SkipDir
				}
				if ignoredGameResourceFolders[low] || ignoredGenericGameFolders[low] {
					return filepath.SkipDir
				}
				if low == "cache" || low == "shader" || low == "trophy_cache" || low == "captures" || low == "temp" {
					return filepath.SkipDir
				}
			}

			// PS4 / ShadPS4 / bbport savedata
			if info.IsDir() && strings.EqualFold(info.Name(), "savedata") {
				parent := filepath.Dir(path)
				if strings.EqualFold(filepath.Base(parent), "user") || strings.EqualFold(filepath.Base(parent), filepath.Base(gameFolder)) {
					scanPS4SaveDir(path, gameFolder, games, seen)
					return filepath.SkipDir
				}
			}

			// Unreal Engine SaveGames
			if info.IsDir() && strings.EqualFold(info.Name(), "SaveGames") {
				parentDir := filepath.Dir(path)
				if strings.EqualFold(filepath.Base(parentDir), "Saved") {
					canonicalPath := strings.ToLower(filepath.Clean(path))
					if seen[canonicalPath] || seen[path] {
						return filepath.SkipDir
					}
					if isDummySaveDir(path) {
						return filepath.SkipDir
					}
					count, size := getDirStats(path)
					if count > 0 && size < 500*1024*1024 {
						seen[canonicalPath] = true
						seen[path] = true
						relPath, _ := filepath.Rel(gameFolder, path)
						appName := cleanGameTitle(entry.Name())
						gameParent := filepath.Base(filepath.Dir(parentDir))
						if gameParent != "" && gameParent != "." && gameParent != "/" && !ignoredGenericGameFolders[strings.ToLower(gameParent)] {
							if strings.EqualFold(appName, "docs") || strings.EqualFold(appName, "documents") || strings.EqualFold(appName, "games") || strings.EqualFold(appName, "my games") {
								appName = cleanGameTitle(gameParent)
							}
						}

						// Handle project name aliases (e.g. FSD -> Deep Rock Galactic, Pagoda -> Dead as Disco)
						if mappedID := findAppIDByMachineCorrelation(appName, *games); mappedID != "" {
							appName = getGameName(mappedID, appName)
						} else if mappedID := findAppIDByMachineCorrelation(gameParent, *games); mappedID != "" {
							appName = getGameName(mappedID, appName)
						} else if strings.EqualFold(appName, "fsd") || strings.Contains(strings.ToLower(gameFolder), "deep rock") {
							appName = "Deep Rock Galactic"
						}

						appID := extractAppIDFromFolder(gameFolder)
						iconURL := ""
						protonTier := ""

						if appID != "" {
							appName = getGameName(appID, appName)
							iconURL = getSteamIconURL(appID)
							protonTier = fetchProtonDBTier(appID)
						} else {
							resID, resIcon, resTier := resolveAppIDByName(appName)
							if resID != "" {
								appID = resID
								iconURL = resIcon
								protonTier = resTier
							}
						}

						if isOnlineLiveServiceGame(appName, appID, path) {
							return filepath.SkipDir
						}

						*games = append(*games, DetectedGame{
							ID:                 "intree_" + sanitizeID(entry.Name()),
							Name:               appName,
							AppID:              appID,
							SaveType:           SaveTypeInTree,
							StorageType:          "In-Tree (Unreal Engine / Portable)",
							SourcePath:         path,
							TargetRelativePath: filepath.ToSlash(relPath),
							FileCount:          count,
							SizeBytes:          size,
							IconURL:            iconURL,
							ProtonTier:         protonTier,
						})
					}
					return filepath.SkipDir
				}
			}

			// Goldberg portable steam_settings/saves
			if info.IsDir() && strings.EqualFold(info.Name(), "steam_settings") {
				saveSub := filepath.Join(path, "saves")
				if stat, err := os.Stat(saveSub); err == nil && stat.IsDir() {
					if !seen[saveSub] {
						count, size := getDirStats(saveSub)
						if count > 0 && size < 500*1024*1024 {
							seen[saveSub] = true
							relPath, _ := filepath.Rel(gameFolder, saveSub)
							appName := entry.Name()
							if strings.EqualFold(appName, "docs") || strings.EqualFold(appName, "games") {
								appName = filepath.Base(filepath.Dir(path))
							}
							appID := extractAppIDFromFolder(gameFolder)
							iconURL := ""
							protonTier := ""

							if appID != "" {
								appName = getGameName(appID, appName)
								iconURL = getSteamIconURL(appID)
								protonTier = fetchProtonDBTier(appID)
							} else {
								resID, resIcon, resTier := resolveAppIDByName(appName)
								if resID != "" {
									appID = resID
									iconURL = resIcon
									protonTier = resTier
								}
							}

							*games = append(*games, DetectedGame{
								ID:                 "portable_" + sanitizeID(entry.Name()),
								Name:               appName,
								AppID:              appID,
								SaveType:           SaveTypeInTree,
								StorageType:          "Portable Goldberg",
								SourcePath:         saveSub,
								TargetRelativePath: filepath.ToSlash(relPath),
								FileCount:          count,
								SizeBytes:          size,
								IconURL:            iconURL,
								ProtonTier:         protonTier,
							})
						}
					}
				}
				return filepath.SkipDir
			}
			return nil
		})
	}
}

func scanCustomPath(customPath string, games *[]DetectedGame, seen map[string]bool) {
	if customPath == "" {
		return
	}
	customPath = filepath.Clean(customPath)
	if _, err := os.Stat(customPath); os.IsNotExist(err) {
		return
	}

	// 1. Direct check: PS4 Emulator savedata
	if stat, err := os.Stat(filepath.Join(customPath, "savedata")); err == nil && stat.IsDir() {
		scanPS4SaveDir(filepath.Join(customPath, "savedata"), customPath, games, seen)
		return
	}
	if stat, err := os.Stat(filepath.Join(customPath, "user", "savedata")); err == nil && stat.IsDir() {
		scanPS4SaveDir(filepath.Join(customPath, "user", "savedata"), customPath, games, seen)
		return
	}
	if isCUSAID(filepath.Base(customPath)) {
		addPS4Game(customPath, filepath.Dir(customPath), filepath.Base(customPath), games, seen)
		return
	}

	// 2. Direct check: Standalone game save folder (e.g. Darkest Dungeon)
	baseName := filepath.Base(customPath)
	baseLow := strings.ToLower(baseName)
	if !ignoredGenericGameFolders[baseLow] && !ignoredGameResourceFolders[baseLow] && !seen[customPath] && hasGameSaveIndicators(customPath) {
		if !isGameAssetDirectory(customPath) && !hasProgramBinaries(customPath) && !isRepackOrInstallerFolder(customPath, baseName) {
			count, size := getDirStats(customPath)
			if count > 0 && size < 500*1024*1024 {
				seen[customPath] = true
				appID, iconURL, protonTier := resolveAppIDByName(baseName)
				cleanName := baseName
				if appID != "" {
					cleanName = getGameName(appID, cleanName)
				}
				*games = append(*games, DetectedGame{
					ID:                 "custom_" + sanitizeID(baseName),
					Name:               cleanName,
					AppID:              appID,
					SaveType:           SaveTypePrefix,
					StorageType:          "Custom / In-Tree Save",
					SourcePath:         customPath,
					TargetRelativePath: "users/steamuser/Documents/" + baseName,
					FileCount:          count,
					SizeBytes:          size,
					IconURL:            iconURL,
					ProtonTier:         protonTier,
				})
				return
			}
		}
	}

	// 3. Container folder scan
	scanInTreeDirectory(customPath, games, seen)
}

var knownOnlineGames = []struct {
	Name       string
	Status     string
	AntiCheats []string
	Signatures []string
}{
	{
		Name:       "Grand Theft Auto V (GTA Online)",
		Status:     "Broken",
		AntiCheats: []string{"BattlEye (Story Mode Works)"},
		Signatures: []string{`GTAV`, `Grand Theft Auto V`, `GTA5`, `Rockstar Games\GTAV`},
	},
	{
		Name:       "Tom Clancy’s The Division 2",
		Status:     "Broken",
		AntiCheats: []string{"Easy Anti-Cheat"},
		Signatures: []string{`The Division 2`, `Tom Clancy's The Division 2`, `TheDivision2`},
	},
	{
		Name:       "Valorant",
		Status:     "Denied",
		AntiCheats: []string{"Vanguard"},
		Signatures: []string{`VALORANT`, `Riot Games\VALORANT`, `Riot Vanguard`},
	},
	{
		Name:       "League of Legends",
		Status:     "Denied",
		AntiCheats: []string{"Vanguard"},
		Signatures: []string{`League of Legends`, `Riot Games\League of Legends`},
	},
	{
		Name:       "Fortnite",
		Status:     "Broken",
		AntiCheats: []string{"Easy Anti-Cheat", "BattlEye"},
		Signatures: []string{`Fortnite`, `Epic Games\Fortnite`, `FortniteGame`},
	},
	{
		Name:       "Tom Clancy's Rainbow Six Siege",
		Status:     "Broken",
		AntiCheats: []string{"BattlEye"},
		Signatures: []string{`appmanifest_359550.acf`, `Rainbow Six Siege`, `Rainbow Six - Siege`},
	},
	{
		Name:       "Destiny 2",
		Status:     "Broken",
		AntiCheats: []string{"BattlEye"},
		Signatures: []string{`appmanifest_1085660.acf`, `Destiny 2`},
	},
	{
		Name:       "Escape from Tarkov",
		Status:     "Broken",
		AntiCheats: []string{"BattlEye"},
		Signatures: []string{`Escape from Tarkov`, `BsgLauncher`, `Battlestate Games`},
	},
	{
		Name:       "EA SPORTS FC / FIFA",
		Status:     "Broken",
		AntiCheats: []string{"EA Anti-Cheat"},
		Signatures: []string{`EA SPORTS FC`, `FIFA 23`, `FIFA 24`},
	},
	{
		Name:       "Battlefield 2042",
		Status:     "Broken",
		AntiCheats: []string{"EA Anti-Cheat"},
		Signatures: []string{`Battlefield 2042`},
	},
	{
		Name:       "Call of Duty (Warzone)",
		Status:     "Broken",
		AntiCheats: []string{"Ricochet"},
		Signatures: []string{`Call of Duty`, `Warzone`},
	},
	{
		Name:       "PUBG: Battlegrounds",
		Status:     "Broken",
		AntiCheats: []string{"BattlEye"},
		Signatures: []string{`appmanifest_578080.acf`, `PUBG`},
	},
	{
		Name:       "Lost Ark",
		Status:     "Broken",
		AntiCheats: []string{"Easy Anti-Cheat"},
		Signatures: []string{`appmanifest_1599340.acf`, `Lost Ark`},
	},
	{
		Name:       "Roblox",
		Status:     "Running",
		AntiCheats: []string{"Hyperion (Playable via Sober / Flatpak)"},
		Signatures: []string{`Roblox Player`, `Roblox`},
	},
	{
		Name:       "Apex Legends",
		Status:     "Supported",
		AntiCheats: []string{"Easy Anti-Cheat"},
		Signatures: []string{`appmanifest_1172470.acf`, `Apex Legends`},
	},
	{
		Name:       "Counter-Strike 2",
		Status:     "Supported",
		AntiCheats: []string{"VAC (Native)"},
		Signatures: []string{`appmanifest_730.acf`, `Counter-Strike 2`, `Counter-Strike Global Offensive`},
	},
	{
		Name:       "Overwatch 2",
		Status:     "Supported",
		AntiCheats: []string{"Battle.net"},
		Signatures: []string{`Overwatch`, `appmanifest_2357570.acf`},
	},
	{
		Name:       "The Finals",
		Status:     "Supported",
		AntiCheats: []string{"Easy Anti-Cheat"},
		Signatures: []string{`appmanifest_2073850.acf`, `The Finals`, `Discovery`},
	},
	{
		Name:       "Deadlock",
		Status:     "Supported",
		AntiCheats: []string{"VAC (Valve Anti-Cheat)"},
		Signatures: []string{`appmanifest_1422450.acf`, `Deadlock`},
	},
	{
		Name:       "Marvel Rivals",
		Status:     "Running",
		AntiCheats: []string{"Anti-Cheat"},
		Signatures: []string{`appmanifest_2767030.acf`, `Marvel Rivals`, `MarvelRivals`},
	},
	{
		Name:       "Squad 44",
		Status:     "Supported",
		AntiCheats: []string{"Easy Anti-Cheat"},
		Signatures: []string{`appmanifest_736220.acf`, `Squad 44`, `Post Scriptum`, `Squad44`},
	},
	{
		Name:       "Squad",
		Status:     "Supported",
		AntiCheats: []string{"Easy Anti-Cheat"},
		Signatures: []string{`appmanifest_393380.acf`, `appmanifest_2011840.acf`, `Squad`, `SquadGame`},
	},
	{
		Name:       "ARC Raiders",
		Status:     "Running",
		AntiCheats: []string{"Easy Anti-Cheat"},
		Signatures: []string{`appmanifest_1808500.acf`, `ARC Raiders`, `PioneerGame`},
	},
	{
		Name:       "Dead by Daylight",
		Status:     "Supported",
		AntiCheats: []string{"Easy Anti-Cheat"},
		Signatures: []string{`appmanifest_381210.acf`, `Dead by Daylight`},
	},
	{
		Name:       "Rust",
		Status:     "Broken",
		AntiCheats: []string{"Easy Anti-Cheat"},
		Signatures: []string{`appmanifest_252490.acf`, `Rust`},
	},
	{
		Name:       "Warframe",
		Status:     "Supported",
		AntiCheats: []string{"None (Proton Friendly)"},
		Signatures: []string{`appmanifest_230410.acf`, `Warframe`},
	},
	{
		Name:       "Helldivers 2",
		Status:     "Running",
		AntiCheats: []string{"nProtect GameGuard"},
		Signatures: []string{`appmanifest_553850.acf`, `Helldivers 2`},
	},
	{
		Name:       "Genshin Impact",
		Status:     "Running",
		AntiCheats: []string{"mhyprot2"},
		Signatures: []string{`Genshin Impact`},
	},
	{
		Name:       "Honkai: Star Rail",
		Status:     "Running",
		AntiCheats: []string{"HoYoverse AC"},
		Signatures: []string{`Honkai: Star Rail`, `Star Rail`},
	},
	{
		Name:       "Brawlhalla",
		Status:     "Supported",
		AntiCheats: []string{"Easy Anti-Cheat"},
		Signatures: []string{`appmanifest_291550.acf`, `Brawlhalla`},
	},
	{
		Name:       "Sky: Children of the Light",
		Status:     "Running",
		AntiCheats: []string{"Cloud Server MMO"},
		Signatures: []string{`appmanifest_2325290.acf`, `Sky Children of the Light`},
	},
	{
		Name:       "Combat Master",
		Status:     "Running",
		AntiCheats: []string{"Cloud Server Online FPS"},
		Signatures: []string{`appmanifest_2281730.acf`, `Combat Master`},
	},
}

func ScanOnlineGames() []OnlineGame {
	var results []OnlineGame
	found := make(map[string]bool)

	// 1. Scan Windows Uninstall Registry (Universal for all launchers & standalone games)
	scanWindowsRegistryOnlineGames(&results, found)

	// 2. Scan standard directory roots
	searchRoots := []string{}
	if runtime.GOOS == "windows" {
		searchRoots = append(searchRoots,
			`C:\Riot Games`,
			`C:\Program Files\Epic Games`,
			`C:\Program Files (x86)\Epic Games`,
			`C:\Program Files (x86)\Battle.net`,
			`C:\Program Files\EA Games`,
			`C:\Program Files\Ubisoft\Ubisoft Game Launcher\games`,
			`C:\Program Files (x86)\Steam\steamapps`,
			`C:\Steam\steamapps`,
			`D:\SteamLibrary\steamapps`,
			`E:\SteamLibrary\steamapps`,
		)
		if localApp := os.Getenv("LOCALAPPDATA"); localApp != "" {
			searchRoots = append(searchRoots, localApp)
		}
		if appData := os.Getenv("APPDATA"); appData != "" {
			searchRoots = append(searchRoots, appData)
		}
		userProf := os.Getenv("USERPROFILE")
		if userProf != "" {
			docs := filepath.Join(userProf, "Documents")
			searchRoots = append(searchRoots, docs, filepath.Join(docs, "My Games"))
		}
	} else {
		home := os.Getenv("HOME")
		if home != "" {
			searchRoots = append(searchRoots,
				filepath.Join(home, "mock_online_games"),
				filepath.Join(home, "Games"),
				filepath.Join(home, "Games", "Heroic"),
				filepath.Join(home, ".local", "share", "lutris", "runners"),
				filepath.Join(home, ".var", "app", "com.heroicgameslauncher.hgl"),
				filepath.Join(home, ".var", "app", "org.vinegarhq.Sober"),
				filepath.Join(home, ".var", "app", "org.vinegarhq.Vinegar"),
				filepath.Join(home, ".config", "hydralauncher"),
			)
		}
		// Discover Steam libraries across all storage drives
		for _, lib := range findSteamLibraryFolders() {
			searchRoots = append(searchRoots,
				filepath.Join(lib, "steamapps"),
				filepath.Join(lib, "steamapps", "common"),
				filepath.Join(lib, "steamapps", "compatdata"),
			)
		}
		if userProf := os.Getenv("USERPROFILE"); userProf != "" {
			docs := filepath.Join(userProf, "Documents")
			searchRoots = append(searchRoots, docs, filepath.Join(docs, "My Games"))
		}
	}

	for _, g := range knownOnlineGames {
		if found[g.Name] {
			continue
		}
		for _, root := range searchRoots {
			for _, sig := range g.Signatures {
				target := filepath.Join(root, sig)
				if _, err := os.Stat(target); err == nil {
					found[g.Name] = true
					results = append(results, OnlineGame{
						Name:          g.Name,
						Status:        g.Status,
						AntiCheats:    g.AntiCheats,
						InstallSource: target,
					})
					break
				}
				// Also try case-insensitive resolution on Linux filesystems
				if runtime.GOOS != "windows" {
					if resolved := resolveCaseInsensitivePath(root, sig); resolved != "" && resolved != root {
						if _, err := os.Stat(resolved); err == nil {
							found[g.Name] = true
							results = append(results, OnlineGame{
								Name:          g.Name,
								Status:        g.Status,
								AntiCheats:    g.AntiCheats,
								InstallSource: resolved,
							})
							break
						}
					}
				}
			}
			if found[g.Name] {
				break
			}
		}
	}

	return results
}

func matchesSignatureWord(text, sig string) bool {
	if strings.HasSuffix(sig, ".acf") {
		return false
	}
	textLow := strings.ToLower(text)
	sigLow := strings.ToLower(sig)

	if len(sigLow) <= 4 {
		words := strings.FieldsFunc(textLow, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsNumber(r)
		})
		for _, w := range words {
			if w == sigLow {
				return true
			}
		}
		return false
	}
	return strings.Contains(textLow, sigLow)
}

func scanWindowsRegistryOnlineGames(results *[]OnlineGame, found map[string]bool) {
	if runtime.GOOS != "windows" {
		return
	}

	psCmd := `Get-ItemProperty HKLM:\Software\Microsoft\Windows\CurrentVersion\Uninstall\*, HKLM:\Software\Wow6432Node\Microsoft\Windows\CurrentVersion\Uninstall\*, HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\* | Where-Object { $_.DisplayName } | ForEach-Object { "$($_.DisplayName)|$($_.InstallLocation)" }`
	cmd := exec.Command("powershell", "-NoProfile", "-Command", psCmd)
	out, err := cmd.Output()
	if err != nil {
		return
	}

	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.Split(line, "|")
		displayName := parts[0]
		installLoc := ""
		if len(parts) > 1 {
			installLoc = parts[1]
		}

		for _, g := range knownOnlineGames {
			if found[g.Name] {
				continue
			}
			for _, sig := range g.Signatures {
				if matchesSignatureWord(displayName, sig) {
					found[g.Name] = true
					src := installLoc
					if src == "" {
						src = displayName + " (Windows Registry)"
					}
					*results = append(*results, OnlineGame{
						Name:          g.Name,
						Status:        g.Status,
						AntiCheats:    g.AntiCheats,
						InstallSource: src,
					})
					break
				}
			}
		}
	}
}

func extractAppIDFromFolder(gameDir string) string {
	// Look for steam_emu.ini, steam_api.ini, or Rune.ini
	var foundAppID string
	filepath.Walk(gameDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || foundAppID != "" {
			return nil
		}
		if !info.IsDir() {
			low := strings.ToLower(info.Name())
			if low == "steam_emu.ini" || low == "steam_api.ini" || low == "rune.ini" {
				if f, err := os.Open(path); err == nil {
					defer f.Close()
					scanner := bufio.NewScanner(f)
					for scanner.Scan() {
						line := strings.TrimSpace(scanner.Text())
						if strings.HasPrefix(strings.ToLower(line), "appid") {
							parts := strings.Split(line, "=")
							if len(parts) >= 2 {
								foundAppID = strings.TrimSpace(parts[1])
								return filepath.SkipDir
							}
						}
					}
				}
			}
		}
		return nil
	})
	return foundAppID
}

func getDirStats(path string) (int, int64) {
	count := 0
	var size int64 = 0
	filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			low := strings.ToLower(info.Name())
			if low == "cache" || low == "webcache" || low == "logs" || low == "crashdumps" || low == "dumps" || low == "temp" {
				return filepath.SkipDir
			}
		} else {
			count++
			size += info.Size()
		}
		return nil
	})
	return count, size
}

func sanitizeID(s string) string {
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, ":", "")
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "\\", "_")
	return strings.ToLower(s)
}
