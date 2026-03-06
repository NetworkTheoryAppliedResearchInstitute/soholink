# SoHoLINK FlutterFlow App — Build Instructions for Claude in Browser

**Goal:** Build the SoHoLINK companion app in FlutterFlow at https://app.flutterflow.io.
The Go backend (`fedaaa`) runs at a configurable base URL (default: `http://localhost:8080`) and
exposes a pure JSON REST API — no Firebase, no third-party auth. This document is a complete,
step-by-step instruction set for Claude operating the FlutterFlow web editor in a browser.

---

## 0. Prerequisites

- You are logged in to FlutterFlow at https://app.flutterflow.io
- A project named **SoHoLINK** exists (create it if not; choose "Blank App", Material 3 theme)
- Target platforms: **Android** and **iOS** (mobile-first; desktop/web as secondary)
- The Go backend is running at a known base URL; store it in App State as `base_url`

---

## 1. App State Variables

Navigate to **App State** (left sidebar → State Management → App State).
Create the following variables:

| Name | Type | Persisted | Initial Value | Purpose |
|---|---|---|---|---|
| `base_url` | String | ✅ Yes | `http://localhost:8080` | Go node API base URL |
| `node_did` | String | ✅ Yes | `` | This node's DID |
| `is_configured` | Boolean | ✅ Yes | `false` | Has the user set a base_url? |

---

## 2. Custom Data Types

Navigate to **Data Types** (left sidebar → Data Types). Create these types:

### 2a. `NodeStatus`
Fields:
- `uptime_seconds` → Integer
- `os` → String
- `active_rentals` → Integer
- `federation_nodes` → Integer
- `mobile_nodes` → Integer
- `earned_sats_today` → Integer
- `cpu_offered_pct` → Integer
- `cpu_used_pct` → Double
- `ram_offered_gb` → Double
- `ram_used_pct` → Double
- `storage_offered_gb` → Double
- `storage_used_pct` → Double
- `net_offered_mbps` → Integer
- `net_used_pct` → Double

### 2b. `PeerInfo`
Fields:
- `did` → String
- `api_addr` → String
- `ipfs_addr` → String
- `cpu_cores` → Double
- `ram_gb` → Double
- `disk_gb` → Integer
- `gpu` → String
- `region` → String
- `last_seen` → String

### 2c. `PeerList`
Fields:
- `count` → Integer
- `peers` → List of `PeerInfo`

### 2d. `RevenueStats`
(Match the JSON from `GET /api/revenue/stats` — create after testing that endpoint)
Fields:
- `total_revenue` → Integer
- `pending_payout` → Integer
- `revenue_today` → Integer

### 2e. `ActiveRental`
(Match `GET /api/revenue/active-rentals` response items)
Fields:
- `rental_id` → String
- `tenant_did` → String
- `started_at` → String
- `resource_type` → String
- `price_per_hour` → Integer

### 2f. `WorkloadStatus`
Fields:
- `workload_id` → String
- `status` → String
- `replicas` → Integer
- `created_at` → String
- `updated_at` → String

---

## 3. API Groups and Calls

Navigate to **API Calls** (left sidebar → API Calls icon). Create one API group:

### API Group: `SoHoLINK`
- **Base URL**: `[base_url]`
  (In FlutterFlow, reference the App State variable using bracket notation)
- **Default Headers**: Add `Content-Type: application/json`

Create the following API calls inside this group:

---

#### 3a. `GetHealth`
- **Method**: GET
- **URL suffix**: `/api/health`
- **Response Type**: JSON
- **Test**: Click **Test** → expect `{"status":"ok","time":"..."}`

#### 3b. `GetStatus`
- **Method**: GET
- **URL suffix**: `/api/status`
- **Response Type**: JSON → **Parse as Data Type** → `NodeStatus`
- **Test**: Run and confirm all fields parse correctly

#### 3c. `GetPeers`
- **Method**: GET
- **URL suffix**: `/api/peers`
- **Response Type**: JSON → **Parse as Data Type** → `PeerList`
- **Test**: Run; if no peers on LAN the `peers` array will be empty — that is correct

#### 3d. `GetRevenueFederation`
- **Method**: GET
- **URL suffix**: `/api/revenue/federation`
- **Response Type**: JSON

#### 3e. `GetRevenueBalance`
- **Method**: GET
- **URL suffix**: `/api/revenue/balance`

#### 3f. `GetRevenueStats`
- **Method**: GET
- **URL suffix**: `/api/revenue/stats`

#### 3g. `GetActiveRentals`
- **Method**: GET
- **URL suffix**: `/api/revenue/active-rentals`
- **Response Type**: JSON

#### 3h. `RequestPayout`
- **Method**: POST
- **URL suffix**: `/api/revenue/request-payout`
- **Body Type**: JSON
- **Body**: `{}`

#### 3i. `GetWorkloads`
- **Method**: GET
- **URL suffix**: `/api/workloads`

#### 3j. `SubmitWorkload`
- **Method**: POST
- **URL suffix**: `/api/workloads/submit`
- **Body Type**: JSON
- **Variables**: `workload_id` (String), `replicas` (Integer), `image` (String), `cpu_millicores` (Integer), `memory_mb` (Integer)
- **Body**:
  ```json
  {
    "workload_id": "[workload_id]",
    "replicas": [replicas],
    "spec": {
      "image": "[image]",
      "cpu_millicores": [cpu_millicores],
      "memory_mb": [memory_mb]
    }
  }
  ```

#### 3k. `GetLBTASScore`
- **Method**: GET
- **URL suffix**: `/api/lbtas/score/[did]`
- **Variables**: `did` (String)

#### 3l. `GetGovernanceProposals`
- **Method**: GET
- **URL suffix**: `/api/governance/proposals`

#### 3m. `GetMobileNodes`
- **Method**: GET
- **URL suffix**: `/api/v1/nodes/mobile`

---

## 4. Pages to Create

Create these pages using the **Pages** section (left sidebar → Pages → + Add Page):

| Page Name | Route | Purpose |
|---|---|---|
| `SetupPage` | `/setup` | First-run: enter node base_url |
| `DashboardPage` | `/dashboard` | Live node status, radial stats |
| `PeersPage` | `/peers` | LAN-discovered peer list |
| `RevenuePage` | `/revenue` | Earnings, active rentals, payout |
| `WorkloadsPage` | `/workloads` | Running workloads, submit new |
| `GovernancePage` | `/governance` | Proposals, voting |
| `SettingsPage` | `/settings` | Change base_url, node DID |

Set up a **Bottom Navigation Bar** component (or use a Scaffold with BottomNavigationBar) with
tabs for: Dashboard, Peers, Revenue, Workloads, Governance.

**Initial page logic**: In the app's entry point (or `main.dart` equivalent), add a Conditional
Navigation action: if `is_configured == false`, navigate to `SetupPage`; else navigate to
`DashboardPage`.

---

## 5. Page-by-Page Build Instructions

### 5a. SetupPage

**Purpose**: Let the user enter the IP/hostname of their SoHoLINK node.

**Widgets**:
- `Column` (centered)
  - `Image` — SoHoLINK logo
  - `Text` — "Connect to your SoHoLINK node"
  - `TextField` (bound to a Page State variable `input_url`, placeholder: `http://192.168.1.x:8080`)
  - `ElevatedButton` — "Connect"

**"Connect" button Action Flow**:
1. Call `GetHealth` API (variable: use Page State `input_url` for `base_url`)
   Note: override the group `base_url` variable with `input_url` for this call
2. **If** API response status == 200 **and** `response.status == "ok"`:
   - Update App State: `base_url` = `input_url`, `is_configured` = `true`
   - Navigate to `DashboardPage`
3. **Else**:
   - Show Snackbar: "Cannot reach node at [input_url]. Check the address and try again."

---

### 5b. DashboardPage

**Purpose**: Live overview of the node — uptime, resources, federation count, earnings.

**On Page Load**: Add a **Backend Query** → API Call → `GetStatus`; store response in Page State
variable `status` (type: `NodeStatus`).

**Widgets**:
- `Column`
  - `Text` — "SoHoLINK Node" (title)
  - `Text` — "Uptime: [status.uptime_seconds ÷ 3600]h" (use a custom function to format)
  - `Text` — "OS: [status.os]"
  - `Row` of stat cards (use `Container` + `Column` for each card):
    - **Federation Peers**: `[status.federation_nodes]`
    - **Active Rentals**: `[status.active_rentals]`
    - **Earned Today**: `[status.earned_sats_today] sats`
    - **Mobile Nodes**: `[status.mobile_nodes]`
  - `Divider`
  - `Text` — "Resource Utilization" (subtitle)
  - `LinearProgressIndicator` — CPU Used: value = `status.cpu_used_pct / 100`
  - `LinearProgressIndicator` — RAM Used: value = `status.ram_used_pct / 100`
  - `LinearProgressIndicator` — Storage Used: value = `status.storage_used_pct / 100`
  - `ElevatedButton` — "Refresh" → trigger `GetStatus` again and update Page State

**Auto-refresh**: Add a `Timer` custom action (see §6) that calls `GetStatus` every 10 seconds.

---

### 5c. PeersPage

**Purpose**: Show LAN-discovered peers from the P2P mesh.

**On Page Load**: Backend Query → `GetPeers` → store in Page State `peer_list` (type: `PeerList`).

**Widgets**:
- `Column`
  - `Text` — "LAN Peers ([peer_list.count])"
  - `ListView` → Generate Children from Variable → `peer_list.peers`
    - **Item Widget** (a `Card`):
      - `Text` — DID: `[item.did]` (truncate to first 24 chars via custom function)
      - `Text` — API: `[item.api_addr]`
      - `Text` — CPU: `[item.cpu_cores]` cores  |  RAM: `[item.ram_gb]` GB  |  Disk: `[item.disk_gb]` GB
      - `Text` (conditional) — GPU: `[item.gpu]` (only show if gpu is not empty)
      - `Text` (conditional) — Region: `[item.region]`
  - `ElevatedButton` — "Refresh"

If `peer_list.count == 0`, show a `Text` — "No peers discovered yet. Peers on the same LAN will
appear here automatically within 10 seconds of joining."

---

### 5d. RevenuePage

**Purpose**: Earnings overview, active rentals, payout trigger.

**On Page Load**: Backend Query → `GetRevenueFederation` → store result in Page State `revenue`.

**Widgets**:
- `Column`
  - **Summary Row** (3 stat cards):
    - Total Revenue: `[revenue.total_revenue]` sats
    - Pending Payout: `[revenue.pending_payout]` sats
    - Earned Today: `[revenue.revenue_today]` sats
  - `Divider`
  - `Text` — "Active Rentals"
  - `ListView` → `revenue.active_rentals` list
    - **Item Widget**:
      - `Text` — Tenant: `[item.tenant_did]` (truncated)
      - `Text` — Resource: `[item.resource_type]` | Rate: `[item.price_per_hour]` sats/hr
      - `Text` — Started: `[item.started_at]`
  - `Divider`
  - `ElevatedButton` — "Request Payout"
    - Action: Call `RequestPayout` API → on success show Snackbar "Payout requested successfully"

---

### 5e. WorkloadsPage

**Purpose**: View running workloads; submit new ones.

**On Page Load**: Backend Query → `GetWorkloads`.

**Widgets**:
- `Column`
  - `Text` — "Active Workloads"
  - `ListView` of workload cards:
    - `Text` — ID: `[item.workload_id]`
    - `Text` — Status: `[item.status]` | Replicas: `[item.replicas]`
  - `Divider`
  - `Text` — "Submit Workload"
  - `TextField` — Workload ID (Page State: `wl_id`)
  - `TextField` — Container Image (Page State: `wl_image`)
  - `Row`:
    - `TextField` — Replicas (Page State: `wl_replicas`, Integer)
    - `TextField` — CPU (millicores, Page State: `wl_cpu`)
    - `TextField` — RAM (MB, Page State: `wl_ram`)
  - `ElevatedButton` — "Submit"
    - Action: Call `SubmitWorkload` with Page State variables → refresh list on success

---

### 5f. GovernancePage

**Purpose**: View governance proposals.

**On Page Load**: Backend Query → `GetGovernanceProposals`.

**Widgets**:
- `ListView` of proposal cards:
  - `Text` — Title, Type, Status
  - `Text` — Proposer DID (truncated)
  - `Text` — Voting period
  - `ElevatedButton` — "View Details" → navigate to sub-page with full detail + vote buttons

---

### 5g. SettingsPage

**Purpose**: Change node URL, view current node DID.

**Widgets**:
- `TextField` — Base URL (pre-filled with `base_url` App State)
- `ElevatedButton` — "Save & Reconnect" → validate with `GetHealth` → update App State
- `Text` — "Node DID: [node_did]" (read-only)
- `ElevatedButton` — "Disconnect" → set `is_configured = false` → navigate to SetupPage

---

## 6. Custom Actions (Dart code)

Navigate to **Custom Code** → **Custom Actions**. Create these:

### 6a. `formatUptime`
**Type**: Custom Function (returns String)
**Parameters**: `seconds` (Integer)
**Purpose**: Convert uptime_seconds to human-readable "Xd Xh Xm"
```dart
String formatUptime(int seconds) {
  final d = seconds ~/ 86400;
  final h = (seconds % 86400) ~/ 3600;
  final m = (seconds % 3600) ~/ 60;
  if (d > 0) return '${d}d ${h}h';
  if (h > 0) return '${h}h ${m}m';
  return '${m}m';
}
```

### 6b. `truncateDID`
**Type**: Custom Function (returns String)
**Parameters**: `did` (String)
**Purpose**: Shorten a long DID for display
```dart
String truncateDID(String did) {
  if (did.length <= 20) return did;
  return '${did.substring(0, 12)}…${did.substring(did.length - 8)}';
}
```

### 6c. `satsToDisplay`
**Type**: Custom Function (returns String)
**Parameters**: `sats` (Integer)
**Purpose**: Format satoshi amounts
```dart
String satsToDisplay(int sats) {
  if (sats >= 100000000) return '${(sats / 100000000).toStringAsFixed(4)} BTC';
  if (sats >= 1000) return '${(sats / 1000).toStringAsFixed(1)}k sats';
  return '$sats sats';
}
```

### 6d. `startPolling` (Custom Action — async)
**Purpose**: Trigger a periodic refresh of the DashboardPage status.
```dart
// NOTE: FlutterFlow does not natively support background timers.
// Use this action on a Button ("Start Live Updates") to manually
// trigger a single refresh cycle. For automatic polling, use a
// PageView with a Timer widget (set interval to 10000ms) available
// in FlutterFlow's built-in widget palette under "Advanced".
```

---

## 7. Theme and Styling

Navigate to **Theme** (left sidebar → Theme Settings):

- **Color Scheme**: Dark mode primary
  - Primary: `#00E5FF` (cyan — matches existing SoHoLINK branding)
  - Background: `#0D1117` (near-black)
  - Surface: `#161B22` (dark card)
  - On-Surface: `#C9D1D9` (light gray text)
  - Error: `#FF6B6B`
- **Typography**: Use `Inter` (or `Roboto` as fallback) for body text; `JetBrains Mono` for DID/hash display fields
- **Card style**: Rounded corners (12px), border `#30363D`, background `Surface`

---

## 8. Navigation Structure

Set up a **Scaffold** with a **BottomNavigationBar** as the app shell:

Tabs (in order):
1. 📊 Dashboard → `DashboardPage`
2. 🌐 Peers → `PeersPage`
3. 💰 Revenue → `RevenuePage`
4. ⚙️ Workloads → `WorkloadsPage`
5. 🗳 Governance → `GovernancePage`

Settings accessible via the AppBar actions button (gear icon) → navigate to `SettingsPage`.

---

## 9. Build & Export

When all pages are built and tested in the FlutterFlow preview:

1. **Test in Preview**: Use the Run button (▶) to test on-device using FlutterFlow's test app
2. **Generate Code**: Settings → Code → Download Code (Flutter project ZIP)
3. **Integrate**: Add the downloaded Flutter project as `mobile/flutter-app/` in the SoHoLINK repo
4. **Build APK**: `flutter build apk --release` inside the downloaded project
5. **Build iOS**: `flutter build ios --release` (requires Xcode on macOS)

---

## 10. Backend API Reference Summary

The Go node exposes these endpoints at `[base_url]`:

| Method | Endpoint | Returns | Use |
|---|---|---|---|
| GET | `/api/health` | `{status, time}` | Connectivity check |
| GET | `/api/status` | `NodeStatus` | Dashboard dials |
| GET | `/api/peers` | `{count, peers[]}` | P2P mesh peers |
| GET | `/api/revenue/federation` | Revenue summary | Revenue screen |
| GET | `/api/revenue/balance` | Balance | Revenue screen |
| GET | `/api/revenue/active-rentals` | Rental list | Revenue screen |
| POST | `/api/revenue/request-payout` | `{status}` | Payout button |
| GET | `/api/workloads` | Workload list | Workloads screen |
| POST | `/api/workloads/submit` | `{workload_id, status}` | Submit form |
| GET | `/api/lbtas/score/{did}` | Score | Peer reputation |
| POST | `/api/lbtas/rate-provider` | `{status}` | Rate a peer |
| GET | `/api/governance/proposals` | Proposals list | Governance screen |
| POST | `/api/governance/vote` | `{status, vote_id}` | Cast vote |
| GET | `/api/v1/nodes/mobile` | Mobile node list | Peers screen |
| GET | `/api/resources/discover` | Resource list | Future: discovery |

All endpoints accept/return `application/json`. No authentication token is required by default
(the node is sovereign and runs on the provider's own hardware/LAN).

---

## 11. Notes for Claude in Browser

- The FlutterFlow editor URL is: https://app.flutterflow.io
- Always **Save** (Ctrl+S / Cmd+S) after each major step
- Use the **Preview** button frequently to verify the layout
- When creating API calls, always run the **Test** to confirm the endpoint responds before wiring it to a widget
- If an API test fails, check that the Go node is running: `fedaaa start` in a terminal
- The globe visualization (`ui/globe-interface/ntarios-globe.html`) is a standalone HTML file served separately — it is NOT part of the FlutterFlow app. It can be opened in a WebView widget if desired, pointing to `[base_url]/globe` (add a static file serve route to the Go backend for this)
- All dollar amounts in the Go backend are stored as **satoshis** (integer), not dollars or floats. Use `satsToDisplay()` for all monetary fields

---

*Generated: 2026-03-04 | SoHoLINK v0.2.x | Backend: Go `fedaaa` binary listening on port 8080*
