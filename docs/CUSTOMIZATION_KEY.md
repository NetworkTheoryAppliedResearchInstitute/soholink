# SoHoLINK — Customization Key

*Version: 1.0 | Effective Date: 2026-03-06*
*Operator: Network Theory Applied Research Institute*

> **DRAFT — Review before public release.**

This document maps every customizable system behaviour to the exact `file:line` numbers that
must be edited. For each axis you get:

- **What it controls** — one-sentence description
- **Find it** — a `grep` command to locate the line
- **Current value** — exactly what is in the source today
- **How to change** — edit instruction
- **Restart required?** — yes / no / config-only

Use this as a changelog target: when you change any listed value, record the new value here
so future maintainers know the intended state.

---

## How to Use This Document

1. Identify the behaviour you want to change from the index below.
2. Run the **Find it** command to confirm the line still matches (line numbers shift as code evolves).
3. Make the edit described in **How to change**.
4. Rebuild if required (`go build ./cmd/soholink/...`).
5. Update this document with the new value.

---

## Index

| # | Axis | File | Line(s) |
|---|------|------|---------|
| 1 | [Database driver](#1-database-driver) | `internal/store/db.go` | 10 |
| 2 | [Database connection string](#2-database-connection-string) | `internal/store/db.go` | 477 |
| 3 | [Database connection pool](#3-database-connection-pool) | `internal/store/db.go` | 486–488 |
| 4 | [Database file path](#4-database-file-path) | `internal/config/config.go` | 377 |
| 5 | [Platform fee (1%)](#5-platform-fee) | `internal/central/pricing.go` | 34, 58 |
| 6 | [Coordinator fee](#6-coordinator-fee) | `internal/config/config.go` | 267 |
| 7 | [IPFS daemon endpoint](#7-ipfs-daemon-endpoint) | `internal/storage/ipfs.go` | 42 |
| 8 | [IPFS HTTP timeout](#8-ipfs-http-timeout) | `internal/storage/ipfs.go` | 47 |
| 9 | [API listen address](#9-api-listen-address) | `configs/default.yaml` | 37 |
| 10 | [Auth nonce TTL](#10-auth-nonce-ttl) | `internal/httpapi/auth.go` | 51 |
| 11 | [Auth nonce size](#11-auth-nonce-size) | `internal/httpapi/auth.go` | 47 |
| 12 | [OPA compute resource limits](#12-opa-compute-resource-limits) | `configs/policies/resource_sharing.rego` | 20–23 |
| 13 | [P2P multicast address](#13-p2p-multicast-address) | `internal/p2p/mesh.go` | 33–35 |
| 14 | [P2P announce interval](#14-p2p-announce-interval) | `internal/p2p/mesh.go` | 37 |
| 15 | [Billing interval](#15-billing-interval) | `internal/payment/meter.go` | 51–52 |

---

## 1. Database Driver

**What it controls:** Which SQL database engine SoHoLINK uses. Currently SQLite via the
`modernc.org/sqlite` pure-Go driver. Change this to swap to PostgreSQL, MySQL, etc.

**Find it:**
```bash
$ grep -n 'modernc.org/sqlite\|"sqlite"' internal/store/db.go
```

**Current value (line 10):**
```go
_ "modernc.org/sqlite"
```

**How to change — example: PostgreSQL**

1. Add the PostgreSQL driver to `go.mod`:
   ```bash
   $ go get github.com/lib/pq
   ```
2. Replace line 10 in `internal/store/db.go`:
   ```go
   // Before:
   _ "modernc.org/sqlite"
   // After:
   _ "github.com/lib/pq"
   ```
3. Update the connection string (see [§2](#2-database-connection-string)).
4. Adjust pool settings (see [§3](#3-database-connection-pool)) — PostgreSQL supports many connections; remove the `SetMaxOpenConns(1)` SQLite workaround.
5. Remove WAL-mode and busy-timeout pragmas (lines ~490–500) — PostgreSQL has its own transaction isolation.
6. Audit all migrations in `internal/store/migrate.go` for SQLite-specific syntax (e.g., `AUTOINCREMENT` → `SERIAL` in PostgreSQL).

**Restart required?** Yes — requires a rebuild and full restart.

---

## 2. Database Connection String

**What it controls:** The Data Source Name (DSN) passed to `sql.Open`. For SQLite this is a
file path. For other drivers it is a URL or DSN string.

**Find it:**
```bash
$ grep -n 'sql.Open' internal/store/db.go
```

**Current value (line 477):**
```go
db, err := sql.Open("sqlite", dbPath)
```

`dbPath` is the string returned by `cfg.DatabasePath()` (see [§4](#4-database-file-path)).

**How to change — example: PostgreSQL**
```go
// Replace line 477:
dsn := os.Getenv("DATABASE_URL")   // e.g. "postgres://user:pass@localhost:5432/soholink?sslmode=require"
db, err := sql.Open("postgres", dsn)
```

Also change the driver name from `"sqlite"` to `"postgres"` (matches the registered driver name).

**Restart required?** Yes — rebuild required.

---

## 3. Database Connection Pool

**What it controls:** Maximum open connections, idle connections, and connection lifetime.
The current values enforce a single writer (required by SQLite's single-writer model).

**Find it:**
```bash
$ grep -n 'SetMaxOpenConns\|SetMaxIdleConns\|SetConnMaxLifetime' internal/store/db.go
```

**Current values (lines 486–488):**
```go
db.SetMaxOpenConns(1)          // line 486 — cap to 1 (SQLite single-writer)
db.SetMaxIdleConns(1)          // line 487
db.SetConnMaxLifetime(0)       // line 488 — keep open indefinitely
```

**How to change — example: PostgreSQL (high concurrency)**
```go
// Lines 486–488:
db.SetMaxOpenConns(25)         // 25 concurrent connections
db.SetMaxIdleConns(10)         // keep 10 idle connections ready
db.SetConnMaxLifetime(5 * time.Minute)  // recycle connections every 5 minutes
```

**Restart required?** Yes — rebuild required.

---

## 4. Database File Path

**What it controls:** The file system path where the SQLite database is written. Derived
from `storage.base_path` in the config.

**Find it:**
```bash
$ grep -n 'DatabasePath\|soholink.db' internal/config/config.go
```

**Current value (line 377):**
```go
func (c *Config) DatabasePath() string {
    return filepath.Join(c.Storage.BasePath, "soholink.db")
}
```

**How to change:**

Option A — config file (preferred):
```yaml
# configs/default.yaml or your config.yaml:
storage:
  base_path: "/data/soholink"    # database will be at /data/soholink/soholink.db
```

Option B — environment variable:
```bash
$ export SOHOLINK_STORAGE_BASE_PATH="/data/soholink"
```

Option C — hard-code a different filename (requires code edit):
```go
// Line 377 in internal/config/config.go:
return filepath.Join(c.Storage.BasePath, "mynode.db")   // change filename
```

**Restart required?** Config change: yes (data directory changes on restart). Code edit: rebuild + restart.

---

## 5. Platform Fee

**What it controls:** The percentage of each transaction that the central SoHoLINK operator
(Network Theory Applied Research Institute) earns. Currently **1%** of the net amount
(after payment processor fees). The provider receives the remaining 99%.

**Find it:**
```bash
$ grep -n 'netAmount / 100\|centralFee' internal/central/pricing.go
```

**Current values:**
```go
// Line 34 (CalculateFees):
centralFee := netAmount / 100   // 1% via integer division

// Line 58 (CalculateFeesWithFixed):
centralFee := netAmount / 100   // 1% via integer division
```

**How to change:**

| Target fee | Replace `/ 100` with |
|-----------|----------------------|
| 0.5% | `/ 200` |
| 1% (current) | `/ 100` |
| 2% | `/ 50` |
| 5% | `/ 20` |
| 10% | `/ 10` |

For non-integer percentages, switch from integer division to floating-point:
```go
// Example: 1.5% fee
centralFee := int64(float64(netAmount) * 0.015)
```

> ⚠️ **Note:** `config.central.transaction_fee_percent` (in `configs/default.yaml`, line 95) is
> a configuration field but is **not** read by `pricing.go`. The fee is computed from the
> hardcoded divisor at lines 34 and 58. Both lines must be changed consistently.

**Restart required?** Yes — rebuild required.

---

## 6. Coordinator Fee

**What it controls:** The percentage of each brokered transaction that a federation
*coordinator* node earns when acting as a matchmaker between providers and requesters.
Separate from the platform fee (§5).

**Find it:**
```bash
$ grep -n 'FeePercent\|fee_percent' internal/config/config.go
```

**Current value (line 267):**
```go
// FeePercent is the percentage of each transaction the coordinator earns.
// Default 1.0 (1%). Set lower to attract more providers.
FeePercent float64 `mapstructure:"fee_percent"`
```

Default in `configs/default.yaml` (federation section not yet in default.yaml — add it):
```yaml
federation:
  fee_percent: 1.0    # coordinator earns 1% of facilitated transactions
```

**How to change:** Edit `fee_percent` in your config file. No code change needed.

```yaml
federation:
  fee_percent: 0.5    # lower fee to attract providers to your coordinator
```

**Restart required?** Config-only — restart to reload.

---

## 7. IPFS Daemon Endpoint

**What it controls:** The URL of the Kubo IPFS daemon's HTTP RPC API. SoHoLINK uses this
to pin uploaded files onto IPFS.

**Find it:**
```bash
$ grep -n '5001\|apiBase\|ipfs_api_addr' internal/storage/ipfs.go
```

**Current value (line 42):**
```go
apiBase = "http://127.0.0.1:5001/api/v0"
```

This fallback is used when `NewIPFSClient("")` is called (i.e., when `storage.ipfs_api_addr`
is empty in the config).

**How to change:**

Option A — config file (preferred):
```yaml
storage:
  ipfs_api_addr: "http://192.168.1.50:5001"   # remote Kubo daemon
```

Option B — environment variable:
```bash
$ export SOHOLINK_STORAGE_IPFS_API_ADDR="http://192.168.1.50:5001"
```

Option C — hard-code a different default (code edit):
```go
// Line 42 in internal/storage/ipfs.go:
apiBase = "http://192.168.1.50:5001/api/v0"
```

To **disable IPFS entirely**, set `ipfs_api_addr: ""` in the config. Uploads will be stored
locally only (no CID pinning or content-addressed retrieval).

**Restart required?** Config change: yes. Code edit: rebuild + restart.

---

## 8. IPFS HTTP Timeout

**What it controls:** How long the IPFS client waits for a single API call to the Kubo
daemon before giving up. Large file uploads need a generous timeout.

**Find it:**
```bash
$ grep -n 'time.Minute\|Timeout' internal/storage/ipfs.go
```

**Current value (line 47):**
```go
Timeout: 5 * time.Minute,   // large files can be slow
```

**How to change:**
```go
// Line 47 in internal/storage/ipfs.go:
Timeout: 30 * time.Minute,  // increase for very large files
// or:
Timeout: 60 * time.Second,  // decrease for small-file-only deployments
```

**Restart required?** Yes — rebuild required.

---

## 9. API Listen Address

**What it controls:** The network address and port the HTTP API server binds to.

**Find it:**
```bash
$ grep -n 'http_api_address\|8080' configs/default.yaml
```

**Current value (`configs/default.yaml` line 37):**
```yaml
http_api_address: "0.0.0.0:8080"
```

**How to change:**

Option A — config file:
```yaml
resource_sharing:
  http_api_address: "127.0.0.1:9090"   # localhost-only on port 9090
```

Option B — environment variable:
```bash
$ export SOHOLINK_RESOURCE_SHARING_HTTP_API_ADDRESS="0.0.0.0:443"
```

**Restart required?** Yes — restart to rebind.

---

## 10. Auth Nonce TTL

**What it controls:** How long a challenge nonce remains valid for signing. After this
window, the nonce is expired and the client must request a new challenge.

**Find it:**
```bash
$ grep -n '5 \* time.Minute\|expiry' internal/httpapi/auth.go
```

**Current value (line 51):**
```go
nonceMap[nonce] = nonceEntry{expiry: time.Now().Add(5 * time.Minute)}
```

**How to change:**
```go
// Line 51 in internal/httpapi/auth.go:
nonceMap[nonce] = nonceEntry{expiry: time.Now().Add(2 * time.Minute)}  // tighter window
// or:
nonceMap[nonce] = nonceEntry{expiry: time.Now().Add(15 * time.Minute)} // more forgiving for slow clients
```

> ℹ️ The `auth.max_nonce_age` config field (`configs/default.yaml` line 13, default `300`
> seconds) is validated separately by the auth middleware as a clock-skew guard. Keep the
> two values consistent.

**Restart required?** Yes — rebuild required.

---

## 11. Auth Nonce Size

**What it controls:** The entropy (in bytes) of the random challenge nonce. 32 bytes = 256
bits of entropy, making brute-force infeasible.

**Find it:**
```bash
$ grep -n 'make(\\[\\]byte' internal/httpapi/auth.go
```

**Current value (line 47):**
```go
b := make([]byte, 32)   // 32 bytes = 256-bit nonce
```

**How to change:**
```go
// Line 47 in internal/httpapi/auth.go:
b := make([]byte, 64)   // 512-bit nonce (more entropy, longer URL-safe string)
```

> ⚠️ Increasing nonce size increases the base64-encoded string length passed to clients.
> Ensure clients can handle longer nonce values. Decreasing below 16 bytes is not recommended.

**Restart required?** Yes — rebuild required.

---

## 12. OPA Compute Resource Limits

**What it controls:** The maximum CPU cores, RAM, disk, and timeout a single compute job
may request. Jobs exceeding any limit are denied by the OPA policy evaluator.

**Find it:**
```bash
$ grep -n 'cpu_cores\|memory_mb\|timeout_seconds\|disk_mb' configs/policies/resource_sharing.rego
```

**Current values (lines 20–23):**
```rego
job_within_limits(spec) if {
    spec.cpu_cores        <= 4      # line 20
    spec.memory_mb        <= 8192   # line 21
    spec.timeout_seconds  <= 3600   # line 22
    spec.disk_mb          <= 10240  # line 23
}
```

**How to change:**

Edit the numeric constants in `configs/policies/resource_sharing.rego`:

```rego
job_within_limits(spec) if {
    spec.cpu_cores        <= 16     # allow up to 16 cores
    spec.memory_mb        <= 65536  # allow up to 64 GB RAM
    spec.timeout_seconds  <= 86400  # allow up to 24-hour jobs
    spec.disk_mb          <= 204800 # allow up to 200 GB disk
}
```

**Restart required?** **No** — OPA re-evaluates the `.rego` file on every request. The
change takes effect immediately after saving.

> ℹ️ Mirror these changes in the GUI **Settings → Provisioning Limits** dialog and in
> `configs/default.yaml` under `resource_sharing.compute.*` so the API, OPA, and GUI stay
> consistent.

---

## 13. P2P Multicast Address

**What it controls:** The LAN multicast group used for automatic peer discovery. All
SoHoLINK nodes on the same LAN join this group and exchange signed announcements.
Change this if `239.255.42.99` conflicts with other software on your network.

**Find it:**
```bash
$ grep -n 'multicastGroup\|multicastPort\|multicastAddr' internal/p2p/mesh.go
```

**Current values (lines 33–35):**
```go
multicastGroup = "239.255.42.99"       // line 33
multicastPort  = 7946                  // line 34
multicastAddr  = "239.255.42.99:7946"  // line 35
```

**How to change:**

Edit all three constants in `internal/p2p/mesh.go`:
```go
multicastGroup = "239.255.100.1"        // any unused address in 239.0.0.0/8 (RFC 2365)
multicastPort  = 9000                   // any unused UDP port
multicastAddr  = "239.255.100.1:9000"   // keep consistent with the two above
```

> ⚠️ **All nodes on the network must use the same multicast address and port.** A mismatch
> silently partitions the mesh — nodes will not discover each other.
>
> Also update the log message in `internal/app/app.go` line 476 to match:
> ```go
> log.Printf("[app] small-world mesh initialized (multicast 239.255.100.1:9000)")
> ```

**Restart required?** Yes — rebuild required.

---

## 14. P2P Announce Interval

**What it controls:** How frequently each node broadcasts its signed announcement to peers.
Shorter intervals mean faster discovery but more UDP traffic.

**Find it:**
```bash
$ grep -n 'announceInterval\|10 \* time.Second' internal/p2p/mesh.go
```

**Current value (line 37):**
```go
announceInterval = 10 * time.Second
```

**How to change:**
```go
// Line 37 in internal/p2p/mesh.go:
announceInterval = 30 * time.Second   // reduce traffic on large LANs
// or:
announceInterval = 5 * time.Second    // faster convergence on small LANs
```

**Restart required?** Yes — rebuild required.

---

## 15. Billing Interval

**What it controls:** How often the usage meter runs and charges tenants for active
workload placements. The meter bills in arrears.

**Find it:**
```bash
$ grep -n 'BillingInterval\|MinBillableSeconds\|time.Hour' internal/payment/meter.go
```

**Current values (lines 51–52):**
```go
BillingInterval:    time.Hour,   // line 51 — charge every 1 hour
MinBillableSeconds: 60,          // line 52 — minimum charge = 1 minute of usage
```

**How to change:**
```go
// Lines 51–52 in internal/payment/meter.go:
BillingInterval:    15 * time.Minute,  // more frequent billing (useful for testing)
MinBillableSeconds: 60,                // keep minimum at 60 s (1 minute) for fairness
```

> ⚠️ Shorter billing intervals increase database write frequency. At 1-minute intervals on
> a node with 100 active placements, the meter writes 100 rows per minute to the ledger.
> Use `time.Hour` (default) for production.

**Restart required?** Yes — rebuild required (or pass a custom `MeterConfig` to `NewUsageMeter`).

---

## Quick Reference Table

| # | File | Lines | Current value | Config alternative |
|---|------|-------|---------------|-------------------|
| 1 | `internal/store/db.go` | 10 | `_ "modernc.org/sqlite"` | — |
| 2 | `internal/store/db.go` | 477 | `sql.Open("sqlite", dbPath)` | — |
| 3 | `internal/store/db.go` | 486–488 | MaxOpen=1, MaxIdle=1 | — |
| 4 | `internal/config/config.go` | 377 | `<base_path>/soholink.db` | `storage.base_path` |
| 5 | `internal/central/pricing.go` | 34, 58 | `netAmount / 100` | — |
| 6 | `internal/config/config.go` | 267 | `fee_percent float64` | `federation.fee_percent` |
| 7 | `internal/storage/ipfs.go` | 42 | `http://127.0.0.1:5001/api/v0` | `storage.ipfs_api_addr` |
| 8 | `internal/storage/ipfs.go` | 47 | `5 * time.Minute` | — |
| 9 | `configs/default.yaml` | 37 | `0.0.0.0:8080` | `resource_sharing.http_api_address` |
| 10 | `internal/httpapi/auth.go` | 51 | `5 * time.Minute` | `auth.max_nonce_age` (partial) |
| 11 | `internal/httpapi/auth.go` | 47 | `32` bytes | — |
| 12 | `configs/policies/resource_sharing.rego` | 20–23 | cpu=4, mem=8192, disk=10240, t=3600 | (edit `.rego` directly) |
| 13 | `internal/p2p/mesh.go` | 33–35 | `239.255.42.99:7946` | — |
| 14 | `internal/p2p/mesh.go` | 37 | `10 * time.Second` | — |
| 15 | `internal/payment/meter.go` | 51–52 | `time.Hour`, min 60s | — |

---

## Verification Commands

After making any change, run these to confirm the correct line now contains your new value:

```bash
# §1 Database driver:
$ sed -n '10p' internal/store/db.go

# §2 Database open call:
$ sed -n '477p' internal/store/db.go

# §5 Platform fee:
$ sed -n '34p' internal/central/pricing.go

# §7 IPFS endpoint:
$ sed -n '42p' internal/storage/ipfs.go

# §10 Auth nonce TTL:
$ sed -n '51p' internal/httpapi/auth.go

# §11 Auth nonce size:
$ sed -n '47p' internal/httpapi/auth.go

# §12 OPA CPU limit:
$ sed -n '20p' configs/policies/resource_sharing.rego

# §13 Multicast address:
$ sed -n '33p' internal/p2p/mesh.go

# §14 Announce interval:
$ sed -n '37p' internal/p2p/mesh.go

# §15 Billing interval:
$ sed -n '51p' internal/payment/meter.go
```

> **Line numbers shift** as the codebase evolves. If `sed -n 'Np'` does not print the
> expected value, re-run the **Find it** `grep` command for that section to locate the
> current line number.

---

*This document is provided for development guidance and does not constitute legal or financial advice.*
