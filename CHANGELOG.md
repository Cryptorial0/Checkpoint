# Changelog

All notable changes to Checkpoint are documented in this file.

## [1.1.5] - 2026-11-08

### Fixed
- Fixed Red Dead Redemption 2 and Grand Theft Auto V restore failures on Linux prefixes where games ran under the Goldberg SocialClub emulator, EMPRESS/.1911, or non-steamuser Wine usernames.
- Added automatic Rockstar Games alignment routine (HandleRockstarGamesAlignment) that auto-seeds saves into all active profile subfolders, Goldberg SocialClub Emu paths, and EMPRESS/.1911 directories.
- Mirrored restored Rockstar save data across all detected Wine user profiles (drive_c/users/steamuser and drive_c/users/<user>).
- Added scanner detection for Goldberg SocialClub Emu Saves in AppData Roaming on both Windows and Linux prefixes.

### Added
- Updated default backup vault archive naming scheme from game_saves_bundle.zip to checkpoint-ddmmyy.zip (e.g. checkpoint-081126.zip) based on creation date.
- Added candidate vault discovery support for checkpoint-*.zip archives while preserving full backwards compatibility for legacy game_saves_bundle.zip archives.

## [1.1.4] - 2026-11-07

### Fixed
- Fixed portable Xenia scanner false-positive regression that mistook arbitrary content directories (such as The Witcher 3 and Roblox asset trees) for Xbox 360 save data.
- Added strict 16-character hexadecimal profile ID and 8-character hexadecimal title ID validation for Xbox 360 save detection.
- Required emulator executable presence before falling back to whole-folder portable Xenia save packaging.
- Fixed Windows Fast Startup toggle permissions failure by requesting UAC elevation when modifying system power configuration and registry.
- Fixed initial view loading so the Backup Saves section renders immediately on startup without requiring switching to Restore and back.

## [1.1.3] - 2026-10-31

### Fixed
- Fixed AppImage self-update collisions by targeting the host binary through $APPIMAGE instead of the read-only squashfs mount.
- Registered Wayland app_id and StartupWMClass so desktop environments show the application icon instead of a generic placeholder.
- Added canonical path resolution to deduplicate Steam library symlinks across ~/.steam and ~/.local/share/Steam.
- Filtered 2-byte empty configuration files from offline emulator profiles.
- Improved game title disambiguation for titles sharing franchise prefixes.

## [1.1.2] - 2026-10-31

### Added
- Parser for local Steam appmanifest files to identify unlisted and offline games without internet access.
- Name normalization for internal Unreal Engine project folders.

### Fixed
- Categorized publisher umbrella folders (WB Games, Square Enix, BioWare) inside Documents to find actual game subdirectories.
- Prevented duplicate save detection across shared Wine prefixes.

## [1.1.1] - 2026-10-30

### Added
- Timestamp comparison between archive files and destination directories before restoring.
- Automatic safety backups (.checkpoint_bak_YYYYMMDD_HHMMSS) when overwriting newer local saves.

## [1.1.0] - 2026-10-30

### Added
- In-app auto-updater querying GitHub Releases with rate-limit protection.
- RPG save optimizer to prune excessive autosaves while preserving manual saves.
- Auto-discovery for Heroic Games Launcher and custom Wine/Proton prefixes.

## [1.0.0] - 2026-10-29

### Added
- Initial stable release of universal save migration.
- Bi-directional restore across Windows and Linux / Steam Deck.
- Native Linux host backup scanner for Proton prefixes and emulators.
- Automatic directory seeding for uninitialized Wine prefixes.
- Timestamp touch on restored files to prevent Steam Cloud overwrite conflicts.
