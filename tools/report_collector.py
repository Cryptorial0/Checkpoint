#!/usr/bin/env python3
"""
Save Migrator Anonymous Telemetry Collector
Listens on port 18085 (behind Cloudflare Tunnel on reports.cryptori.al)
Stores sanitized JSON reports into /DATA/telemetry/reports/<timestamp>_<uuid>.json
"""

import http.server
import socketserver
import json
import os
import uuid
from datetime import datetime

PORT = int(os.environ.get("PORT", 18085))
STORAGE_DIR = os.environ.get("REPORTS_DIR", "/DATA/telemetry/reports")

os.makedirs(STORAGE_DIR, exist_ok=True)

class TelemetryHandler(http.server.BaseHTTPRequestHandler):
    def do_POST(self):
        if self.path == "/api/report":
            content_length = int(self.headers.get("Content-Length", 0))
            if content_length > 2 * 1024 * 1024:  # Limit payload to 2MB
                self.send_error(413, "Payload too large")
                return

            body = self.rfile.read(content_length)
            try:
                data = json.loads(body.decode("utf-8"))
            except Exception as e:
                self.send_error(400, f"Invalid JSON: {e}")
                return

            # Save report to disk
            ts = datetime.utcnow().strftime("%Y%m%d_%H%M%S")
            short_id = uuid.uuid4().hex[:8]
            os_name = data.get("os", "unknown")
            filename = f"report_{ts}_{os_name}_{short_id}.json"
            filepath = os.path.join(STORAGE_DIR, filename)

            with open(filepath, "w", encoding="utf-8") as f:
                json.dump(data, f, indent=2)

            print(f"[{datetime.utcnow().isoformat()}] Received report ({data.get('total_titles', 0)} titles) -> {filename}")

            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            self.wfile.write(b'{"status":"saved"}\n')
        else:
            self.send_error(404, "Endpoint not found")

    def do_GET(self):
        if self.path == "/healthz" or self.path == "/api/health":
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            self.wfile.write(b'{"status":"healthy"}\n')
        else:
            self.send_error(404, "Not found")

    def log_message(self, format, *args):
        # Override standard noisy logging
        pass

if __name__ == "__main__":
    with socketserver.TCPServer(("", PORT), TelemetryHandler) as httpd:
        print(f"Save Migrator Telemetry Collector running on port {PORT}")
        print(f"Saving reports to: {STORAGE_DIR}")
        try:
            httpd.serve_forever()
        except KeyboardInterrupt:
            print("\nShutting down collector.")
