# Changelog

All notable changes to Checkpoint are documented in this file.

## [1.2.3] - 2026-11-11

### Fixed
- Patched WebKit2GTK hardcoded PKGLIBEXECDIR inside the bundled shared library to resolve helper process execution in unprivileged container environments.
- Staged WebKit helper processes (WebKitNetworkProcess and WebKitWebProcess) into /tmp at runtime to bypass container path isolation.
- Configured WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS=1 to support execution in nested firejail test runners.

## [1.2.2] - 2026-11-11

### Fixed
- Fixed WebKit2GTK child process spawning crash in AppImages (`WebKitNetworkProcess` / `WebKitWebProcess` no such file or directory) by bundling helper processes into `AppDir/usr/lib/x86_64-linux-gnu/webkit2gtk-4.1`.
- Configured runtime environment variables (`WEBKIT_EXEC_PATH`, `WEBKIT_INJECTED_BUNDLE_PATH`, `WEBKIT_FORCE_SANDBOX=0`) in both `AppRun` and native Linux window initialization.
- Added software compositing fallbacks for headless CI environments, nested Firejail containers, and NVIDIA drivers.

## [1.2.1] - 2026-11-11

### Fixed
- Fixed AppImage portability failure where `libwebkit2gtk-4.1.so.0` was missing on minimal Linux systems and SteamOS.
- Integrated `linuxdeploy` and `linuxdeploy-plugin-gtk` into GitHub Actions release workflow to bundle WebKit2GTK, GTK3, and shared runtime libraries into the AppImage.
- Lowered Linux CI build runner target from Ubuntu 24.04 to Ubuntu 22.04 LTS to reduce required GLIBC version to 2.35 for wider distribution compatibility.

## [1.2.0] - 2026-11-08

### Added
- Added scanner and restorer detection for standalone offline profiles (.1911, Fairlight, and EMPRESS AppData roaming structures).
- Added in-tree save detection and title alias resolution for Lies of P (AppID 1627720) across Unreal Engine portable directories.
- Strengthened tarball creation resilience against dehydrated cloud storage placeholders (OneDrive Files On-Demand) to prevent corrupted archives and unexpected EOF errors.

### Fixed
- Fixed tar stream alignment when source files report nominal sizes but yield partial reads, padding to promised byte lengths to preserve archive structure.
- Explicitly flushed and closed tar and gzip stream writers during vault packaging.
- Sanitized internal alignment handlers and storage labels for standalone offline profiles.

## [1.1.5] - 2026-11-08

### Fixed
- Fixed Red Dead Redemption 2 and Grand Theft Auto V restore failures on Linux prefixes where games ran under the Goldberg SocialClub emulator, standalone .1911 profiles, or non-steamuser Wine usernames.
- Added automatic Rockstar Games alignment routine (HandleRockstarGamesAlignment) that auto-seeds saves into all active profile subfolders, Goldberg SocialClub Emu paths, and standalone .1911 directories.
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
