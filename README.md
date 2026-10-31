<p align="center">
  <img src="assets/logo.png" alt="Checkpoint Logo" width="128" height="128">
</p>

<h1 align="center">Checkpoint</h1>

<p align="center">
  <strong>Universal game save manager, backup vault, and cross-platform migration suite for Windows and Linux / Steam Deck.</strong>
</p>

<p align="center">
  <a href="https://ko-fi.com/cryptorial"><img src="https://img.shields.io/badge/Support%20on%20Ko--fi-2ea043?style=for-the-badge&logo=ko-fi&logoColor=white" alt="Support on Ko-fi"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-GPLv3-blue.svg?style=for-the-badge" alt="License: GPL v3"></a>
  <a href="https://github.com/Cryptorial0/Checkpoint/releases"><img src="https://img.shields.io/badge/Release-v1.1.2-00d2aa?style=for-the-badge" alt="Release"></a>
</p>

Checkpoint is a game save manager and migration tool designed for modern PC gaming setups. It automatically discovers where your games store progress, packages them into verified portable archives, and restores them seamlessly across operating systems and launcher environments.

Whether you are switching between a desktop PC and a Steam Deck, upgrading your Windows rig, or moving across Linux distributions, Checkpoint bridges the gap between Windows directories, Steam Proton prefixes, and standalone emulators with zero manual file shuffling.

---

## Releases & Downloads (v1.1.2)

Pre-built binaries for **Release 1.1.2**:

| Platform | Package | Description | Binary |
| :--- | :--- | :--- | :--- |
| **Linux** | **AppImage** | Standalone portable bundle (runs on all Linux distributions) | [checkpoint-v1.1.2.AppImage](https://github.com/Cryptorial0/Checkpoint/releases/download/v1.1.2/checkpoint-v1.1.2.AppImage) |
| **Linux** | **Binary** | Native 64-bit ELF executable | [checkpoint](https://github.com/Cryptorial0/Checkpoint/releases/download/v1.1.2/checkpoint) |
| **Windows** | **Executable** | Standalone 64-bit Windows portable application | [checkpoint.exe](https://github.com/Cryptorial0/Checkpoint/releases/download/v1.1.2/checkpoint.exe) |

> [!NOTE]
> All binary artifacts and release checksums are published on the [GitHub Releases](https://github.com/Cryptorial0/Checkpoint/releases) page. Checkpoint is free open-source software under the GPLv3 license. See [PRIVACY.md](PRIVACY.md) for details.

---

## Supported Migration Paths

Checkpoint supports full bi-directional backup and restore across all major PC gaming environments:

- **Windows -> Linux / Steam Deck:** Restores PC saves directly into Steam Proton prefixes (`compatdata`), Hydra Launcher prefixes, Heroic Games Launcher, or Lutris prefixes with case-insensitive path navigation.
- **Linux / Steam Deck -> Windows:** Restores saves gathered from Proton prefixes, Flatpaks, or EmuDeck directly into native Windows paths (`%USERPROFILE%\Saved Games`, `AppData\Local`, `AppData\Roaming`, `AppData\LocalLow`, and `Documents`).
- **Windows -> Windows:** Perfect for fresh Windows installations, upgrading to a new machine, or transferring saves to a secondary gaming laptop.
- **Linux -> Linux:** Seamless migration between Steam Deck and Linux desktops, or when migrating between Linux distributions.

---

## Core Capabilities

### 1. Automated Game Save Discovery
- **Standard PC Game Trees:** Scans `%USERPROFILE%\Saved Games`, `Documents`, `My Games`, localized document paths (e.g., Arabic `المستندات`), `AppData\Local`, `AppData\LocalLow`, and `AppData\Roaming`.
- **Publisher Umbrella Directories:** Correctly parses publisher structures (such as WB Games, Square Enix, Rockstar Games, Capcom, CD Projekt Red, and Bethesda Softworks) to locate individual game progress.
- **Offline Steam Profiles & Custom Launchers:** Detects save game data and configuration metadata stored in Public Documents, offline Steam wrappers (such as Goldberg SteamEmu), and custom game directory hierarchies.
- **Console Emulators:** Packages memory cards, virtual flash drives, and save states for:
  - **PlayStation:** ShadPS4 (PS4), RPCS3 (PS3), PCSX2 (PS2), DuckStation (PS1)
  - **Nintendo:** Ryujinx (Switch), Yuzu (Switch), Eden (Switch), Cemu (Wii U), Citra (3DS), Dolphin (GameCube / Wii)
  - **Xbox:** Xenia (Xbox 360)
  - **EmuDeck & Flatpak Integrations:** Scans both native and containerized emulator installations automatically.
- **Steam Userdata:** Scans Steam Cloud userdata across all local account IDs on the system.
- **Portable & In-Tree Games:** Identifies Unreal Engine and portable game folders storing saves directly within game install roots.

### 2. Smart RPG Save Optimizer
Large role-playing games (such as Baldur's Gate 3, Starfield, Cyberpunk 2077, Skyrim, Fallout 4, and The Witcher 3) often accumulate gigabytes of autosaves and quicksaves. Checkpoint can automatically prune redundant autosaves while strictly preserving 100% of your named manual checkpoints.

### 3. Conflict Prevention & Timestamp Diffing
Before restoring, Checkpoint inspects destination directories and compares local file modification times against archive timestamps. If a newer local save already exists, Checkpoint:
- Flags the game with a clear notification and unchecks it by default.
- Offers granular resolution choices: `Skip (Keep Newer Local)`, `Backup Local and Overwrite`, or `Overwrite Local Save`.
- Automatically generates timestamped safety copies (`.checkpoint_bak_YYYYMMDD_HHMMSS`) if backup-and-overwrite is chosen.

### 4. Cross-Platform Engine Intelligence
- **Filesystem Casing Resolver:** Linux filesystems (ext4, btrfs) are case-sensitive while Windows is not. Checkpoint navigates target paths case-insensitively so save files land where the game expects them without manual renaming.
- **Steam Cloud Timestamp Protection:** Sets timestamps on restored saves to ensure Steam Cloud does not overwrite fresh restores with older remote cloud data upon game launch.
- **Prefix Auto-Seeding:** Detects uninitialized Wine and Proton prefixes on Linux and pre-creates necessary directory skeletons and registry keys on demand.
- **Canonical Symlink Deduplication:** Evaluates filesystem symbolic links to prevent duplicate detection when Wine prefixes share the host user's documents directory.
- **Online Game & Anti-Cheat Awareness:** Identifies installed online multiplayer titles with server-side progress and displays Linux anti-cheat status.

### 5. In-App Auto-Updater
Checkpoint includes an integrated updater that queries official GitHub Releases, respects API rate limits, and performs atomic binary replacement on Linux and safe executable replacement on Windows.

---

## Usage

### Desktop Application

Launch the desktop executable:
```bash
# On Linux
./bin/checkpoint

# On Windows
checkpoint.exe
```

The desktop interface automatically adapts to your operating system, providing one-click backup and restore modes, directory selectors, and progress tracking.

### Command Line Interface (CLI)

Checkpoint can run headlessly for terminal scripts and automated workflows:

```bash
# Inspect a backup vault and show detected destination paths
./bin/checkpoint -restore "/path/to/game_saves_bundle.zip"

# Automated restore to all auto-located destinations
./bin/checkpoint -restore "/path/to/game_saves_bundle.zip" -auto

# Show application version
./bin/checkpoint -v
```

### CLI Flags

| Flag | Description |
| :--- | :--- |
| `(none)` | Launches the graphical desktop interface. |
| `-restore <vault.zip>` | Inspects the archive and displays mapped target directories in terminal. |
| `-restore <vault.zip> -auto` | Restores all games to their resolved target paths automatically. |
| `-v`, `--version` | Displays the current version and exits. |

---

## Release Highlights

- **v1.1.2 (Current):**
  - **Publisher Subdirectory Resolution:** Properly categorizes publisher umbrella directories (such as WB Games, Square Enix, and BioWare) inside Documents and My Games to identify child game saves and eliminate false positive publisher entries.
  - **Canonical Symlink Deduplication:** Resolves symbolic links across Steam installations and Wine prefixes (such as ~/.steam/steam and shared Documents folders) to prevent duplicate save detection across prefixes.
  - **Local Steam Manifest Title Discovery:** Reads local Steam library appmanifest files to identify unlisted, playtest, and offline titles without requiring Steam Web API access.
  - **Unreal Project Name Normalization:** Resolves internal Unreal Engine project folders (such as Palworld and Deep Rock Galactic: Rogue Core) to their canonical titles.

- **v1.1.1:**
  - **Conflict Prevention & Timestamp Diffing:** Compares destination files against archive timestamps to prevent accidental overwrite of newer local save progress during migration, with automated safety backups.

- **v1.1.0:**
  - **In-App Auto-Updater:** Built-in updater engine querying official GitHub Releases with rate-limit protection, atomic binary replacement on POSIX/Linux, and safe running-executable migration on Windows.
  - **Smart RPG Save Optimizer:** Automatically prunes bloated, redundant autosaves and quicksaves for large RPGs while strictly preserving manual checkpoints.
  - **Heroic and Custom Prefix Auto-Discovery:** Seamlessly scans custom Wine/Proton directories and resolves game titles by executable name or AppID.

- **v1.0.0:**
  - **Bi-Directional Migration:** Full cross-platform support across Windows -> Linux / Steam Deck, Linux -> Windows, Windows -> Windows, and Linux -> Linux.
  - **Native Linux Host Backup Scanner:** Scans and packages save files directly on Linux from Steam Proton prefixes (`compatdata`), Hydra, Heroic, Lutris, and EmuDeck.
  - **Uninitialized Wine Prefix Auto-Seeding:** Pre-creates Wine/Proton prefixes on demand with proper user hierarchies.
  - **Steam Cloud Timestamp Protection:** Prevents cloud sync from overwriting newly restored save files.

---

## Building from Source

### Prerequisites
- Go 1.20 or newer
- On Linux: `gtk3` and `webkit2gtk-4.1`
- On Windows (optional, for embedding application icons/manifest): `go-winres`

### Build (Linux)
```bash
cd src
go build -ldflags="-s -w" -o ../bin/checkpoint .
```

### Build (Windows)
```bash
cd src
# Optional resource embedding:
# go-winres make --arch amd64
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -H=windowsgui" -o ../bin/checkpoint.exe .
```

---

## License

This project is licensed under the GNU General Public License v3.0. See the [LICENSE](LICENSE) file for details.
