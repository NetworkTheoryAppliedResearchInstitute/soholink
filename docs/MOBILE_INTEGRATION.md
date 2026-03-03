# Mobile Integration Plan

**Project:** SoHoLINK — Federated SOHO Compute Marketplace
**Date:** 2026-03-02
**Research basis:** [`docs/research/MOBILE_PARTICIPATION.md`](research/MOBILE_PARTICIPATION.md)

This document is the actionable engineering plan derived from the mobile participation research. It is organized into four sequential phases, each independently deployable and useful, with later phases building on earlier ones.

---

## Summary of Approach

| Platform | Role | Priority |
|---|---|---|
| **Android TV / Fire TV** | Full compute node (always-on, no battery) | Phase 1 |
| **Android smartphone** | Short-burst compute + storage relay while charging | Phase 2 |
| **iOS** | Monitoring, earnings, job management client | Phase 3 |
| **iOS (Core ML)** | On-device Neural Engine inference endpoint | Phase 4 |

**Core design principles driving every phase:**
1. **Explicit consent only** — compute participation never activates silently; Google Play and App Store policies require this and user trust demands it
2. **Pull-based networking** — mobile nodes initiate outbound connections; inbound connections are never assumed (CGNAT)
3. **Micro-segment tasks** — tasks ≤120 seconds with disk checkpointing; mobile nodes can vanish mid-task
4. **WebAssembly portability** — Wasm is the universal task container across ARM64 mobile and x86 desktop
5. **Lightning micropayments** — sub-cent per-task earnings are only rational via Lightning, not Stripe

---

## Phase 1 — Android TV / Fire TV (Always-On Headless Nodes)

**Target:** Android TV boxes (Google TV, Nvidia Shield) and Amazon Fire TV sticks/cubes.

**Why first:** These devices eliminate the two hardest mobile constraints (battery + thermal throttling) while running the same Android OS stack. They are always plugged in, always on WiFi, and run 24/7 without duty-cycle limits. An Android TV node with modest CPU running continuously outperforms a flagship phone subject to overnight-only participation.

**Estimated effort:** 3–4 weeks

### 1.1 FedScheduler: Add `android-tv` Node Class

**File:** `internal/orchestration/scheduler.go`

- Add `NodeClassAndroidTV = "android-tv"` to the node class enum
- Add capability tags: `arch: arm64`, `mobile: false` (no replication penalty), `always-on: true`
- No change to scheduling policy — Android TV nodes behave identically to desktop nodes except for architecture

**File:** `configs/policies/resource_sharing.rego`

```rego
# Android TV treated as full node (no replication requirement)
task_replication_factor[node_class] = factor {
    node_class := input.node.class
    factor := {"mobile-android": 2, "android-tv": 1, "desktop": 1}[node_class]
}
```

### 1.2 Android TV Application

**New project:** `mobile/android-tv/` (separate Android Studio project, or Kotlin Multiplatform)

**Architecture:** Standard Android background service — no foreground service notification required on TV (leanback UI paradigm allows background operation).

```
SoHoLINK TV App
├── Main activity: earnings dashboard (TV-optimized leanback UI)
├── BackgroundWorker: WorkManager task that runs continuously
│   ├── WebSocket connection to coordinator
│   ├── Task pull loop
│   ├── Wasm executor (via wasmer-android or custom JNI bridge)
│   └── Result push + payment receipt
└── Settings activity: coordinator URL, max CPU%, wallet address
```

**Task execution:** Wasm tasks via a JNI bridge to `wasmer` or `wasmtime`. The Wasm module is architecture-agnostic; the JNI layer handles ARM64 execution context.

**Payment:** Lightning custodial wallet (LDK or similar). Auto-withdrawal to user's external Lightning wallet when balance exceeds configurable threshold.

### 1.3 Coordinator Changes (Go)

**File:** `internal/httpapi/server.go`

- Add `/api/v1/nodes/register` endpoint accepting mobile node registration (class, capabilities, WebSocket address for reverse signaling)
- Add WebSocket hub for mobile node connections (`/ws/nodes`) — coordinator pushes task descriptors to connected mobile nodes
- Task state machine: add `MOBILE_PENDING` state for tasks awaiting pull by mobile node

**File:** `internal/orchestration/scheduler.go`

- `ScheduleMobile(workload)`: routes task to connected mobile node WebSocket clients based on capability match
- Heartbeat tracking: mobile nodes send `{"type":"heartbeat"}` every 30 seconds; coordinator marks node unavailable after 2 missed heartbeats

### 1.4 Acceptance Criteria

- [ ] Android TV app installs from APK sideload and connects to a local SoHoLINK coordinator
- [ ] Node appears in dashboard Orchestration tab with class `android-tv`
- [ ] A test Wasm task executes successfully and result is recorded
- [ ] Lightning payment credited to TV app wallet after task completion
- [ ] Node disappears from active list within 90 seconds of network disconnect

---

## Phase 2 — Android Smartphone ("Earn While Charging")

**Target:** Android phones and tablets running Android 10+ (API 29+).

**Design gate:** Compute participation activates **only** when ALL of the following are true:
1. Device is connected to mains power (`ACTION_POWER_CONNECTED`)
2. Device is on WiFi (`ConnectivityManager`)
3. User has explicitly enabled "Earn While Charging" in app settings
4. Thermal headroom is adequate (`getThermalHeadroom() > 0.2`)

**Estimated effort:** 6–8 weeks

### 2.1 FedScheduler: Add `mobile-android` Node Class

**File:** `internal/orchestration/scheduler.go`

```go
type NodeConstraints struct {
    MaxTaskDurationSeconds int    `json:"max-task-duration-seconds"`
    RequiresPluggedIn      bool   `json:"requires-plugged-in"`
    WifiOnly               bool   `json:"wifi-only"`
    Arch                   string `json:"arch"`
    Mobile                 bool   `json:"mobile"`
}
```

- Scheduler filters: never assign tasks with `estimatedDuration > node.MaxTaskDurationSeconds`
- Scheduler assigns: always attach checkpoint metadata to tasks assigned to mobile nodes
- Preemption: if mobile node goes silent mid-task, reassign to a desktop node from last checkpoint

### 2.2 Task Micro-Segmentation

**File:** `internal/orchestration/workload.go`

Add `Checkpoint` field to `WorkloadState`:

```go
type WorkloadState struct {
    // ... existing fields ...
    CheckpointData []byte    `json:"checkpoint_data,omitempty"` // serialized state between segments
    SegmentIndex   int       `json:"segment_index"`              // which 120s segment we're on
    SegmentCount   int       `json:"segment_count"`              // total segments
}
```

Tasks must declare `SegmentDurationSeconds` in their spec. The coordinator sends one segment at a time to mobile nodes; the next segment is sent only after the previous segment's result is received.

### 2.3 WebAssembly Task Executor

**New package:** `internal/wasm/`

```go
// Executor runs a Wasm task module with given input bytes
// Returns result bytes or error; respects context cancellation for timeout
type Executor interface {
    Execute(ctx context.Context, module []byte, input []byte) ([]byte, error)
}
```

The Go-side executor wraps `github.com/wasmerio/wasmer-go` or `github.com/tetratelabs/wazero` (pure Go, no CGO). The same package is used on both the Go coordinator and the Android client (via gomobile or a REST shim).

### 2.4 Android Smartphone Application

**New project:** `mobile/android/`

**Key components:**

```
SoHoLINK Android App
│
├── EarnWhileChargingService (ForegroundService)
│   ├── Persistent notification: "SoHoLINK: earning 0.0042 SATS — tap to pause"
│   ├── PowerManager.getThermalHeadroom() polling (every 60s)
│   ├── WebSocket → coordinator
│   ├── Task pull + Wasm executor
│   └── Result push + payment receipt
│
├── PowerConnectionReceiver (BroadcastReceiver)
│   ├── ACTION_POWER_CONNECTED  → start EarnWhileChargingService
│   └── ACTION_POWER_DISCONNECTED → stop EarnWhileChargingService
│
├── NetworkChangeReceiver (ConnectivityManager.NetworkCallback)
│   └── WiFi lost → pause task intake (don't stop service, just drain queue)
│
├── WorkManager (ScheduledTask)
│   └── Polls coordinator for task availability when service not active
│
└── UI
    ├── MainActivity: earnings dashboard, on/off toggle
    ├── WalletFragment: Lightning balance + withdrawal
    └── SettingsFragment: coordinator URL, max CPU%, thermal threshold
```

**Battery optimization exemption flow:**
```
First enable → app shows explanation screen:
  "To earn while your phone charges overnight, SoHoLINK needs to be
   excluded from battery optimization. Tap 'Allow' to go to Settings."

  → Opens ACTION_REQUEST_IGNORE_BATTERY_OPTIMIZATIONS
  → User grants manually (required — Play policy prevents direct prompt)
```

**Thermal governor:**

```kotlin
private fun adjustThermalLoad() {
    val headroom = powerManager.getThermalHeadroom(30) // predict 30s ahead
    taskConcurrency = when {
        headroom >= 0.7 -> maxConcurrency        // full speed
        headroom >= 0.5 -> maxConcurrency / 2    // half speed
        headroom >= 0.2 -> 1                     // single task only
        else -> { pauseTaskIntake(); 0 }         // too hot — stop
    }
}
```

### 2.5 Result Verification (Replication)

**File:** `internal/orchestration/scheduler.go`

When a task is assigned to a `mobile-android` node, the scheduler simultaneously assigns an identical task to a desktop node (the "shadow replica"). Both results are compared before payment releases.

```go
func (s *FedScheduler) assignWithReplication(workload *Workload, primaryNode *Node) {
    // Assign to mobile primary
    s.assign(workload, primaryNode)

    // Find a desktop shadow node
    shadowNode := s.findNode(NodeConstraints{Mobile: false})
    if shadowNode != nil {
        shadowWorkload := workload.Clone()
        shadowWorkload.WorkloadID = workload.WorkloadID + "-shadow"
        s.assign(shadowWorkload, shadowNode)
    }
}
```

Results are compared by hash. Lightning hold invoices to the mobile node are released only after hash match.

### 2.6 Acceptance Criteria

- [ ] App installs from Google Play (or APK)
- [ ] "Earn While Charging" toggle correctly gates on all four conditions
- [ ] Foreground notification visible and dismissable
- [ ] Thermal governor reduces concurrency when device gets warm
- [ ] Service stops within 5 seconds of unplugging
- [ ] A 60-second Wasm task completes and payment is received
- [ ] If node vanishes mid-task, coordinator reassigns to desktop node from last checkpoint
- [ ] App passes Google Play policy review (background compute disclosed in listing)

---

## Phase 3 — iOS Management Client

**Target:** iPhone and iPad (iOS 16+).

**Scope:** Monitoring, earnings, job approval, and node configuration. **No compute participation** — iOS background processing restrictions make this structurally impossible (see research report).

**Estimated effort:** 4–5 weeks

### 3.1 iOS Application

**New project:** `mobile/ios/` (Swift / SwiftUI)

**Screens:**

```
SoHoLINK iOS App
│
├── DashboardView
│   ├── Earnings summary (today / week / all-time)
│   ├── Active rental count
│   ├── Node health status
│   └── Recent transactions
│
├── GlobeView
│   └── WKWebView embedding ntarios-globe.html
│       (WebSocket to coordinator for live data)
│
├── JobsView
│   ├── Pending job requests awaiting manual approval
│   └── Active jobs with progress
│
├── WalletView
│   ├── Lightning balance
│   └── Withdraw to external wallet
│
└── SettingsView
    ├── Coordinator URL
    ├── Policy configuration (max CPU%, wifi-only, etc.)
    └── Pricing adjustments
```

**Push notifications:** Coordinator sends APNs push when:
- New job request arrives (requires manual approval)
- Earnings milestone reached
- Node goes offline unexpectedly
- Payment received

**No compute, no background processing.** The app is exclusively a management interface.

### 3.2 Coordinator: APNs Integration

**New file:** `internal/notification/apns.go`

```go
type APNSNotifier struct {
    client    *http.Client
    teamID    string
    keyID     string
    bundleID  string
    privateKey crypto.PrivateKey
}

func (n *APNSNotifier) SendJobRequest(deviceToken string, jobID string, amount float64) error
func (n *APNSNotifier) SendPaymentReceived(deviceToken string, amount float64) error
func (n *APNSNotifier) SendNodeOffline(deviceToken string, nodeID string) error
```

### 3.3 Acceptance Criteria

- [ ] App installs from TestFlight (internal) or App Store
- [ ] Earnings dashboard shows real-time data from coordinator
- [ ] Globe visualization renders correctly in WKWebView
- [ ] Push notification received when a new job request arrives
- [ ] Job can be approved or rejected from the iOS app
- [ ] Lightning withdrawal initiates correctly

---

## Phase 4 — iOS Core ML Inference Endpoint

**Target:** iPhone 15 Pro, iPhone 16 Pro (devices with high-TOPS Neural Engine).

**Scope:** iOS devices can serve as ML inference endpoints for specific model classes while the app is in the foreground. This is the one compute role iOS permits.

**Estimated effort:** 3–4 weeks (after Phase 3)

### 4.1 Inference Task Type

**New task class:** `TaskTypeInference`

```go
const TaskTypeInference TaskType = "inference"

type InferenceTaskSpec struct {
    ModelCID     string   `json:"model_cid"`     // IPFS CID of Core ML .mlmodelc
    InputCID     string   `json:"input_cid"`     // IPFS CID of input tensor
    MaxTokens    int      `json:"max_tokens"`    // for generative models
    Quantization string   `json:"quantization"`  // "int4", "int8", "float16"
}
```

### 4.2 iOS Inference Execution

```swift
// In SoHoLINK iOS app — only runs while app is foreground
class InferenceEngine {
    func execute(spec: InferenceTaskSpec) async throws -> Data {
        // 1. Fetch model from IPFS (if not cached)
        let modelURL = try await ipfsClient.fetch(cid: spec.modelCID)

        // 2. Load into Core ML
        let model = try MLModel(contentsOf: modelURL)

        // 3. Run inference on Neural Engine
        let input = try await ipfsClient.fetch(cid: spec.inputCID)
        let prediction = try model.prediction(from: MLDictionaryFeatureProvider(...))

        // 4. Return result bytes
        return try prediction.encode()
    }
}
```

**Capability advertisement:**

```json
{
  "node_class": "mobile-ios",
  "inference_capable": true,
  "neural_engine_tops": 35,
  "supported_quantizations": ["int4", "int8", "float16"],
  "max_model_size_mb": 2000,
  "foreground_only": true
}
```

### 4.3 FedScheduler: Inference Routing

- Inference tasks are only routed to iOS nodes when the node has been active (heartbeat) within the last 60 seconds (confirms app is in foreground)
- No replication required for inference — outputs are deterministic for the same model + input
- iOS inference nodes earn at a premium rate: Neural Engine throughput per dollar is competitive for quantized model inference

### 4.4 Acceptance Criteria

- [ ] iOS app accepts inference task while in foreground
- [ ] Core ML model fetched from IPFS, loaded, and executed
- [ ] Result returned to coordinator and payment credited
- [ ] Node marked as unavailable within 60 seconds of app backgrounding
- [ ] Inference results match reference output from desktop node

---

## Cross-Cutting Work (All Phases)

### Wasm Task Standard

All tasks intended for mobile execution must be packaged as Wasm modules:

```
task.wasm       — compiled module (WASI target)
task.json       — manifest: name, version, max_duration_s, min_ram_mb, arch_hint
inputs/         — input data (or CIDs for large inputs via IPFS)
```

Build toolchain additions to `Makefile`:

```makefile
# Compile a Go task to Wasm
build-task-wasm:
    GOOS=wasip1 GOARCH=wasm go build -o tasks/$(TASK)/task.wasm ./tasks/$(TASK)
```

### Lightning Integration for Mobile

**File:** `internal/payment/lightning.go`

Add `CreateHoldInvoice(amount, hash)` and `SettleHoldInvoice(preimage)` methods. The HTLC flow:

```
1. Coordinator creates hold invoice H for mobile node
2. Mobile node receives task + invoice H
3. Mobile node completes task, submits result
4. Coordinator verifies result (Phase 2: against shadow; Phase 4: deterministic check)
5. Coordinator settles invoice → mobile node receives payment
6. On verification failure: coordinator cancels hold invoice → no payment
```

### OPA Policy Extensions

**File:** `configs/policies/resource_sharing.rego`

```rego
# Replication requirements by node class
task_replication_factor[node_class] = factor {
    node_class := input.node.class
    factor := {
        "desktop":        1,
        "android-tv":     1,
        "mobile-android": 2,
        "mobile-ios":     1   # inference is deterministic
    }[node_class]
}

# Mobile nodes only receive tasks matching their constraints
mobile_eligible_task {
    input.task.duration_seconds <= input.node.max_task_duration_seconds
    not input.task.requires_inbound_connection
    input.node.plugged_in == true
    input.node.wifi_connected == true
}
```

### Coordinator WebSocket Hub

**New file:** `internal/httpapi/mobilehub.go`

```go
type MobileHub struct {
    mu      sync.RWMutex
    clients map[string]*MobileClient // nodeID → client
}

func (h *MobileHub) Register(nodeID string, conn *websocket.Conn)
func (h *MobileHub) Unregister(nodeID string)
func (h *MobileHub) PushTask(nodeID string, task *orchestration.Workload) error
func (h *MobileHub) Broadcast(msg interface{}) // for network announcements
```

---

## Timeline

| Phase | Scope | Estimated Duration | Dependencies |
|---|---|---|---|
| **Phase 1** | Android TV headless node | 3–4 weeks | Coordinator WebSocket hub |
| **Phase 2** | Android smartphone "Earn While Charging" | 6–8 weeks | Phase 1 + Wasm executor + task micro-segmentation |
| **Phase 3** | iOS management + monitoring client | 4–5 weeks | Phase 1 (coordinator WebSocket + APNs) |
| **Phase 4** | iOS Core ML inference endpoint | 3–4 weeks | Phase 3 + inference task type |

Total estimated duration: **16–21 weeks** for all four phases sequentially. Phases 2 and 3 can run in parallel (separate teams / separate Android Studio and Xcode projects).

---

## Files To Create / Modify

### New Files

| File | Phase | Description |
|---|---|---|
| `internal/orchestration/mobile.go` | 1 | Mobile node class constants + constraint types |
| `internal/httpapi/mobilehub.go` | 1 | WebSocket hub for mobile node connections |
| `internal/wasm/executor.go` | 2 | Go-side Wasm task executor (wazero) |
| `internal/payment/htlc.go` | 2 | Lightning hold invoice (HTLC) helpers |
| `internal/notification/apns.go` | 3 | Apple Push Notification Service client |
| `mobile/android-tv/` | 1 | Android TV application (Kotlin) |
| `mobile/android/` | 2 | Android smartphone application (Kotlin) |
| `mobile/ios/` | 3 | iOS application (Swift / SwiftUI) |

### Modified Files

| File | Change |
|---|---|
| `internal/orchestration/scheduler.go` | Add mobile node routing, preemption tolerance, shadow replication |
| `internal/orchestration/workload.go` | Add `CheckpointData`, `SegmentIndex`, `SegmentCount` fields |
| `internal/payment/lightning.go` | Add `CreateHoldInvoice`, `SettleHoldInvoice`, `CancelHoldInvoice` |
| `configs/policies/resource_sharing.rego` | Add replication factor rules, mobile eligibility rules |
| `internal/httpapi/server.go` | Add `/api/v1/nodes/mobile/register` and `/ws/nodes` endpoints |

---

## Risk Register

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Google Play rejects app for background compute | Medium | High | Ensure foreground service is always visible; disclose in store listing; follow BOINC's precedent |
| Android OEM battery optimization overrides user grant | Medium | Medium | Document known OEM issues (Samsung, Xiaomi); provide OEM-specific guidance in app |
| Wasm executor performance insufficient for useful tasks | Low | Medium | Benchmark early; target data transformation and hash tasks (not compute-intensive ML) |
| Lightning node not available on user's device | Medium | Low | Custodial wallet in-app as default; external wallet optional |
| iOS Core ML model size exceeds device RAM | Low | Medium | Cap at 2 GB; use quantized models (int4/int8) |
| Coordinator WebSocket hub becomes bottleneck at scale | Low | High | Design hub as horizontally scalable from Phase 1; use Redis pub/sub for multi-instance |

---

*For background on why these design choices were made, see [`docs/research/MOBILE_PARTICIPATION.md`](research/MOBILE_PARTICIPATION.md).*
