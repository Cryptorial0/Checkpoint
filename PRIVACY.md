# Privacy Policy

**Effective Date:** October 26, 2026  
**Project:** Checkpoint (Save Migrator)  
**Repository:** https://github.com/Cryptorial0/Checkpoint

## Overview
Checkpoint is designed with a strict privacy-first philosophy. By default, Checkpoint operates entirely locally and offline on your computer.

## Data Collection and Telemetry
Checkpoint includes an optional, opt-in anonymous diagnostic reporting feature to improve game detection compatibility for unknown indie and standalone titles.

- **Explicit Opt-In:** Telemetry is disabled by default until the user explicitly opts in via the first-run prompt or settings toggle.
- **Client-Side Sanitization:** All personal user paths and usernames (`C:\Users\<username>\...`) are scrubbed and replaced with generic environment variables (`%LOCALAPPDATA%`, `%DOCUMENTS%`, `%USERPROFILE%`) before submission.
- **Zero Personal Files:** Checkpoint never accesses, inspects, bundles, or transmits save game file contents, credentials, computer names, MAC addresses, or IP identifiers.
- **Transmission:** When enabled, reports are transmitted securely over HTTPS to `reports.cryptori.al`.
- **Control:** Users can disable diagnostic reporting at any time in the application settings.

## Open Source Integrity
Checkpoint is open-source software licensed under GNU General Public License v3.0. The complete source code and telemetry sanitization pipeline are publicly auditable.
