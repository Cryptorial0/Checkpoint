<p align="center">
  <img src="assets/logo.png" alt="Checkpoint Logo" width="128" height="128">
</p>

<h1 align="center">Checkpoint</h1>

<p align="center">
  Game save backup and migration tool for Windows, Linux, and Steam Deck.
</p>

<p align="center">
  <a href="https://ko-fi.com/cryptorial"><img src="https://img.shields.io/badge/Support%20on%20Ko--fi-2ea043?style=for-the-badge&logo=ko-fi&logoColor=white" alt="Support on Ko-fi"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-GPLv3-blue.svg?style=for-the-badge" alt="License: GPL v3"></a>
  <a href="https://github.com/Cryptorial0/Checkpoint/releases"><img src="https://img.shields.io/badge/Release-v1.2.0-00d2aa?style=for-the-badge" alt="Release"></a>
</p>

Checkpoint backs up and restores PC game saves across Windows, Linux, and Steam Deck. Instead of hunting through AppData or matching Proton prefix IDs in `compatdata` by hand, Checkpoint finds your saves, packages them into a verified archive, and restores them to the correct paths on the destination machine.

---

## Downloads

Binaries for release 1.2.0:

| Platform | Format | Description | Download |
| :--- | :--- | :--- | :--- |
| Linux | AppImage | Portable package for all Linux distributions | [checkpoint-v1.2.0.AppImage](https://github.com/Cryptorial0/Checkpoint/releases/download/v1.2.0/checkpoint-v1.2.0.AppImage) |
| Linux | Binary | Standalone 64-bit ELF binary | [checkpoint](https://github.com/Cryptorial0/Checkpoint/releases/download/v1.2.0/checkpoint) |
| Windows | Executable | Standalone 64-bit Windows binary | [checkpoint.exe](https://github.com/Cryptorial0/Checkpoint/releases/download/v1.2.0/checkpoint.exe) |

Checksums and past versions are on the [GitHub Releases](https://github.com/Cryptorial0/Checkpoint/releases) page. Checkpoint is licensed under GPLv3. See [PRIVACY.md](PRIVACY.md) for telemetry details.

---

## Supported Platforms

Checkpoint handles save migration in any direction:
- Windows to Linux / Steam Deck: restores saves directly into Steam Proton prefixes (`compatdata`), Heroic, or Lutris with case-insensitive path traversal.
- Linux / Steam Deck to Windows: restores saves collected from Proton prefixes or Flatpaks into standard Windows directories (`Saved Games`, `AppData`, `Documents`).
- Windows to Windows: useful when reinstalling Windows or moving saves to another PC.
- Linux to Linux: transfers saves between a desktop Linux PC and a Steam Deck.

---

## Features

- Scans standard PC save directories (`Saved Games`, `Documents`, `My Games`, localized document paths, `AppData/Local`, `AppData/Roaming`, `AppData/LocalLow`).
- Handles publisher subfolders (Bethesda, Capcom, CD Projekt Red, Rockstar, Square Enix, WB Games).
- Detects offline Steam profiles and portable game directories.
- Supports console emulators and EmuDeck setups (Cemu, Citra, Dolphin, DuckStation, PCSX2, RPCS3, Ryujinx, ShadPS4, Xenia, Yuzu).
- Handles Linux filesystem case-sensitivity when restoring Windows save structures into ext4/btrfs Proton prefixes.
- Compares archive modification times against existing local saves to prevent overwriting newer progress, with automatic safety backups (`.checkpoint_bak_*`).
- Optional save pruner for RPGs that accumulate hundreds of autosaves and quicksaves (Baldur's Gate 3, Cyberpunk 2077, Skyrim, Witcher 3).
- Built-in updater with GitHub Releases checking and AppImage support.

---

## Usage

### GUI Mode

Run the executable without arguments to launch the desktop interface:

```bash
# Linux
./checkpoint

# Windows
checkpoint.exe
```

### CLI Mode

Checkpoint can also run in the terminal:

```bash
# Inspect an archive and show resolved target directories
checkpoint -restore "/path/to/game_saves_bundle.zip"

# Restore all saves automatically to detected paths
checkpoint -restore "/path/to/game_saves_bundle.zip" -auto

# Show version
checkpoint -v
```

| Flag | Description |
| :--- | :--- |
| `(none)` | Launches the GUI desktop interface. |
| `-restore <file.zip>` | Inspects the archive and lists destination paths. |
| `-restore <file.zip> -auto` | Restores all games to resolved paths without prompting. |
| `-v`, `--version` | Prints application version and exits. |

---

## Building from Source

### Requirements
- Go 1.20 or newer
- Linux: `gtk3` and `webkit2gtk-4.1`
- Windows (optional, for embedding icons/manifest): `go-winres`

### Linux
```bash
cd src
go build -ldflags="-s -w" -o ../bin/checkpoint .
```

### Windows
```bash
cd src
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -H=windowsgui" -o ../bin/checkpoint.exe .
```

---

## License

GNU General Public License v3.0. See [LICENSE](LICENSE) for the full text.
