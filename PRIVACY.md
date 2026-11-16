# Privacy

Checkpoint runs offline on your machine.

Telemetry is disabled by default. If you opt into diagnostic reporting:
- Only unmapped save folder names and game executable names are sent so new game paths can be identified.
- Usernames and home folder roots (`C:\Users\<user>\` or `/home/<user>/`) are stripped locally before transmission.
- Save contents, files, hostnames, and IP logs are never collected.
- Diagnostic payloads post over HTTPS to `reports.cryptori.al`.

You can toggle reporting off at any time in the settings menu.
