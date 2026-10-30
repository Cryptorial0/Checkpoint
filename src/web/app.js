let detectedGames = [];
let vaultGames = [];
let currentPlatform = "windows";
let originalTitle = document.title || "Checkpoint";
let titleAlertTimer = null;

function notifyBackground(msg) {
  if (document.hidden) {
    let toggle = false;
    if (titleAlertTimer) clearInterval(titleAlertTimer);
    titleAlertTimer = setInterval(() => {
      document.title = toggle ? `(${msg}) Checkpoint` : originalTitle;
      toggle = !toggle;
    }, 1000);
  }
}

window.addEventListener("focus", () => {
  if (titleAlertTimer) {
    clearInterval(titleAlertTimer);
    titleAlertTimer = null;
  }
  document.title = originalTitle;
});

// Windows 7 Audio Synthesis
function playWin7Notify() {
  try {
    const AudioContext = window.AudioContext || window.webkitAudioContext;
    if (!AudioContext) return;
    const ctx = new AudioContext();
    if (ctx.state === "suspended") ctx.resume();
    const now = ctx.currentTime;

    // Windows 7 Notify: Two warm bell harmonics
    // Tone 1: E5 (659.25 Hz)
    const osc1 = ctx.createOscillator();
    const gain1 = ctx.createGain();
    osc1.type = "sine";
    osc1.frequency.setValueAtTime(659.25, now);
    gain1.gain.setValueAtTime(0.001, now);
    gain1.gain.exponentialRampToValueAtTime(0.24, now + 0.015);
    gain1.gain.exponentialRampToValueAtTime(0.001, now + 0.22);
    osc1.connect(gain1);
    gain1.connect(ctx.destination);
    osc1.start(now);
    osc1.stop(now + 0.25);

    // Tone 2: A5 (880.00 Hz) with soft secondary harmonic
    const osc2 = ctx.createOscillator();
    const gain2 = ctx.createGain();
    osc2.type = "sine";
    osc2.frequency.setValueAtTime(880.00, now + 0.12);
    gain2.gain.setValueAtTime(0.001, now + 0.12);
    gain2.gain.exponentialRampToValueAtTime(0.28, now + 0.135);
    gain2.gain.exponentialRampToValueAtTime(0.001, now + 0.45);
    osc2.connect(gain2);
    gain2.connect(ctx.destination);
    osc2.start(now + 0.12);
    osc2.stop(now + 0.48);
  } catch (e) {
    // Audio context unavailable
  }
}

function playWin7Tada() {
  try {
    const AudioContext = window.AudioContext || window.webkitAudioContext;
    if (!AudioContext) return;
    const ctx = new AudioContext();
    if (ctx.state === "suspended") ctx.resume();
    const now = ctx.currentTime;

    // Windows 7 Tada Fanfare: C4 -> E4 -> G4 -> C5
    const notes = [
      { freq: 261.63, start: 0.00, dur: 0.13, gain: 0.20 }, // C4
      { freq: 329.63, start: 0.11, dur: 0.13, gain: 0.22 }, // E4
      { freq: 392.00, start: 0.22, dur: 0.16, gain: 0.25 }, // G4
      { freq: 523.25, start: 0.36, dur: 0.55, gain: 0.30 }, // C5 (held)
    ];

    notes.forEach(n => {
      const osc = ctx.createOscillator();
      const gain = ctx.createGain();
      osc.type = "triangle"; // warm brass/synth timbre
      osc.frequency.setValueAtTime(n.freq, now + n.start);

      gain.gain.setValueAtTime(0.001, now + n.start);
      gain.gain.exponentialRampToValueAtTime(n.gain, now + n.start + 0.02);
      gain.gain.exponentialRampToValueAtTime(0.001, now + n.start + n.dur);

      osc.connect(gain);
      gain.connect(ctx.destination);
      osc.start(now + n.start);
      osc.stop(now + n.start + n.dur + 0.05);
    });
  } catch (e) {
    // Audio context unavailable
  }
}

function log(msg) {
  const box = document.getElementById("log-box");
  const time = new Date().toLocaleTimeString();
  box.textContent += `[${time}] ${msg}\n`;
  box.scrollTop = box.scrollHeight;
}

// Tab Switching
function setAppMode(mode) {
  const restoreBtn = document.getElementById("btn-mode-restore");
  const backupBtn = document.getElementById("btn-mode-backup");
  const restoreSec = document.getElementById("restore-content");
  const backupSec = document.getElementById("backup-content");
  const bannerRestore = document.getElementById("banner-restore");
  const bannerBackup = document.getElementById("banner-backup");

  if (mode === "restore") {
    restoreBtn?.classList.add("active");
    backupBtn?.classList.remove("active");
    restoreSec?.classList.remove("hidden");
    backupSec?.classList.add("hidden");
    bannerRestore?.classList.remove("hidden");
    bannerBackup?.classList.add("hidden");
  } else {
    backupBtn?.classList.add("active");
    restoreBtn?.classList.remove("active");
    backupSec?.classList.remove("hidden");
    restoreSec?.classList.add("hidden");
    bannerBackup?.classList.remove("hidden");
    bannerRestore?.classList.add("hidden");
  }
}

// Load System Status
async function loadSystemStatus() {
  try {
    const res = await fetch("/api/system");
    const data = await res.json();
    currentPlatform = (data.platform || "windows").toLowerCase();

    const badge = document.getElementById("os-badge");
    if (badge) {
      if (currentPlatform === "linux") {
        badge.textContent = "Linux Restorer";
        badge.style.backgroundColor = "#2ea043";
        setAppMode("restore");
        document.getElementById("target-launcher-container")?.classList.remove("hidden");
      } else {
        badge.textContent = "Windows Edition";
        badge.style.backgroundColor = "#1f6feb";
        setAppMode("backup");
        document.getElementById("target-launcher-container")?.classList.add("hidden");
        const bannerRestoreText = document.getElementById("banner-restore-text");
        if (bannerRestoreText) {
          bannerRestoreText.innerHTML = "Select your <code>game_saves_bundle.zip</code> to restore. Automatically restores saves directly to native <strong>%APPDATA%</strong>, <strong>%LOCALAPPDATA%</strong>, <strong>Documents</strong>, and <strong>Saved Games</strong> with Steam Cloud timestamp protection.";
        }
      }
    }

    const card = document.getElementById("syscheck-card");
    const content = document.getElementById("syscheck-content");
    content.innerHTML = "";

    let hasWarnings = false;

    if (data.fast_startup_active) {
      hasWarnings = true;
      const tweakDiv = document.createElement("div");
      tweakDiv.className = "alert alert-warning";
      tweakDiv.style.flexDirection = "column";
      tweakDiv.style.alignItems = "flex-start";
      tweakDiv.style.gap = "8px";

      if (data.fast_startup_active) {
        tweakDiv.innerHTML = `
          <div style="font-weight: 600;">Fast Startup Tweak (Windows)</div>
          <div style="font-size: 12px; color: #c9d1d9;">Windows Fast Startup is currently enabled. This hibernates the system and locks storage partitions during shutdown.</div>
          <div style="display: flex; align-items: center; gap: 8px; margin-top: 4px;">
            <input type="checkbox" id="chk-stage-fastboot" checked>
            <label for="chk-stage-fastboot" style="font-size: 13px;">Stage tweak: Disable Fast Startup</label>
          </div>
          <div style="display: flex; gap: 8px; margin-top: 6px;">
            <button id="btn-apply-fastboot" class="btn-warning">Apply Tweak</button>
            <button id="btn-undo-stage-fastboot" class="btn-subtle">Undo / Reset</button>
          </div>
        `;
        content.appendChild(tweakDiv);

        setTimeout(() => {
          const chk = document.getElementById("chk-stage-fastboot");
          const applyBtn = document.getElementById("btn-apply-fastboot");
          const undoBtn = document.getElementById("btn-undo-stage-fastboot");

          chk.addEventListener("change", () => {
            applyBtn.disabled = !chk.checked;
          });

          undoBtn.addEventListener("click", () => {
            chk.checked = false;
            applyBtn.disabled = true;
            log("Fast startup tweak un-staged (no changes made).");
          });

          applyBtn.addEventListener("click", disableFastStartup);
        }, 10);
      } else {
        // Already disabled
        tweakDiv.className = "alert alert-info";
        tweakDiv.innerHTML = `
          <div style="display: flex; justify-content: space-between; width: 100%; align-items: center;">
            <div>
              <span style="font-weight: 600;">Fast Startup is Disabled.</span>
              <span style="font-size: 12px; display: block; color: #8b949e;">Your system partitions will unmount cleanly and safely.</span>
            </div>
            <button id="btn-re-enable-fastboot" class="btn-subtle" style="font-size: 12px;">Re-enable / Undo</button>
          </div>
        `;
        content.appendChild(tweakDiv);

        setTimeout(() => {
          document.getElementById("btn-re-enable-fastboot").addEventListener("click", enableFastStartup);
        }, 10);
      }

      if (data.bitlocker_warning) {
        hasWarnings = true;
        const alert = document.createElement("div");
        alert.className = "alert alert-danger";
        alert.textContent = data.bitlocker_details;
        content.appendChild(alert);
      }
    }

    if (hasWarnings) {
      card.classList.remove("hidden");
    } else {
      card.classList.add("hidden");
    }
  } catch (err) {
    log("Failed to load system status: " + err);
  }
}

async function disableFastStartup() {
  log("Attempting to disable fast startup...");
  try {
    const res = await fetch("/api/disable-fast-startup", { method: "POST" });
    const text = await res.text();
    log(text);
    loadSystemStatus();
  } catch (err) {
    log("Error: " + err);
  }
}

async function enableFastStartup() {
  log("Attempting to re-enable fast startup...");
  try {
    const res = await fetch("/api/enable-fast-startup", { method: "POST" });
    const text = await res.text();
    log(text);
    loadSystemStatus();
  } catch (err) {
    log("Error: " + err);
  }
}

// Folder & File Browsers
async function pickFolder(title) {
  try {
    const res = await fetch(`/api/pick-folder?title=${encodeURIComponent(title)}`, { method: "POST" });
    const data = await res.json();
    return data.path || "";
  } catch (err) {
    log("Folder picker error: " + err);
    return "";
  }
}

async function pickFile(title) {
  try {
    const res = await fetch(`/api/pick-file?title=${encodeURIComponent(title)}`, { method: "POST" });
    const data = await res.json();
    return data.path || "";
  } catch (err) {
    log("File picker error: " + err);
    return "";
  }
}

async function loadAppConfig() {
  try {
    const res = await fetch("/api/config");
    const cfg = await res.json();
    if (cfg) {
      if (cfg.custom_paths && Array.isArray(cfg.custom_paths)) {
        cfg.custom_paths.forEach(p => {
          if (!customScanPaths.includes(p)) {
            customScanPaths.push(p);
          }
        });
        renderCustomPaths();
      }
      if (cfg.backup_dest && (!document.getElementById("backup-dest").value || document.getElementById("backup-dest").value.trim() === "")) {
        document.getElementById("backup-dest").value = cfg.backup_dest;
      }
    }
  } catch (err) {
    // Ignore config load error
  }
}

async function saveAppConfig() {
  const dest = document.getElementById("backup-dest").value.trim();
  try {
    await fetch("/api/config", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        custom_paths: customScanPaths,
        backup_dest: dest,
      }),
    });
  } catch (err) {
    // Ignore config save error
  }
}

let customScanPaths = [];

function renderCustomPaths() {
  const container = document.getElementById("custom-paths-container");
  const list = document.getElementById("custom-paths-list");
  list.innerHTML = "";

  if (customScanPaths.length === 0) {
    container.classList.add("hidden");
    return;
  }

  container.classList.remove("hidden");
  customScanPaths.forEach((p, idx) => {
    const chip = document.createElement("div");
    chip.className = "path-chip";
    chip.title = p;

    const span = document.createElement("span");
    span.textContent = p;

    const removeBtn = document.createElement("button");
    removeBtn.className = "btn-remove-chip";
    removeBtn.textContent = "✕";
    removeBtn.title = "Remove path";
    removeBtn.type = "button";
    removeBtn.addEventListener("click", () => {
      customScanPaths.splice(idx, 1);
      renderCustomPaths();
      saveAppConfig();
    });

    chip.appendChild(span);
    chip.appendChild(removeBtn);
    list.appendChild(chip);
  });
}

function addCustomPath(p) {
  if (!p) return;
  p = p.trim();
  if (p === "") return;
  // Semicolon/comma support
  const parts = p.split(/[;,]+/).map(s => s.trim()).filter(Boolean);
  for (const part of parts) {
    if (!customScanPaths.includes(part)) {
      customScanPaths.push(part);
    }
  }
  renderCustomPaths();
  saveAppConfig();
}

document.getElementById("btn-add-path").addEventListener("click", () => {
  const input = document.getElementById("custom-scan-path");
  if (input.value.trim() !== "") {
    addCustomPath(input.value);
    input.value = "";
  }
});

document.getElementById("custom-scan-path").addEventListener("keydown", (e) => {
  if (e.key === "Enter") {
    e.preventDefault();
    const input = document.getElementById("custom-scan-path");
    if (input.value.trim() !== "") {
      addCustomPath(input.value);
      input.value = "";
    }
  }
});

document.getElementById("btn-browse-scan").addEventListener("click", async () => {
  const p = await pickFolder("Select Additional Game Folder");
  if (p) {
    addCustomPath(p);
  }
});

document.getElementById("btn-browse-dest").addEventListener("click", async () => {
  const p = await pickFolder("Select Backup Destination Folder (e.g. USB Drive)");
  if (p) {
    document.getElementById("backup-dest").value = p;
    saveAppConfig();
    updateDiskSpaceBadge();
  }
});

document.getElementById("backup-dest").addEventListener("input", () => {
  updateDiskSpaceBadge();
});

document.getElementById("backup-dest").addEventListener("change", () => {
  saveAppConfig();
  updateDiskSpaceBadge();
});

// Progress Polling Controller (Scan)
let progressTimer = null;

function startProgressPolling(initialStatus) {
  const container = document.getElementById("progress-container");
  const fill = document.getElementById("progress-bar-fill");
  const percentText = document.getElementById("progress-percent");
  const statusText = document.getElementById("progress-status");
  const detailText = document.getElementById("progress-detail");
  const scanSpinner = document.getElementById("scan-spinner");
  const scanProgressSpinner = document.getElementById("scan-progress-spinner");
  const scanBtnText = document.getElementById("scan-btn-text");
  const scanBtn = document.getElementById("btn-scan");

  if (scanSpinner) scanSpinner.classList.remove("hidden");
  if (scanProgressSpinner) scanProgressSpinner.classList.remove("hidden");
  if (scanBtnText) scanBtnText.textContent = "Scanning...";
  if (scanBtn) scanBtn.disabled = true;

  if (container) container.classList.remove("hidden");
  if (fill) fill.style.width = "0%";
  if (percentText) percentText.innerText = "0%";
  if (statusText) statusText.innerText = initialStatus || "Scanning games...";
  if (detailText) detailText.innerText = "";

  if (progressTimer) clearInterval(progressTimer);

  progressTimer = setInterval(async () => {
    try {
      const res = await fetch("/api/progress");
      if (!res.ok) return;
      const data = await res.json();

      const pct = Math.min(100, Math.max(0, Math.round(data.percent || 0)));
      if (fill) fill.style.width = pct + "%";
      if (percentText) percentText.innerText = pct + "%";
      if (statusText && data.status) statusText.innerText = data.status;
      if (detailText) detailText.innerText = data.detail || "";

      if (!data.active && pct >= 100) {
        clearInterval(progressTimer);
        progressTimer = null;
      }
    } catch (e) {
      // Ignore network hiccups during poll
    }
  }, 120);
}

function stopProgressPolling(finalStatus) {
  if (progressTimer) {
    clearInterval(progressTimer);
    progressTimer = null;
  }
  const fill = document.getElementById("progress-bar-fill");
  const percentText = document.getElementById("progress-percent");
  const statusText = document.getElementById("progress-status");
  const detailText = document.getElementById("progress-detail");
  const scanSpinner = document.getElementById("scan-spinner");
  const scanProgressSpinner = document.getElementById("scan-progress-spinner");
  const scanBtnText = document.getElementById("scan-btn-text");
  const scanBtn = document.getElementById("btn-scan");

  if (scanSpinner) scanSpinner.classList.add("hidden");
  if (scanProgressSpinner) scanProgressSpinner.classList.add("hidden");
  if (scanBtnText) scanBtnText.textContent = "Scan Games";
  if (scanBtn) scanBtn.disabled = false;

  if (fill) fill.style.width = "100%";
  if (percentText) percentText.innerText = "100%";
  if (statusText && finalStatus) statusText.innerText = finalStatus;
  if (detailText) detailText.innerText = "";
}

// Dedicated Backup Progress Controller
let backupProgressTimer = null;

function startBackupProgress(initialStatus) {
  const container = document.getElementById("backup-progress-container");
  const fill = document.getElementById("backup-progress-bar-fill");
  const percentText = document.getElementById("backup-progress-percent");
  const statusText = document.getElementById("backup-progress-status");
  const detailText = document.getElementById("backup-progress-detail");
  const spinner = document.getElementById("backup-spinner");
  const backupBtn = document.getElementById("btn-backup");

  if (container) container.classList.remove("hidden");
  if (spinner) spinner.classList.remove("hidden");
  if (fill) fill.style.width = "0%";
  if (percentText) percentText.innerText = "0%";
  if (statusText) statusText.innerText = initialStatus || "Backing up...";
  if (detailText) detailText.innerText = "";
  if (backupBtn) {
    backupBtn.disabled = true;
    backupBtn.textContent = "Backing up...";
  }

  if (backupProgressTimer) clearInterval(backupProgressTimer);

  backupProgressTimer = setInterval(async () => {
    try {
      const res = await fetch("/api/progress");
      if (!res.ok) return;
      const data = await res.json();

      if (data.operation === "backup") {
        const pct = Math.min(100, Math.max(0, Math.round(data.percent || 0)));
        if (fill) fill.style.width = pct + "%";
        if (percentText) percentText.innerText = pct + "%";
        if (statusText && data.status) statusText.innerText = data.status;
        if (detailText) detailText.innerText = data.detail || "";

        if (!data.active && pct >= 100) {
          clearInterval(backupProgressTimer);
          backupProgressTimer = null;
        }
      }
    } catch (e) {
      // Ignore network hiccups during poll
    }
  }, 100);
}

function stopBackupProgress(finalStatus) {
  if (backupProgressTimer) {
    clearInterval(backupProgressTimer);
    backupProgressTimer = null;
  }
  const fill = document.getElementById("backup-progress-bar-fill");
  const percentText = document.getElementById("backup-progress-percent");
  const statusText = document.getElementById("backup-progress-status");
  const detailText = document.getElementById("backup-progress-detail");
  const spinner = document.getElementById("backup-spinner");
  const backupBtn = document.getElementById("btn-backup");

  if (spinner) spinner.classList.add("hidden");
  if (fill) fill.style.width = "100%";
  if (percentText) percentText.innerText = "100%";
  if (statusText && finalStatus) statusText.innerText = finalStatus;
  if (detailText) detailText.innerText = "";
  if (backupBtn) {
    backupBtn.disabled = false;
    backupBtn.textContent = "Start Backup";
  }
}

// Scan Games
document.getElementById("btn-scan").addEventListener("click", async () => {
  const inputVal = document.getElementById("custom-scan-path").value.trim();
  if (inputVal !== "") {
    addCustomPath(inputVal);
    document.getElementById("custom-scan-path").value = "";
  }

  log("Scanning for game saves across standard paths and custom folders...");
  startProgressPolling("Scanning installed games...");

  try {
    const res = await fetch("/api/scan", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ 
        custom_paths: customScanPaths,
        custom_path: ""
      }),
    });
    const resData = await res.json();
    detectedGames = resData.games || [];
    renderScanResults(detectedGames);
    renderOnlineGames(resData.online_games || []);
    log(`Scan finished. Found ${detectedGames.length} game saves.`);
    if (resData.report_path) {
      log(`Scan report generated: ${resData.report_path}`);
    }
    stopProgressPolling(`Scan complete. Found ${detectedGames.length} game saves.`);
    playWin7Notify();
    notifyBackground("Scan Complete");
    updateDiskSpaceBadge();
  } catch (err) {
    log("Scan failed: " + err);
    stopProgressPolling("Scan failed");
  }
});

function formatBytes(bytes) {
  if (!bytes || bytes <= 0) return "0 KB";
  if (bytes < 1024 * 1024) {
    const kb = (bytes / 1024).toFixed(2);
    return `${kb} KB`;
  }
  if (bytes >= 1024 * 1024 * 1024) {
    const gb = (bytes / (1024 * 1024 * 1024)).toFixed(2);
    return `${gb} GB`;
  }
  const mb = (bytes / (1024 * 1024)).toFixed(2);
  return `${mb} MB`;
}

function generateReportText(games) {
  const list = games || detectedGames || [];
  if (list.length === 0) {
    return "=== SAVE MIGRATOR SCAN REPORT ===\nNo titles detected.";
  }

  const lines = [];
  lines.push("=== SAVE MIGRATOR SCAN REPORT ===");
  lines.push(`Generated: ${new Date().toLocaleString()}`);
  lines.push(`Total Titles Found: ${list.length}`);
  lines.push("--------------------------------------------------");

  list.forEach((g, idx) => {
    const sizeStr = formatBytes(g.size_bytes);
    lines.push(`[${idx + 1}] Title: ${g.name}`);
    if (g.appid) {
      lines.push(`    AppID: ${g.appid}`);
    }
    if (g.proton_tier) {
      lines.push(`    ProtonDB: ${g.proton_tier.toUpperCase()}`);
    }
    lines.push(`    Type: ${g.storage_type || g.save_type}`);
    lines.push(`    Location: ${g.source_path}`);
    if (g.target_relative_path) {
      lines.push(`    Target Relative: ${g.target_relative_path}`);
    }
    lines.push(`    Files: ${g.file_count} (${sizeStr})`);
    if (g.notes) {
      lines.push(`    Notes: ${g.notes}`);
    }
    lines.push("");
  });

  return lines.join("\n");
}

async function copyReportToClipboard() {
  const text = generateReportText(detectedGames);
  try {
    if (navigator.clipboard && navigator.clipboard.writeText) {
      await navigator.clipboard.writeText(text);
    } else {
      throw new Error("Clipboard API unavailable");
    }
    log("Scan report copied to clipboard. You can paste it directly into the chat.");
    const btn = document.getElementById("btn-copy-report");
    const oldText = btn.textContent;
    btn.textContent = "Copied to Clipboard!";
    setTimeout(() => { btn.textContent = oldText; }, 2500);
  } catch (err) {
    const textarea = document.createElement("textarea");
    textarea.value = text;
    document.body.appendChild(textarea);
    textarea.select();
    document.execCommand("copy");
    document.body.removeChild(textarea);
    log("Scan report copied to clipboard.");
    const btn = document.getElementById("btn-copy-report");
    const oldText = btn.textContent;
    btn.textContent = "Copied to Clipboard!";
    setTimeout(() => { btn.textContent = oldText; }, 2500);
  }
}

async function exportReportTxt() {
  const text = generateReportText(detectedGames);

  // Copy to clipboard for instant pasting
  copyReportToClipboard();

  // Webview file download
  try {
    const blob = new Blob([text], { type: "text/plain;charset=utf-8" });
    const blobUrl = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = blobUrl;
    link.download = "save_migrator_scan_report.txt";
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    URL.revokeObjectURL(blobUrl);
  } catch (e) {
    // Ignore in embedded desktop mode
  }

  // Request backend to save to disk in app folder and temp folder
  try {
    const res = await fetch("/api/export-report", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ games: detectedGames }),
    });
    const data = await res.json();
    if (data.path) {
      log(`Scan report saved to disk: ${data.path}`);
    }
  } catch (err) {
    log("Export report error: " + err);
  }
}

document.getElementById("btn-copy-report").addEventListener("click", copyReportToClipboard);
document.getElementById("btn-export-report").addEventListener("click", exportReportTxt);

function renderScanResults(games) {
  const container = document.getElementById("scan-results");
  const backupBtn = document.getElementById("btn-backup");
  const toolbar = document.getElementById("scan-toolbar");

  if (!games || games.length === 0) {
    if (toolbar) toolbar.classList.add("hidden");
    container.innerHTML = '<div class="empty-state">No games found in standard emulator or scan paths.</div>';
    backupBtn.disabled = true;
    return;
  }

  if (toolbar) toolbar.classList.remove("hidden");

  const skipSteamCloud = document.getElementById("chk-skip-steam-cloud") ? document.getElementById("chk-skip-steam-cloud").checked : false;
  const skipUserSettings = document.getElementById("chk-skip-user-settings") ? document.getElementById("chk-skip-user-settings").checked : false;

  container.innerHTML = "";
  games.forEach((game, idx) => {
    const div = document.createElement("div");
    div.className = "game-item";
    const sizeStr = formatBytes(game.size_bytes);
    const appidHtml = game.appid ? `<span class="appid-badge">Steam AppID: ${game.appid}</span>` : "";
    const protonHtml = game.proton_tier ? `<span class="proton-badge proton-${game.proton_tier}">ProtonDB: ${game.proton_tier}</span>` : "";
    const iconHtml = game.icon_url ? `<img class="game-icon" src="${game.icon_url}" onerror="if(this.dataset.tried!=='1'&&'${game.appid}'){this.dataset.tried='1';this.src='https://shared.steamstatic.com/store_item_assets/steam/apps/${game.appid}/header.jpg';}else{this.style.display='none';}">` : "";

    const isCloud = !!game.is_steam_cloud;
    const isSettings = !!game.is_user_settings;
    let isChecked = "checked";
    if (isCloud && skipSteamCloud) isChecked = "";
    if (isSettings && skipUserSettings) isChecked = "";

    let extraBadge = "";
    if (isCloud) {
      extraBadge += `<span class="game-tag" style="background:#238636;color:#ffffff;border:1px solid #2ea043;font-size:11px;margin-left:4px;">Steam Cloud</span>`;
    }
    if (isSettings) {
      extraBadge += `<span class="game-tag" style="background:#1f6feb;color:#ffffff;border:1px solid #388bfd;font-size:11px;margin-left:4px;">User Settings</span>`;
    }

    div.innerHTML = `
      <input type="checkbox" id="game-chk-${idx}" ${isChecked} data-idx="${idx}" style="margin-top: 6px;">
      ${iconHtml}
      <div class="game-info">
        <div class="game-title-row">
          <input type="text" class="game-name-input" value="${game.name}" data-idx="${idx}" title="Click to edit game name if desired">
          ${appidHtml}
          ${protonHtml}
          ${extraBadge}
          <button class="btn-subtle btn-open-folder" data-path="${game.source_path}" style="font-size: 11px; padding: 2px 8px; margin-left: auto;" type="button" title="Open this save directory in Windows File Explorer">Open Folder</button>
        </div>
        <div class="game-meta">
          <span class="game-tag">${game.storage_type}</span>
          ${game.file_count} files (${sizeStr}) | Source: ${game.source_path}
        </div>
      </div>
    `;
    container.appendChild(div);
  });

  // Attach listener to open folder buttons
  container.querySelectorAll(".btn-open-folder").forEach((btn) => {
    btn.addEventListener("click", async (e) => {
      const p = e.target.dataset.path;
      if (p) {
        try {
          await fetch("/api/open-folder", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ path: p }),
          });
          log(`Opened in Explorer: ${p}`);
        } catch (err) {
          log(`Open folder error: ${err}`);
        }
      }
    });
  });

  // Attach listener to name inputs with automatic AppID & artwork repicking
  container.querySelectorAll(".game-name-input").forEach((inp) => {
    inp.addEventListener("input", (e) => {
      const idx = parseInt(e.target.dataset.idx, 10);
      if (detectedGames[idx]) {
        detectedGames[idx].name = e.target.value.trim();
      }
      clearTimeout(resolveDebounceTimers[idx]);
      resolveDebounceTimers[idx] = setTimeout(() => {
        repickGameMetadata(e.target);
      }, 600);
    });

    inp.addEventListener("change", (e) => {
      const idx = parseInt(e.target.dataset.idx, 10);
      clearTimeout(resolveDebounceTimers[idx]);
      repickGameMetadata(e.target);
    });

    inp.addEventListener("keydown", (e) => {
      if (e.key === "Enter") {
        e.preventDefault();
        const idx = parseInt(e.target.dataset.idx, 10);
        clearTimeout(resolveDebounceTimers[idx]);
        repickGameMetadata(e.target);
        e.target.blur();
      }
    });
  });

  // Checkbox change updates free space badge
  container.querySelectorAll("input[type='checkbox']").forEach((chk) => {
    chk.addEventListener("change", () => {
      updateDiskSpaceBadge();
    });
  });

  backupBtn.disabled = false;
  updateDiskSpaceBadge();
  filterDetectedGames();
}

function filterDetectedGames() {
  const input = document.getElementById("game-search-input");
  const query = (input ? input.value : "").toLowerCase().trim();
  const clearBtn = document.getElementById("btn-clear-search");
  const countEl = document.getElementById("search-count");
  const items = document.querySelectorAll("#scan-results .game-item");

  if (clearBtn) {
    if (query.length > 0) clearBtn.classList.remove("hidden");
    else clearBtn.classList.add("hidden");
  }

  let visibleCount = 0;
  items.forEach((item) => {
    const chk = item.querySelector("input[type='checkbox']");
    const idx = chk ? parseInt(chk.dataset.idx, 10) : -1;
    const g = detectedGames[idx];

    if (!query) {
      item.style.display = "";
      visibleCount++;
      return;
    }

    if (!g) {
      item.style.display = "";
      visibleCount++;
      return;
    }

    const name = (g.name || "").toLowerCase();
    const appid = (g.appid || "").toLowerCase();
    const storage = (g.storage_type || "").toLowerCase();
    const path = (g.source_path || "").toLowerCase();

    if (name.includes(query) || appid.includes(query) || storage.includes(query) || path.includes(query)) {
      item.style.display = "";
      visibleCount++;
    } else {
      item.style.display = "none";
    }
  });

  if (countEl) {
    if (query) {
      countEl.classList.remove("hidden");
      countEl.textContent = `Showing ${visibleCount} of ${detectedGames.length} games`;
    } else {
      countEl.classList.add("hidden");
    }
  }
}

const searchInput = document.getElementById("game-search-input");
if (searchInput) {
  searchInput.addEventListener("input", filterDetectedGames);
}

const clearSearchBtn = document.getElementById("btn-clear-search");
if (clearSearchBtn) {
  clearSearchBtn.addEventListener("click", () => {
    if (searchInput) {
      searchInput.value = "";
      filterDetectedGames();
      searchInput.focus();
    }
  });
}

// Select All / Deselect All
const selectAllBtn = document.getElementById("btn-select-all");
if (selectAllBtn) {
  selectAllBtn.addEventListener("click", () => {
    document.querySelectorAll("#scan-results input[type='checkbox']").forEach((chk) => {
      chk.checked = true;
    });
    updateDiskSpaceBadge();
  });
}

const deselectAllBtn = document.getElementById("btn-deselect-all");
if (deselectAllBtn) {
  deselectAllBtn.addEventListener("click", () => {
    document.querySelectorAll("#scan-results input[type='checkbox']").forEach((chk) => {
      chk.checked = false;
    });
    updateDiskSpaceBadge();
  });
}

async function updateDiskSpaceBadge() {
  const destInput = document.getElementById("backup-dest");
  const badge = document.getElementById("disk-space-badge");
  const backupBtn = document.getElementById("btn-backup");
  const spaceSummary = document.getElementById("space-summary-card");
  const spaceReqAmount = document.getElementById("space-req-amount");
  const spaceReqDetail = document.getElementById("space-req-detail");
  const spaceTargetSummary = document.getElementById("space-target-summary");

  let selectedBytes = 0;
  let selectedCount = 0;
  let selectedFiles = 0;
  document.querySelectorAll("#scan-results input[type='checkbox']:checked").forEach((chk) => {
    const idx = parseInt(chk.dataset.idx, 10);
    if (detectedGames[idx]) {
      selectedBytes += detectedGames[idx].size_bytes || 0;
      selectedFiles += detectedGames[idx].file_count || 0;
      selectedCount++;
    }
  });

  const selectedMB = (selectedBytes / (1024 * 1024)).toFixed(2);
  const selectedGB = (selectedBytes / (1024 * 1024 * 1024)).toFixed(2);
  const sizeDisplay = selectedBytes >= 1024 * 1024 * 1024 ? `${selectedGB} GB` : `${selectedMB} MB`;

  if (spaceSummary) {
    if (selectedCount > 0) {
      spaceSummary.classList.remove("hidden");
      if (spaceReqAmount) spaceReqAmount.textContent = sizeDisplay;
      if (spaceReqDetail) spaceReqDetail.textContent = `${selectedCount} titles selected (${selectedFiles} files)`;
    } else {
      spaceSummary.classList.add("hidden");
    }
  }

  if (!destInput || !badge) return;

  const destPath = destInput.value.trim();
  if (!destPath) {
    badge.classList.add("hidden");
    if (spaceTargetSummary && selectedCount > 0) {
      spaceTargetSummary.innerHTML = `<span style="color:#d29922;">Select destination folder or USB drive (Needs ${sizeDisplay})</span>`;
    }
    return;
  }

  try {
    const res = await fetch(`/api/disk-space?path=${encodeURIComponent(destPath)}`);
    const data = await res.json();
    if (data.error) {
      badge.style.color = "#d29922";
      badge.textContent = `Destination check: ${data.error}`;
      badge.classList.remove("hidden");
      if (spaceTargetSummary) spaceTargetSummary.textContent = data.error;
      return;
    }

    const freeBytes = data.free_bytes || 0;
    const freeGB = data.free_gb || "0";

    badge.classList.remove("hidden");
    if (freeBytes >= selectedBytes) {
      badge.style.color = "#3fb950";
      badge.innerHTML = `Target Free Space: <strong>${freeGB} GB</strong> available (Selected payload: <strong>${sizeDisplay}</strong> in ${selectedCount} games) - Ready`;
      if (spaceTargetSummary) {
        const remainingBytes = freeBytes - selectedBytes;
        const remGB = (remainingBytes / (1024 * 1024 * 1024)).toFixed(2);
        spaceTargetSummary.innerHTML = `<span style="color:#3fb950; font-weight:600;">Destination OK:</span> ${freeGB} GB available (${remGB} GB left after backup)`;
      }
      if (selectedCount > 0) {
        backupBtn.disabled = false;
      } else {
        backupBtn.disabled = true;
      }
    } else {
      badge.style.color = "#f85149";
      badge.innerHTML = `Warning: Low Disk Space! Only <strong>${freeGB} GB</strong> free on destination, but <strong>${sizeDisplay}</strong> is required.`;
      if (spaceTargetSummary) {
        spaceTargetSummary.innerHTML = `<span style="color:#f85149; font-weight:600;">Low Space:</span> Only ${freeGB} GB free on destination (Requires ${sizeDisplay})`;
      }
      backupBtn.disabled = true;
    }
  } catch (err) {
    // Silent on transient blip
  }
}

let resolveDebounceTimers = {};

async function repickGameMetadata(inputEl) {
  const idx = parseInt(inputEl.dataset.idx, 10);
  const newName = inputEl.value.trim();
  if (!detectedGames[idx]) return;

  detectedGames[idx].name = newName;
  if (!newName || newName.length < 2) return;

  inputEl.style.borderColor = "#58a6ff";

  try {
    const res = await fetch("/api/resolve-game", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name: newName }),
    });

    if (!res.ok) {
      inputEl.style.borderColor = "";
      return;
    }

    const data = await res.json();
    if (data && data.appid) {
      detectedGames[idx].appid = data.appid;
      if (data.icon_url) detectedGames[idx].icon_url = data.icon_url;
      if (data.proton_tier) detectedGames[idx].proton_tier = data.proton_tier;

      const card = inputEl.closest(".game-item");
      if (card) {
        // Update or insert icon image
        let img = card.querySelector(".game-icon");
        if (data.icon_url) {
          if (!img) {
            img = document.createElement("img");
            img.className = "game-icon";
            card.insertBefore(img, card.querySelector(".game-info"));
          }
          img.src = data.icon_url;
          img.style.display = "";
        }

        const titleRow = card.querySelector(".game-title-row");

        // Update or insert AppID badge
        let appidBadge = titleRow.querySelector(".appid-badge");
        if (!appidBadge) {
          appidBadge = document.createElement("span");
          appidBadge.className = "appid-badge";
          const openFolderBtn = titleRow.querySelector(".btn-open-folder");
          titleRow.insertBefore(appidBadge, openFolderBtn);
        }
        appidBadge.textContent = `Steam AppID: ${data.appid}`;

        // Update or insert ProtonDB badge
        let protonBadge = titleRow.querySelector(".proton-badge");
        if (data.proton_tier) {
          if (!protonBadge) {
            protonBadge = document.createElement("span");
            const openFolderBtn = titleRow.querySelector(".btn-open-folder");
            titleRow.insertBefore(protonBadge, openFolderBtn);
          }
          protonBadge.className = `proton-badge proton-${data.proton_tier}`;
          protonBadge.textContent = `ProtonDB: ${data.proton_tier}`;
        } else if (protonBadge) {
          protonBadge.remove();
        }

        inputEl.style.borderColor = "#3fb950";
        setTimeout(() => { inputEl.style.borderColor = ""; }, 1200);
        log(`Renamed to "${newName}" -> Repicked AppID ${data.appid} (${data.proton_tier || 'ProtonDB'})`);
      }
    } else {
      inputEl.style.borderColor = "";
    }
  } catch (err) {
    console.warn("Repick game metadata failed:", err);
    inputEl.style.borderColor = "";
  }
}

function renderOnlineGames(onlineGames, targetContainerId, targetSectionId) {
  const targets = [];
  if (targetContainerId && targetSectionId) {
    targets.push({
      section: document.getElementById(targetSectionId),
      container: document.getElementById(targetContainerId),
    });
  } else {
    const bSection = document.getElementById("online-games-section");
    const bContainer = document.getElementById("online-games-results");
    if (bSection && bContainer) targets.push({ section: bSection, container: bContainer });

    const rSection = document.getElementById("online-games-restore-section");
    const rContainer = document.getElementById("online-games-restore-results");
    if (rSection && rContainer) targets.push({ section: rSection, container: rContainer });
  }

  if (!onlineGames || onlineGames.length === 0) {
    targets.forEach(t => t.section?.classList.add("hidden"));
    return;
  }

  targets.forEach(t => {
    if (!t.section || !t.container) return;
    t.section.classList.remove("hidden");
    t.container.innerHTML = "";

    onlineGames.forEach((g) => {
      const div = document.createElement("div");
      div.className = "game-item";

      let statusClass = "ac-broken";
      if (g.status === "Supported") statusClass = "ac-supported";
      else if (g.status === "Running") statusClass = "ac-running";
      else if (g.status === "Denied") statusClass = "ac-denied";

      const anticheatsText = g.anticheats && g.anticheats.length > 0 ? ` (${g.anticheats.join(", ")})` : "";
      const sourceText = g.install_source ? `Detected at: ${escapeHtml(g.install_source)}` : "Online / Server Cloud Progress";

      div.innerHTML = `
        <div class="game-info">
          <div class="game-title-row">
            <span style="font-weight: 600; color: #f0f6fc; font-size: 14px;">${escapeHtml(g.name)}</span>
            <span class="anticheat-badge ${statusClass}">${escapeHtml(g.status)}${escapeHtml(anticheatsText)}</span>
          </div>
          <div class="game-meta" style="font-size: 12px; color: #8b949e; margin-top: 4px;">
            Cloud / Server Saves (No local saves needed) | ${sourceText}
          </div>
        </div>
      `;
      t.container.appendChild(div);
    });
  });
}

async function loadInitialOnlineGames() {
  try {
    const res = await fetch("/api/online-games");
    if (res.ok) {
      const data = await res.json();
      if (Array.isArray(data) && data.length > 0) {
        renderOnlineGames(data);
      }
    }
  } catch (e) {
    console.error("Failed to load online games:", e);
  }
}

// Backup Games
document.getElementById("btn-backup").addEventListener("click", async () => {
  const destPath = document.getElementById("backup-dest").value.trim();
  if (!destPath) {
    alert("Please enter a target backup folder (e.g. your USB drive).");
    return;
  }

  const selectedGames = [];
  document.querySelectorAll("#scan-results input[type='checkbox']:checked").forEach((chk) => {
    const idx = parseInt(chk.dataset.idx, 10);
    selectedGames.push(detectedGames[idx]);
  });

  if (selectedGames.length === 0) {
    alert("No games selected.");
    return;
  }

  const optRPG = document.getElementById("chk-optimize-rpg") ? document.getElementById("chk-optimize-rpg").checked : true;
  log(`Starting backup of ${selectedGames.length} games to ${destPath} (Smart RPG Optimizer: ${optRPG ? "ON" : "OFF"})...`);
  startBackupProgress(`Backing up ${selectedGames.length} games...`);

  try {
    const res = await fetch("/api/backup", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        games: selectedGames,
        dest_dir: destPath,
        create_single_zip: true,
        optimize_rpg: optRPG,
      }),
    });
    const result = await res.json();
    if (result.error) {
      log("Backup error: " + result.error);
      stopBackupProgress("Backup failed");
    } else {
      const manifest = result.manifest || result;
      const count = manifest.games ? manifest.games.length : (result.games ? result.games.length : selectedGames.length);
      log(`Backup successfully completed! Total games archived: ${count}`);
      const zipPath = result.zip_path || `${destPath}/game_saves_bundle.zip`;
      log(`Created archive: ${zipPath}`);

      // Verification Card
      const verifyCard = document.getElementById("backup-verify-card");
      if (result.verification && result.verification.valid) {
        const v = result.verification;
        const sizeMB = (v.zip_size / (1024 * 1024)).toFixed(2);
        verifyCard.className = "alert alert-info";
        verifyCard.innerHTML = `
          <div style="font-weight: 600;">Backup Integrity Verified (0 Errors)</div>
          <div style="font-size: 12px; color: #c9d1d9; margin-top: 3px;">
            Target Archive: <strong>${zipPath}</strong> (${sizeMB} MB)<br>
            All <strong>${v.entry_count} archive entries</strong> decompressed cleanly and passed CRC32 validation.
          </div>
        `;
        verifyCard.classList.remove("hidden");
        log(`Integrity test passed: ${v.entry_count} entries verified against CRC32.`);
      }

      // Show Safely Eject USB button if destination is a USB drive or contains drive letter
      const ejectBtn = document.getElementById("btn-eject-usb");
      if (destPath.includes(":") || destPath.includes("/media")) {
        ejectBtn.classList.remove("hidden");
        ejectBtn.disabled = false;
        ejectBtn.textContent = "Safely Eject USB";
      }

      saveAppConfig();
      stopBackupProgress(`Backup complete. ${count} games archived & verified.`);
      playWin7Tada();
      notifyBackground("Backup Complete");
      updateDiskSpaceBadge();
    }
  } catch (err) {
    log("Backup request failed: " + err);
    stopBackupProgress("Backup failed");
  }
});

document.getElementById("btn-eject-usb").addEventListener("click", async () => {
  const destPath = document.getElementById("backup-dest").value.trim();
  if (!destPath) return;

  log(`Requesting safe USB ejection for: ${destPath}...`);
  try {
    const res = await fetch("/api/eject-usb", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ path: destPath }),
    });
    const data = await res.json();
    if (data.error) {
      log(`Eject error: ${data.error}`);
    } else {
      log(data.message || "USB drive safely ejected.");
      const ejectBtn = document.getElementById("btn-eject-usb");
      ejectBtn.textContent = "USB Ejected";
      ejectBtn.disabled = true;
      document.getElementById("usb-detect-badge").classList.add("hidden");
    }
  } catch (err) {
    log(`Eject request error: ${err}`);
  }
});

// Toggle Steam Cloud filter
if (document.getElementById("chk-skip-steam-cloud")) {
  document.getElementById("chk-skip-steam-cloud").addEventListener("change", (e) => {
    const skip = e.target.checked;
    detectedGames.forEach((game, idx) => {
      if (game.is_steam_cloud) {
        const chk = document.getElementById(`game-chk-${idx}`);
        if (chk) {
          chk.checked = !skip;
        }
      }
    });
    updateDiskSpaceBadge();
  });
}

// Toggle User Settings filter
if (document.getElementById("chk-skip-user-settings")) {
  document.getElementById("chk-skip-user-settings").addEventListener("change", (e) => {
    const skip = e.target.checked;
    detectedGames.forEach((game, idx) => {
      if (game.is_user_settings) {
        const chk = document.getElementById(`game-chk-${idx}`);
        if (chk) {
          chk.checked = !skip;
        }
      }
    });
    updateDiskSpaceBadge();
  });
}

// Toggle Smart RPG Optimizer
const chkOptimizeRPG = document.getElementById("chk-optimize-rpg");
if (chkOptimizeRPG) {
  chkOptimizeRPG.addEventListener("change", (e) => {
    log(`Smart RPG Save Optimizer: ${e.target.checked ? "Enabled (prunes bloated autosaves/quicksaves)" : "Disabled (preserves all files)"}`);
  });
}

async function autoDetectUsbDrives() {
  try {
    const res = await fetch("/api/drives");
    const drives = await res.json();
    if (!drives || drives.length === 0) return;

    const badge = document.getElementById("usb-detect-badge");
    const destInput = document.getElementById("backup-dest");

    // Look for first removable drive (e.g. USB flash drive)
    const usb = drives.find(d => d.is_removable);
    if (usb) {
      if (!destInput.value || destInput.value.trim() === "") {
        destInput.value = usb.path;
        log(`Auto-detected USB drive: ${usb.label}. Auto-selected as backup destination.`);
      }
      if (badge) {
        badge.textContent = `Auto-detected USB: ${usb.label}`;
        badge.classList.remove("hidden");
      }
      updateDiskSpaceBadge();
    } else {
      if (badge) {
        badge.classList.add("hidden");
      }
    }
  } catch (err) {
    // Ignore drive fetch error
  }
}

// Mode toggle listeners
document.getElementById("btn-mode-restore")?.addEventListener("click", () => setAppMode("restore"));
document.getElementById("btn-mode-backup")?.addEventListener("click", () => setAppMode("backup"));

// ==========================================
// LINUX RESTORER WORKFLOW IMPLEMENTATION
// ==========================================

const vaultDropZone = document.getElementById("vault-drop-zone");
const vaultPathInput = document.getElementById("vault-path");
const btnBrowseVault = document.getElementById("btn-browse-vault");
const btnInspectVault = document.getElementById("btn-inspect-vault");
const restoreResultsDiv = document.getElementById("restore-results");
const btnStartRestore = document.getElementById("btn-start-restore");
const restoreSearchInput = document.getElementById("restore-search-input");
const btnRestoreClearSearch = document.getElementById("btn-restore-clear-search");
const restoreSearchCount = document.getElementById("restore-search-count");

// Drag & Drop
if (vaultDropZone) {
  vaultDropZone.addEventListener("click", () => {
    btnBrowseVault?.click();
  });

  vaultDropZone.addEventListener("dragover", (e) => {
    e.preventDefault();
    vaultDropZone.classList.add("dragover");
  });

  vaultDropZone.addEventListener("dragleave", () => {
    vaultDropZone.classList.remove("dragover");
  });

  vaultDropZone.addEventListener("drop", (e) => {
    e.preventDefault();
    vaultDropZone.classList.remove("dragover");
    if (e.dataTransfer && e.dataTransfer.files && e.dataTransfer.files.length > 0) {
      const file = e.dataTransfer.files[0];
      // On webview / Chromium, file.path is often available
      const fullPath = file.path || file.name;
      if (vaultPathInput) {
        vaultPathInput.value = fullPath;
        log(`Dropped vault file: ${fullPath}`);
        inspectVault(fullPath);
      }
    }
  });
}

// Browse for Vault Archive
if (btnBrowseVault) {
  btnBrowseVault.addEventListener("click", async () => {
    const selected = await pickFile("Select Game Save Vault Archive (*.zip)");
    if (selected && selected.trim() !== "") {
      if (vaultPathInput) {
        vaultPathInput.value = selected;
        inspectVault(selected);
      }
    }
  });
}

// Inspect Button
if (btnInspectVault) {
  btnInspectVault.addEventListener("click", () => {
    const path = vaultPathInput ? vaultPathInput.value.trim() : "";
    if (!path) {
      alert("Please select or enter the path to game_saves_bundle.zip first.");
      return;
    }
    inspectVault(path);
  });
}

// Inspect Vault Function
async function inspectVault(sourcePath) {
  if (!sourcePath) return;

  const spinner = document.getElementById("inspect-spinner");
  const btnText = document.getElementById("inspect-btn-text");
  if (spinner) spinner.classList.remove("hidden");
  if (btnText) btnText.textContent = "Inspecting...";
  log(`Inspecting vault archive: ${sourcePath}...`);

  try {
    const res = await fetch("/api/inspect", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ source_path: sourcePath })
    });
    const manifest = await res.json();

    if (manifest.error) {
      alert(`Error reading vault: ${manifest.error}`);
      log(`Vault inspection failed: ${manifest.error}`);
      return;
    }

    vaultGames = manifest.games || [];
    log(`Vault loaded successfully: ${vaultGames.length} game saves found.`);
    playWin7Notify();

    // Show summary card
    const summaryCard = document.getElementById("vault-summary-card");
    const summaryTitle = document.getElementById("vault-summary-title");
    const summaryDetails = document.getElementById("vault-summary-details");
    const summaryStats = document.getElementById("vault-summary-stats");

    if (summaryCard) {
      summaryCard.classList.remove("hidden");
      if (summaryTitle) summaryTitle.textContent = `Vault Archive: ${sourcePath.split("/").pop() || "Bundle"}`;
      if (summaryDetails) {
        const dateStr = manifest.created_at ? new Date(manifest.created_at).toLocaleString() : "Unknown date";
        summaryDetails.textContent = `Source OS: ${manifest.source_os || "Windows"} • Backed up: ${dateStr} • Version: ${manifest.version || "1.0"}`;
      }
      if (summaryStats) {
        let totalBytes = 0;
        vaultGames.forEach(g => totalBytes += (g.size_bytes || 0));
        summaryStats.textContent = `${vaultGames.length} Games (${formatBytes(totalBytes)})`;
      }
    }

    // Show restore games section
    const gamesCard = document.getElementById("restore-games-card");
    if (gamesCard) gamesCard.classList.remove("hidden");

    renderRestoreResults(vaultGames);

    if (manifest.online_games && manifest.online_games.length > 0) {
      renderOnlineGames(manifest.online_games, "online-games-restore-results", "online-games-restore-section");
    }

    if (btnStartRestore) {
      btnStartRestore.disabled = false;
    }
  } catch (err) {
    log(`Inspection network error: ${err}`);
  } finally {
    if (spinner) spinner.classList.add("hidden");
    if (btnText) btnText.textContent = "Inspect Vault";
  }
}

// Quick Load Found Vaults
async function loadFoundVaults() {
  try {
    const res = await fetch("/api/find-vaults");
    const data = await res.json();
    const vaults = data.vaults || [];
    const container = document.getElementById("quick-load-container");
    const chipsDiv = document.getElementById("quick-load-chips");

    if (!container || !chipsDiv) return;

    if (vaults.length > 0) {
      chipsDiv.innerHTML = "";
      vaults.forEach(vPath => {
        const chip = document.createElement("button");
        chip.type = "button";
        chip.className = "quick-chip";
        const filename = vPath.split("/").pop() || vPath;
        const dir = vPath.includes("/") ? vPath.split("/").slice(-2, -1)[0] : "";
        chip.textContent = dir ? `${filename} (${dir})` : filename;
        chip.title = vPath;
        chip.addEventListener("click", () => {
          if (vaultPathInput) {
            vaultPathInput.value = vPath;
            inspectVault(vPath);
          }
        });
        chipsDiv.appendChild(chip);
      });
      container.classList.remove("hidden");
    } else {
      container.classList.add("hidden");
    }
  } catch (err) {
    // Ignore find vaults error
  }
}

// Global Launcher Selection
const globalLauncherSelect = document.getElementById("global-launcher-select");
if (globalLauncherSelect) {
  globalLauncherSelect.addEventListener("change", (e) => {
    const chosen = e.target.value;
    log(`Global target launcher changed to: ${chosen.toUpperCase()}`);
    vaultGames.forEach((g, idx) => {
      const select = document.getElementById(`launcher-select-${idx}`);
      if (select) select.value = chosen;
      applyLauncherToGameRow(g, idx, chosen);
    });
  });
}

function applyLauncherToGameRow(game, idx, launcher) {
  const targetInput = document.getElementById(`restore-target-${idx}`);
  const statusBadge = document.getElementById(`restore-status-${idx}`);
  if (!targetInput || !statusBadge) return;

  if (launcher === "auto") {
    // Priority order: Hydra -> Heroic -> Lutris -> PortProton/Bottles -> Hydra (uninitialized)
    if (game.launcher_statuses?.hydra === "hydra_detected") {
      setRowTarget(idx, game.launcher_paths.hydra, "status-badge-hydra", "Hydra Prefix (Auto-Detected)");
    } else if (game.launcher_statuses?.heroic === "heroic_detected") {
      setRowTarget(idx, game.launcher_paths.heroic, "status-badge-heroic", "Heroic Prefix (Auto-Detected)");
    } else if (game.launcher_statuses?.lutris === "lutris_detected") {
      setRowTarget(idx, game.launcher_paths.lutris, "status-badge-lutris", "Lutris Prefix (Auto-Detected)");
    } else if (game.target_status === "emulator_detected") {
      setRowTarget(idx, game.suggested_linux_path, "status-badge-emu", game.target_status_text);
    } else if (game.launcher_paths?.hydra) {
      setRowTarget(idx, game.launcher_paths.hydra, "status-badge-seed", "Prefix Uninitialized (Auto-seeds on Restore)");
    } else if (game.launcher_paths?.heroic) {
      setRowTarget(idx, game.launcher_paths.heroic, "status-badge-seed", "Prefix Uninitialized (Auto-seeds on Restore)");
    } else {
      setRowTarget(idx, game.suggested_linux_path || "", "status-badge-manual", "Manual Target Required");
    }
  } else if (launcher === "hydra") {
    const p = game.launcher_paths?.hydra || "";
    const isDetected = game.launcher_statuses?.hydra === "hydra_detected";
    setRowTarget(idx, p, isDetected ? "status-badge-hydra" : "status-badge-seed",
      isDetected ? "Hydra Prefix (Auto-Detected)" : "Prefix Uninitialized (Auto-seeds on Restore)");
  } else if (launcher === "heroic") {
    const p = game.launcher_paths?.heroic || "";
    const isDetected = game.launcher_statuses?.heroic === "heroic_detected";
    setRowTarget(idx, p, isDetected ? "status-badge-heroic" : "status-badge-seed",
      isDetected ? "Heroic Prefix (Auto-Detected)" : "Prefix Uninitialized (Auto-seeds on Restore)");
  } else if (launcher === "lutris") {
    const p = game.launcher_paths?.lutris || "";
    const isDetected = game.launcher_statuses?.lutris === "lutris_detected";
    setRowTarget(idx, p, isDetected ? "status-badge-lutris" : "status-badge-seed",
      isDetected ? "Lutris Prefix (Auto-Detected)" : "Prefix Uninitialized (Auto-seeds on Restore)");
  } else if (launcher === "steam") {
    const p = game.launcher_paths?.steam || "";
    const isDetected = game.launcher_statuses?.steam === "proton_detected";
    setRowTarget(idx, p, isDetected ? "status-badge-proton" : "status-badge-seed",
      isDetected ? "Steam Proton Prefix" : "Prefix Uninitialized (Auto-seeds on Restore)");
  } else if (launcher === "custom") {
    statusBadge.className = "status-badge status-badge-manual";
    statusBadge.textContent = "Custom Prefix Selected";
  }
}

function setRowTarget(idx, path, statusClass, statusText) {
  const targetInput = document.getElementById(`restore-target-${idx}`);
  const statusBadge = document.getElementById(`restore-status-${idx}`);
  if (targetInput) targetInput.value = path;
  if (statusBadge) {
    statusBadge.className = `status-badge ${statusClass}`;
    statusBadge.textContent = statusText;
  }
}

async function validateAndSetCustomPrefix(idx, game, chosenFolder) {
  if (!chosenFolder || chosenFolder.trim() === "") return;

  const targetInput = document.getElementById(`restore-target-${idx}`);
  const statusBadge = document.getElementById(`restore-status-${idx}`);
  const select = document.getElementById(`launcher-select-${idx}`);

  if (currentPlatform === "windows") {
    if (targetInput) targetInput.value = chosenFolder;
    if (statusBadge) {
      statusBadge.className = "status-badge status-badge-proton";
      statusBadge.textContent = "✓ Custom Folder Selected";
    }
    log(`Custom Windows save destination set to: ${chosenFolder}`);
    return;
  }

  log(`Validating custom prefix: ${chosenFolder}...`);
  try {
    const res = await fetch(`/api/validate-prefix?path=${encodeURIComponent(chosenFolder)}`);
    const data = await res.json();

    if (select) select.value = "custom";

    if (data.valid && data.drive_c) {
      // It's an initialized prefix! Inject into drive_c
      const relPath = game.target_relative_path || "";
      const fullTarget = data.drive_c + "/" + relPath;
      if (targetInput) targetInput.value = fullTarget;
      if (statusBadge) {
        statusBadge.className = "status-badge status-badge-proton";
        statusBadge.textContent = "✓ Initialized Wine Prefix Validated";
      }
      log(`Prefix validated! Save destination set to: ${fullTarget}`);
    } else {
      // Uninitialized or not a prefix
      if (targetInput) targetInput.value = chosenFolder;
      if (statusBadge) {
        statusBadge.className = "status-badge status-badge-warning";
        statusBadge.textContent = "⚠️ Uninitialized / Invalid Prefix";
      }
      const errMsg = data.error || "Selected folder is not an initialized Wine/Proton prefix.";
      alert(`Prefix Warning:\n${errMsg}`);
      log(`Prefix warning for ${game.game_name}: ${errMsg}`);
    }
  } catch (err) {
    log(`Validation error: ${err}`);
  }
}

// Render Restore Results
function renderRestoreResults(games) {
  if (!restoreResultsDiv) return;
  restoreResultsDiv.innerHTML = "";

  if (!games || games.length === 0) {
    restoreResultsDiv.innerHTML = `<div class="empty-state">No save games found in this vault.</div>`;
    return;
  }

  const badgeCount = document.getElementById("restore-count-badge");
  if (badgeCount) badgeCount.textContent = `${games.length} Titles`;

  games.forEach((game, idx) => {
    const item = document.createElement("div");
    item.className = "game-item";
    item.id = `restore-card-${idx}`;

    const isWindows = currentPlatform === "windows";
    const isUninit = game.target_status === "not_initialized";

    // Status Badge styling
    let statusClass = "status-badge-manual";
    if (game.target_status === "hydra_detected") statusClass = "status-badge-hydra";
    else if (game.target_status === "heroic_detected") statusClass = "status-badge-heroic";
    else if (game.target_status === "lutris_detected") statusClass = "status-badge-lutris";
    else if (game.target_status === "proton_detected") statusClass = "status-badge-proton";
    else if (game.target_status === "emulator_detected") statusClass = "status-badge-emu";
    else if (game.target_status === "windows_detected" || game.target_status === "windows_ready") statusClass = "status-badge-proton";
    else if (isUninit) statusClass = "status-badge-seed";
    else if (game.target_status === "remembered") statusClass = "status-badge-proton";

    const statusBadgeText = isUninit ? "Prefix Uninitialized (Auto-seeds on Restore)" : (game.target_status_text || "Manual Target");
    const protonBadgeHtml = game.proton_tier ? `<span class="proton-badge proton-${game.proton_tier.toLowerCase()}">ProtonDB: ${game.proton_tier}</span>` : "";
    const iconHtml = game.icon_url ? `<img class="game-icon" src="${game.icon_url}" onerror="if(this.dataset.tried!=='1'&&'${game.appid}'){this.dataset.tried='1';this.src='https://shared.steamstatic.com/store_item_assets/steam/apps/${game.appid}/header.jpg';}else{this.style.display='none';}">` : "";

    const selectedLauncher = game.selected_launcher || "auto";

    const launcherControlsHtml = isWindows
      ? `<span style="font-size: 11px; color: #3fb950; font-weight: 600;">Windows Destination</span>`
      : `<div style="display: inline-flex; align-items: center; gap: 4px;">
            <label for="launcher-select-${idx}" style="font-size: 11px; color: #8b949e;">Launcher:</label>
            <select id="launcher-select-${idx}" class="launcher-select" style="font-size: 11px; padding: 2px 24px 2px 8px;">
              <option value="auto" ${selectedLauncher === 'auto' ? 'selected' : ''}>Auto (Hydra → Heroic → Lutris)</option>
              <option value="hydra" ${selectedLauncher === 'hydra' ? 'selected' : ''}>Hydra Launcher</option>
              <option value="heroic" ${selectedLauncher === 'heroic' ? 'selected' : ''}>Heroic Games Launcher</option>
              <option value="lutris" ${selectedLauncher === 'lutris' ? 'selected' : ''}>Lutris</option>
              <option value="steam" ${selectedLauncher === 'steam' ? 'selected' : ''}>Steam Proton</option>
              <option value="custom" ${selectedLauncher === 'custom' ? 'selected' : ''}>Custom Prefix</option>
            </select>
          </div>`;

    const browseBtnText = isWindows ? "Change Folder..." : "Change Prefix...";
    const placeholderText = isWindows ? "Click Change Folder to select destination..." : "Click Change Prefix to select prefix...";

    const hasConflict = !!game.has_local_conflict;
    let conflictHtml = "";
    if (hasConflict) {
      conflictHtml = `
        <div class="conflict-notice-row" style="margin-top: 8px; padding: 6px 10px; background: rgba(210, 153, 34, 0.15); border-left: 3px solid #d29922; border-radius: 4px;">
          <div style="display: flex; align-items: center; justify-content: space-between; gap: 8px; flex-wrap: wrap;">
            <div style="font-size: 11px; color: #f2cc60; font-weight: 500;">
              Notice: ${escapeHtml(game.conflict_notice || "Local save is newer than archive")}
            </div>
            <div style="display: inline-flex; align-items: center; gap: 6px;">
              <label for="conflict-action-${idx}" style="font-size: 11px; color: #c9d1d9;">Action:</label>
              <select id="conflict-action-${idx}" class="launcher-select" style="font-size: 11px; padding: 2px 20px 2px 6px;">
                <option value="skip" selected>Skip (Keep Newer Local)</option>
                <option value="backup_local">Backup Local and Overwrite</option>
                <option value="overwrite">Overwrite Local Save</option>
              </select>
            </div>
          </div>
        </div>
      `;
    }

    const isChecked = hasConflict ? "" : "checked";

    item.innerHTML = `
      <input type="checkbox" id="restore-chk-${idx}" class="restore-game-chk" ${isChecked} style="margin-top: 6px; cursor: pointer;">
      ${iconHtml}
      <div class="game-info">
        <div class="game-title-row" style="display: flex; align-items: center; gap: 8px; flex-wrap: wrap;">
          <span style="font-weight: 600; font-size: 14px; color: #f0f6fc;">${escapeHtml(game.game_name)}</span>
          ${game.appid ? `<span class="appid-badge">Steam AppID: ${game.appid}</span>` : ""}
          ${protonBadgeHtml}
          ${hasConflict ? `<span class="badge" style="background: #d29922; color: #0d1117; font-weight: bold; border-color: #d29922; font-size: 10px;">Local Save is Newer</span>` : ""}
          <span id="restore-status-${idx}" class="status-badge ${statusClass}" style="margin-left: auto;">${escapeHtml(statusBadgeText)}</span>
        </div>

        <div class="game-meta" style="display: flex; align-items: center; justify-content: space-between; gap: 8px; flex-wrap: wrap; margin-top: 4px;">
          <div>
            <span class="game-tag">${escapeHtml(game.storage_type || game.save_type)}</span>
            <span style="font-size: 11px; color: #8b949e;">${game.file_count || 0} files (${formatBytes(game.size_bytes || 0)})</span>
          </div>
          ${launcherControlsHtml}
        </div>

        <div class="restore-target-row" style="margin-top: 6px; display: flex; align-items: center; gap: 6px; flex-wrap: wrap;">
          <span style="font-size: 11px; color: #8b949e; white-space: nowrap;">Restore To:</span>
          <input type="text" id="restore-target-${idx}" class="restore-target-path" value="${escapeHtml(game.suggested_linux_path || "")}" placeholder="${placeholderText}">
          <button id="btn-browse-target-${idx}" class="btn-subtle btn-mini" type="button" style="font-size: 11px; padding: 2px 8px;">${browseBtnText}</button>
          ${isUninit ? `<button id="btn-seed-target-${idx}" class="btn-subtle btn-mini" type="button" style="font-size: 11px; padding: 2px 8px; color: #f2cc60; border-color: #d29922;" title="Select and pre-create Wine prefix on restore">Pre-Seed</button>` : ""}
        </div>
        ${conflictHtml}
      </div>
    `;

    restoreResultsDiv.appendChild(item);

    // Hooks
    setTimeout(() => {
      // Launcher select hook
      document.getElementById(`launcher-select-${idx}`)?.addEventListener("change", (e) => {
        applyLauncherToGameRow(game, idx, e.target.value);
      });

      // Manual browse button hook
      document.getElementById(`btn-browse-target-${idx}`)?.addEventListener("click", async () => {
        const pickerPrompt = isWindows ? `Select destination folder for ${game.game_name}` : `Select Wine/Proton Prefix or Destination folder for ${game.game_name}`;
        const selected = await pickFolder(pickerPrompt);
        if (selected && selected.trim() !== "") {
          await validateAndSetCustomPrefix(idx, game, selected);
        }
      });

      // Pre-seed button hook
      document.getElementById(`btn-seed-target-${idx}`)?.addEventListener("click", () => {
        const chk = document.getElementById(`restore-chk-${idx}`);
        if (chk) {
          chk.checked = true;
          log(`Selected ${game.game_name} for prefix pre-seeding at: ${game.suggested_linux_path}`);
        }
      });
    }, 10);
  });
}

// Bulk Selection
document.getElementById("btn-restore-select-all")?.addEventListener("click", () => {
  let count = 0;
  let seedCount = 0;
  vaultGames.forEach((g, idx) => {
    const chk = document.getElementById(`restore-chk-${idx}`);
    if (chk) {
      chk.checked = true;
      count++;
      if (g.target_status === "not_initialized") seedCount++;
    }
  });
  if (seedCount > 0) {
    log(`Selected all ${count} titles (${seedCount} uninitialized prefixes will be auto-seeded on restore).`);
  } else {
    log(`Selected all ${count} titles.`);
  }
});

document.getElementById("btn-restore-deselect-all")?.addEventListener("click", () => {
  vaultGames.forEach((_, idx) => {
    const chk = document.getElementById(`restore-chk-${idx}`);
    if (chk) chk.checked = false;
  });
  log("Deselected all titles.");
});

function isGameDetected(idx, game) {
  const badge = document.getElementById(`restore-status-${idx}`);
  if (badge) {
    if (badge.classList.contains("status-badge-warning") || badge.classList.contains("status-badge-manual") || badge.classList.contains("status-badge-seed")) {
      return false;
    }
    if (badge.classList.contains("status-badge-hydra") ||
        badge.classList.contains("status-badge-heroic") ||
        badge.classList.contains("status-badge-lutris") ||
        badge.classList.contains("status-badge-steam") ||
        badge.classList.contains("status-badge-emu") ||
        badge.classList.contains("status-badge-proton") ||
        badge.classList.contains("status-badge-seeded")) {
      return true;
    }
  }
  if (!game) return false;
  const status = game.target_status || "";
  if (status === "manual" || status === "not_initialized") return false;
  return status.endsWith("_detected") || status === "remembered" || status.startsWith("windows_");
}

document.getElementById("btn-restore-select-detected")?.addEventListener("click", () => {
  let detectedCount = 0;
  vaultGames.forEach((g, idx) => {
    const chk = document.getElementById(`restore-chk-${idx}`);
    if (chk) {
      const detected = isGameDetected(idx, g);
      chk.checked = detected;
      if (detected) detectedCount++;
    }
  });
  if (detectedCount === 0) {
    if (currentPlatform === "windows") {
      log("No detected Windows game destinations found yet.");
    } else {
      log("No games with initialized Wine/Proton prefixes found on disk yet. Use 'Select All (Auto-Seed Prefixes)' to pre-create prefixes automatically.");
    }
  } else {
    log(`Selected ${detectedCount} detected game(s).`);
  }
});

// Restore Search Filter
if (restoreSearchInput) {
  restoreSearchInput.addEventListener("input", () => {
    const query = restoreSearchInput.value.trim().toLowerCase();
    if (btnRestoreClearSearch) {
      if (query.length > 0) btnRestoreClearSearch.classList.remove("hidden");
      else btnRestoreClearSearch.classList.add("hidden");
    }

    let matchCount = 0;
    vaultGames.forEach((g, idx) => {
      const card = document.getElementById(`restore-card-${idx}`);
      if (!card) return;

      const nameMatch = (g.game_name || "").toLowerCase().includes(query);
      const appidMatch = (g.appid || "").toLowerCase().includes(query);
      const typeMatch = (g.storage_type || "").toLowerCase().includes(query);
      const targetMatch = (g.suggested_linux_path || "").toLowerCase().includes(query);

      if (query === "" || nameMatch || appidMatch || typeMatch || targetMatch) {
        card.style.display = "block";
        matchCount++;
      } else {
        card.style.display = "none";
      }
    });

    if (restoreSearchCount) {
      if (query.length > 0) {
        restoreSearchCount.textContent = `Showing ${matchCount} of ${vaultGames.length} titles`;
        restoreSearchCount.classList.remove("hidden");
      } else {
        restoreSearchCount.classList.add("hidden");
      }
    }
  });
}

if (btnRestoreClearSearch) {
  btnRestoreClearSearch.addEventListener("click", () => {
    if (restoreSearchInput) {
      restoreSearchInput.value = "";
      restoreSearchInput.dispatchEvent(new Event("input"));
    }
  });
}

// Execute Restore Button
if (btnStartRestore) {
  btnStartRestore.addEventListener("click", async () => {
    const sourcePath = vaultPathInput ? vaultPathInput.value.trim() : "";
    if (!sourcePath) {
      alert("No vault archive selected.");
      return;
    }

    const tasks = [];
    vaultGames.forEach((g, idx) => {
      const chk = document.getElementById(`restore-chk-${idx}`);
      if (chk && chk.checked) {
        const inp = document.getElementById(`restore-target-${idx}`);
        const customDir = inp ? inp.value.trim() : (g.suggested_linux_path || "");
        const actionEl = document.getElementById(`conflict-action-${idx}`);
        const conflictAction = actionEl ? actionEl.value : (g.has_local_conflict ? "skip" : "overwrite");
        tasks.push({
          meta: g,
          custom_dir: customDir,
          conflict_resolution: conflictAction
        });
      }
    });

    if (tasks.length === 0) {
      alert("Please select at least one game save to restore.");
      return;
    }

    const overwriteConflicts = tasks.filter(t => t.meta.has_local_conflict && t.conflict_resolution === "overwrite");
    if (overwriteConflicts.length > 0) {
      const names = overwriteConflicts.map(t => t.meta.game_name).join(", ");
      const confirmed = confirm(`Notice: The following titles have newer local saves on this machine:\n\n${names}\n\nOverwriting will replace your newer local progress with older backup saves. Proceed with overwrite?`);
      if (!confirmed) {
        return;
      }
    }

    btnStartRestore.disabled = true;
    const progressBox = document.getElementById("restore-progress-container");
    const progressBar = document.getElementById("restore-progress-bar-fill");
    const progressStatus = document.getElementById("restore-progress-status");
    const progressPercent = document.getElementById("restore-progress-percent");
    const progressDetail = document.getElementById("restore-progress-detail");
    const reportCard = document.getElementById("restore-report-card");

    if (reportCard) reportCard.classList.add("hidden");
    if (progressBox) progressBox.classList.remove("hidden");
    log(`Starting restoration of ${tasks.length} game saves from ${sourcePath}...`);

    let progressTimer = setInterval(async () => {
      try {
        const pRes = await fetch("/api/progress");
        const pData = await pRes.json();
        if (pData.phase === "restore") {
          const pct = Math.min(100, Math.max(0, pData.percent));
          if (progressBar) progressBar.style.width = `${pct}%`;
          if (progressPercent) progressPercent.textContent = `${Math.round(pct)}%`;
          if (progressStatus) progressStatus.textContent = pData.status;
          if (progressDetail) progressDetail.textContent = pData.detail;
        }
      } catch (e) {
        // Ignore progress poll error
      }
    }, 150);

    try {
      const res = await fetch("/api/restore", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          source_path: sourcePath,
          tasks: tasks
        })
      });

      clearInterval(progressTimer);

      const reports = await res.json();
      if (progressBox) progressBox.classList.add("hidden");
      btnStartRestore.disabled = false;

      let successCount = 0;
      let skippedCount = 0;
      let failCount = 0;

      reports.forEach(r => {
        if (r.skipped) {
          skippedCount++;
          log(`Skipped ${r.game_name}: ${r.message || "Preserved newer local save"}`);
          const cardIdx = vaultGames.findIndex(g => g.id === r.game_id || g.game_name === r.game_name);
          if (cardIdx !== -1) {
            const badge = document.getElementById(`restore-status-${cardIdx}`);
            if (badge) {
              badge.className = "status-badge status-badge-warning";
              badge.textContent = "Skipped (Newer Local Preserved)";
            }
          }
        } else if (r.success) {
          successCount++;
          log(`Restored ${r.game_name} -> ${r.target_resolved} (Steam Cloud Timestamp Protected)`);
          const cardIdx = vaultGames.findIndex(g => g.id === r.game_id || g.game_name === r.game_name);
          if (cardIdx !== -1) {
            const badge = document.getElementById(`restore-status-${cardIdx}`);
            if (badge) {
              const origText = badge.textContent || "";
              if (origText.includes("Uninitialized") || origText.includes("Auto-seed")) {
                badge.className = "status-badge status-badge-seeded";
                badge.textContent = "Prefix Pre-Created & Restored";
              } else {
                badge.className = "status-badge status-badge-proton";
                badge.textContent = "Restored";
              }
            }
          }
        } else {
          failCount++;
          log(`Failed to restore ${r.game_name}: ${r.error}`);
        }
      });

      playWin7Tada();
      notifyBackground("Restore Completed");

      if (reportCard) {
        reportCard.classList.remove("hidden");
        if (failCount === 0) {
          reportCard.className = "alert alert-info";
          reportCard.style.borderLeftColor = "#2ea043";
          reportCard.innerHTML = `
            <div>
              <div style="font-weight: 600; color: #2ea043;">Restoration Succeeded!</div>
              <div style="font-size: 12px; color: #c9d1d9; margin-top: 2px;">All ${successCount} selected game saves restored. File timestamps touched for Steam Cloud protection.</div>
            </div>
          `;
        } else {
          reportCard.className = "alert alert-warning";
          reportCard.innerHTML = `
            <div>
              <div style="font-weight: 600;">Restoration Complete with Warnings</div>
              <div style="font-size: 12px; color: #c9d1d9; margin-top: 2px;">${successCount} restored, ${failCount} failed. Review the operation log below for details.</div>
            </div>
          `;
        }
      }
    } catch (err) {
      clearInterval(progressTimer);
      if (progressBox) progressBox.classList.add("hidden");
      btnStartRestore.disabled = false;
      log(`Restore execution error: ${err}`);
      alert(`Restore execution error: ${err}`);
    }
  });
}

function escapeHtml(str) {
  if (!str) return "";
  return str.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");
}

async function initTelemetryUI() {
  const modal = document.getElementById("telemetry-modal");
  const chkShare = document.getElementById("chk-telemetry-share");
  const chkShareRestore = document.getElementById("chk-telemetry-share-restore");
  const btnAllow = document.getElementById("btn-telemetry-allow");
  const btnDeny = document.getElementById("btn-telemetry-deny");

  const syncCheckboxes = (enabled) => {
    if (chkShare) chkShare.checked = enabled;
    if (chkShareRestore) chkShareRestore.checked = enabled;
  };

  try {
    const res = await fetch("/api/telemetry/status");
    if (res.ok) {
      const data = await res.json();
      syncCheckboxes(!!data.enabled);
      if (!data.prompted && modal) {
        modal.classList.remove("hidden");
      }
    }
  } catch (e) {
    // Ignore error
  }

  if (btnAllow) {
    btnAllow.addEventListener("click", async () => {
      if (modal) modal.classList.add("hidden");
      syncCheckboxes(true);
      try {
        await fetch("/api/telemetry/set", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ enabled: true })
        });
        log("Anonymous detection reports enabled.");
      } catch (e) {}
    });
  }

  if (btnDeny) {
    btnDeny.addEventListener("click", async () => {
      if (modal) modal.classList.add("hidden");
      syncCheckboxes(false);
      try {
        await fetch("/api/telemetry/set", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ enabled: false })
        });
        log("Anonymous detection reports kept private.");
      } catch (e) {}
    });
  }

  const handleCheckboxToggle = async (val) => {
    syncCheckboxes(val);
    try {
      await fetch("/api/telemetry/set", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ enabled: val })
      });
      log(`Anonymous detection reports ${val ? "enabled" : "disabled"}.`);
    } catch (e) {}
  };

  if (chkShare) {
    chkShare.addEventListener("change", () => handleCheckboxToggle(chkShare.checked));
  }
  if (chkShareRestore) {
    chkShareRestore.addEventListener("change", () => handleCheckboxToggle(chkShareRestore.checked));
  }
}

// Application Auto-Updater Module
let activeUpdateInfo = null;
let updateProgressInterval = null;

async function checkForUpdates(manualTrigger = false) {
  const updateAvailableBtn = document.getElementById("btn-update-available");
  const updateVersionBadge = document.getElementById("update-badge-version");

  if (manualTrigger) {
    log("Checking GitHub for application updates...");
  }

  try {
    const response = await fetch(`/api/update/check?force=${manualTrigger ? "true" : "false"}`);
    const data = await response.json();

    if (data.error) {
      if (manualTrigger) {
        log(`Update check failed: ${data.error}`);
        alert(`Update check failed: ${data.error}`);
      }
      return;
    }

    if (data.update_available) {
      activeUpdateInfo = data;
      log(`New update available: ${data.latest_version} (Current: ${data.current_version})`);

      if (updateAvailableBtn && updateVersionBadge) {
        updateVersionBadge.textContent = `v${data.latest_version}`;
        updateAvailableBtn.classList.remove("hidden");
      }

      if (manualTrigger) {
        showUpdateModal();
      }
    } else {
      activeUpdateInfo = null;
      if (updateAvailableBtn) {
        updateAvailableBtn.classList.add("hidden");
      }
      if (manualTrigger) {
        log(`Checkpoint is up to date (version ${data.current_version}).`);
        alert(`Checkpoint is up to date (version ${data.current_version}).`);
      }
    }
  } catch (err) {
    if (manualTrigger) {
      log(`Network error checking for updates: ${err}`);
      alert(`Network error checking for updates: ${err}`);
    }
  }
}

function showUpdateModal() {
  if (!activeUpdateInfo) return;

  const modal = document.getElementById("update-modal");
  const badge = document.getElementById("update-modal-badge");
  const title = document.getElementById("update-modal-title");
  const notes = document.getElementById("update-modal-notes");
  const errorText = document.getElementById("update-error-text");
  const progressBox = document.getElementById("update-progress-container");
  const installBtn = document.getElementById("btn-update-install");
  const restartBtn = document.getElementById("btn-update-restart");
  const dismissBtn = document.getElementById("btn-update-dismiss");

  if (badge) badge.textContent = `v${activeUpdateInfo.latest_version}`;
  if (title) title.textContent = activeUpdateInfo.release_title || `Release v${activeUpdateInfo.latest_version}`;
  if (notes) notes.textContent = activeUpdateInfo.release_notes || "No release notes provided.";

  if (errorText) {
    errorText.textContent = "";
    errorText.classList.add("hidden");
  }

  if (progressBox) progressBox.classList.add("hidden");
  if (restartBtn) restartBtn.classList.add("hidden");
  if (installBtn) {
    installBtn.classList.remove("hidden");
    installBtn.disabled = false;
    installBtn.textContent = "Install Update";
  }
  if (dismissBtn) dismissBtn.disabled = false;

  if (modal) modal.classList.remove("hidden");
}

function initUpdaterUI() {
  const updateAvailableBtn = document.getElementById("btn-update-available");
  if (updateAvailableBtn) {
    updateAvailableBtn.addEventListener("click", () => {
      showUpdateModal();
    });
  }

  const checkUpdatesBtn = document.getElementById("btn-check-updates");
  if (checkUpdatesBtn) {
    checkUpdatesBtn.addEventListener("click", () => {
      checkForUpdates(true);
    });
  }

  const dismissBtn = document.getElementById("btn-update-dismiss");
  if (dismissBtn) {
    dismissBtn.addEventListener("click", () => {
      document.getElementById("update-modal")?.classList.add("hidden");
    });
  }

  const installBtn = document.getElementById("btn-update-install");
  if (installBtn) {
    installBtn.addEventListener("click", async () => {
      if (!activeUpdateInfo || !activeUpdateInfo.download_url) {
        alert("No compatible binary asset found for your platform in this release.");
        return;
      }

      const progressBox = document.getElementById("update-progress-container");
      const progressBar = document.getElementById("update-progress-bar");
      const progressPercent = document.getElementById("update-progress-percent");
      const progressStatus = document.getElementById("update-progress-status");
      const errorText = document.getElementById("update-error-text");
      const restartBtn = document.getElementById("btn-update-restart");

      installBtn.disabled = true;
      if (dismissBtn) dismissBtn.disabled = true;
      if (progressBox) progressBox.classList.remove("hidden");
      if (progressBar) progressBar.style.width = "0%";
      if (progressPercent) progressPercent.textContent = "0%";
      if (progressStatus) progressStatus.textContent = "Connecting to download server...";

      // Poll progress endpoint during download
      updateProgressInterval = setInterval(async () => {
        try {
          const res = await fetch("/api/progress");
          const p = await res.json();
          if (p && p.op === "update") {
            const pct = Math.min(Math.max(p.percent, 0), 100);
            if (progressBar) progressBar.style.width = `${pct}%`;
            if (progressPercent) progressPercent.textContent = `${Math.round(pct)}%`;
            if (progressStatus) progressStatus.textContent = p.status || "Downloading update...";
          }
        } catch (e) {}
      }, 250);

      try {
        log(`Downloading Checkpoint update from: ${activeUpdateInfo.download_url}`);
        const response = await fetch("/api/update/apply", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ download_url: activeUpdateInfo.download_url }),
        });

        clearInterval(updateProgressInterval);
        const result = await response.json();

        if (result.error) {
          log(`Update installation failed: ${result.error}`);
          if (errorText) {
            errorText.textContent = `Update failed: ${result.error}`;
            errorText.classList.remove("hidden");
          }
          installBtn.disabled = false;
          if (dismissBtn) dismissBtn.disabled = false;
          return;
        }

        log("Update installed successfully. Application ready for restart.");
        if (progressBar) progressBar.style.width = "100%";
        if (progressPercent) progressPercent.textContent = "100%";
        if (progressStatus) progressStatus.textContent = "Update installed successfully.";

        installBtn.classList.add("hidden");
        if (restartBtn) restartBtn.classList.remove("hidden");
        if (dismissBtn) dismissBtn.disabled = false;
      } catch (err) {
        clearInterval(updateProgressInterval);
        log(`Update error: ${err}`);
        if (errorText) {
          errorText.textContent = `Network error: ${err}`;
          errorText.classList.remove("hidden");
        }
        installBtn.disabled = false;
        if (dismissBtn) dismissBtn.disabled = false;
      }
    });
  }

  const restartBtn = document.getElementById("btn-update-restart");
  if (restartBtn) {
    restartBtn.addEventListener("click", async () => {
      restartBtn.disabled = true;
      restartBtn.textContent = "Restarting...";
      try {
        await fetch("/api/update/restart", { method: "POST" });
      } catch (e) {}
      setTimeout(() => {
        window.location.reload();
      }, 1500);
    });
  }
}

document.getElementById("btn-refresh").addEventListener("click", () => {
  loadSystemStatus();
  loadAppConfig();
  autoDetectUsbDrives();
  loadFoundVaults();
  loadInitialOnlineGames();
  initTelemetryUI();
  checkForUpdates(false);
  log("Refreshed status.");
});

window.addEventListener("focus", () => {
  autoDetectUsbDrives();
  loadFoundVaults();
});

// Initial boot
loadSystemStatus();
loadAppConfig();
autoDetectUsbDrives();
loadFoundVaults();
loadInitialOnlineGames();
initTelemetryUI();
initUpdaterUI();
checkForUpdates(false);
