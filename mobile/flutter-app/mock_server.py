"""
Mock API server for SoHoLINK Flutter preview.
Serves the Flutter web build at / and mock REST endpoints at /api/*.

Auth endpoints (/api/auth/challenge, /api/auth/connect) are accepted without
real verification so the setup screen works against this mock server.
Any non-empty 64-char hex string is treated as a valid owner private key.
"""
import json
import os
import secrets
from http.server import HTTPServer, SimpleHTTPRequestHandler

MOCK = {
    "/api/health": {"status": "ok"},
    "/api/status": {
        "uptime_seconds": 307200,
        "os": "Windows 11",
        "active_rentals": 3,
        "federation_nodes": 12,
        "mobile_nodes": 4,
        "earned_sats_today": 48250,
        "cpu_offered_pct": 75,
        "cpu_used_pct": 0.62,
        "ram_offered_gb": 16.0,
        "ram_used_pct": 0.44,
        "storage_offered_gb": 500.0,
        "storage_used_pct": 0.31,
        "net_offered_mbps": 100,
        "net_used_pct": 0.28,
        "btc_usd_rate": 95000.0,
    },
    "/api/peers": {
        "count": 3,
        "peers": [
            {"id": "p1", "did": "did:key:z6Mk1", "address": "192.168.1.42:9000",
             "region": "us-east-1", "latency_ms": 12, "status": "connected", "last_seen_unix": 1741046400},
            {"id": "p2", "did": "did:key:z6Mk2", "address": "10.0.0.55:9000",
             "region": "eu-west-1", "latency_ms": 88, "status": "connected", "last_seen_unix": 1741046200},
            {"id": "p3", "did": "did:key:z6Mk3", "address": "172.16.0.9:9000",
             "region": "ap-south-1", "latency_ms": 210, "status": "degraded", "last_seen_unix": 1741045000},
        ],
    },
    "/api/revenue": {
        "earned_sats_total": 1842000,
        "earned_sats_today": 48250,
        "earned_sats_7d": 312400,
        "earned_sats_30d": 1105000,
        "fee_pct": 1.0,
        "net_sats_today": 47768,
        "btc_usd_rate": 95000.0,
        "history": [
            {"date": "2026-03-04", "sats": 48250},
            {"date": "2026-03-03", "sats": 41200},
            {"date": "2026-03-02", "sats": 38900},
            {"date": "2026-03-01", "sats": 52100},
            {"date": "2026-02-28", "sats": 29800},
            {"date": "2026-02-27", "sats": 44600},
            {"date": "2026-02-26", "sats": 36700},
        ],
    },
    "/api/workloads": {
        "count": 2,
        "btc_usd_rate": 95000.0,
        "workloads": [
            {"id": "wl1", "name": "nginx-proxy", "tenant_did": "did:key:xyz1",
             "status": "running", "cpu_millis": 500, "ram_mb": 512,
             "storage_mb": 5120, "started_unix": 1741003200, "earned_sats": 18400},
            {"id": "wl2", "name": "ml-inference", "tenant_did": "did:key:xyz2",
             "status": "running", "cpu_millis": 2000, "ram_mb": 4096,
             "storage_mb": 20480, "started_unix": 1741006800, "earned_sats": 29850},
        ],
    },
}

WEB_DIR = os.path.join(os.path.dirname(__file__), "build", "web")

# In-memory nonce store for mock auth (maps nonce -> True).
_nonces = {}


class Handler(SimpleHTTPRequestHandler):
    def __init__(self, *args, **kwargs):
        super().__init__(*args, directory=WEB_DIR, **kwargs)

    # ── CORS pre-flight ──────────────────────────────────────────────────────

    def do_OPTIONS(self):
        self.send_response(204)
        self._cors()
        self.end_headers()

    # ── GET ──────────────────────────────────────────────────────────────────

    def do_GET(self):
        path = self.path.split("?")[0]

        # Mock auth challenge — issue a fake nonce.
        if path == "/api/auth/challenge":
            nonce = secrets.token_hex(32)
            _nonces[nonce] = True
            self._json({"nonce": nonce, "expires_at": "2099-01-01T00:00:00Z"})
            return

        if path in MOCK:
            self._json(MOCK[path])
        else:
            super().do_GET()

    # ── POST ─────────────────────────────────────────────────────────────────

    def do_POST(self):
        path = self.path.split("?")[0]

        # Mock auth connect — accept any valid-looking request and return a token.
        if path == "/api/auth/connect":
            length = int(self.headers.get("Content-Length", 0))
            body = self.rfile.read(length)
            try:
                data = json.loads(body)
            except Exception:
                self.send_error(400, "bad JSON")
                return
            nonce = data.get("nonce", "")
            # Accept any nonce that was issued by this mock (or any nonce for
            # simplicity — this is a dev-only server with no real security).
            _nonces.pop(nonce, None)
            fake_token = secrets.token_hex(32)
            self._json({"device_token": fake_token})
            return

        self.send_error(404, "Not found")

    # ── Helpers ──────────────────────────────────────────────────────────────

    def _cors(self):
        self.send_header("Access-Control-Allow-Origin", "*")
        self.send_header("Access-Control-Allow-Headers", "Authorization, Content-Type")
        self.send_header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")

    def _json(self, data):
        body = json.dumps(data).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self._cors()
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, fmt, *args):
        pass  # quiet


if __name__ == "__main__":
    port = 4000
    print(f"SoHoLINK mock server -> http://localhost:{port}")
    HTTPServer(("", port), Handler).serve_forever()
