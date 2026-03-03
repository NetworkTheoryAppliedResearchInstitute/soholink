# Mobile Devices as Federated Compute Nodes: A Research Report

**Project:** SoHoLINK — Federated SOHO Compute Marketplace
**Date:** 2026-03-02
**Status:** Technical Research — Internal

---

## Executive Summary

The global installed base of mobile devices represents the largest untapped pool of idle compute capacity in existence. With over 6.8 billion smartphones active worldwide, the aggregate CPU, GPU, and Neural Processing Unit (NPU) resources sitting dormant on nightstands and desk chargers each evening dwarfs the capacity of any conventional data center. For a federated compute marketplace like SoHoLINK — where the fundamental value proposition is aggregating underutilized SOHO hardware into a marketplace — mobile devices represent a logical, if technically challenging, frontier for network expansion.

The findings of this report are nuanced. Modern flagship smartphones, particularly those built on Apple Silicon (A17/A18 Pro) and Qualcomm Snapdragon 8 Gen 3/Gen 4, deliver CPU and neural inference performance that rivals or exceeds 2020-era laptop hardware. This is not a marginal gap — it is a substantive capability that makes mobile devices genuinely competitive as compute nodes for specific workload classes. However, three structural constraints — OS-level background processing restrictions, thermal throttling under sustained load, and carrier NAT preventing inbound connections — significantly limit how these devices can participate. The answer is not "mobile devices cannot participate" but rather "mobile devices can participate in a carefully scoped, consent-driven, and architecturally adapted manner."

For SoHoLINK specifically, the most actionable path is a tiered participation model: Android devices operating as short-burst compute workers and storage relay nodes while plugged in and on WiFi; iOS devices serving as management, monitoring, and billing clients; and Android TV/Fire TV boxes — always-on, thermally unconstrained devices running the Android stack — serving as the most immediately attractive mobile-ecosystem participant category. This report details the hardware capabilities, hard constraints, architectural adaptations, and concrete recommendations necessary to realize this vision.

---

## 1. The Opportunity: Mobile's Aggregate Scale

The sheer scale of the mobile installed base makes even a modest participation rate consequential. As of 2025–2026, approximately 6.8–7.1 billion smartphones are in active use globally, compared to an estimated 1.8–2.2 billion desktop and laptop computers. Traditional SOHO compute nodes — NAS devices, mini PCs, always-on workstations — number perhaps in the low hundreds of millions worldwide. The mobile device population is not merely larger; it is several times larger than all other consumer compute categories combined.

The idle capacity argument is compelling when behavioral patterns are considered. Research into device usage consistently shows that the median smartphone is actively used for 3–5 hours per day, leaving 19–21 hours in which the device is either sleeping, locked, or sitting on a charger. The overnight charging window alone — typically 6–8 hours during which the device is plugged into mains power, on home WiFi, and thermally unconstrained by user expectation — represents a regular, predictable window of compute availability. This is precisely the condition under which responsible background compute participation becomes viable.

Aggregate potential is significant even at low participation rates. If 1% of the global smartphone base — roughly 68 million devices — each contributed just 10 minutes of compute per day while plugged in, and each device delivered the equivalent of a single-core modern ARM CPU, the aggregate CPU-hours available would exceed 11 million CPU-hours daily. At 5% participation, that figure approaches 57 million CPU-hours per day. For context, major cloud providers sell CPU-hours at $0.01–$0.05 each at commodity pricing; the latent economic value of even 1% mobile participation is measurable in the hundreds of millions of dollars annually. For SoHoLINK, which aggregates SOHO nodes that each contribute similarly modest but consistent capacity, mobile participation is not a novelty — it is a natural extension of the same model to a vastly larger hardware pool.

---

## 2. Mobile Hardware Capabilities (2024–2026)

### Apple Silicon (A17 Pro, A18 Pro)

The A17 Pro, introduced with the iPhone 15 Pro line, represents a landmark in mobile silicon. Its 6-core CPU (2 performance + 4 efficiency cores) on a 3nm TSMC process delivers Geekbench 6 multi-core scores in the range of 7,200–7,500 — exceeding the multi-core performance of Intel Core i5-10th generation processors and competing directly with Core i7-11th generation parts. The 6-core GPU handles graphics workloads previously associated with discrete mobile GPUs. Most significantly for compute marketplace purposes, the 16-core Neural Engine delivers **35 TOPS** (tera-operations per second) of machine learning inference throughput. This is sufficient to run quantized large language models at multiple tokens per second, perform real-time image classification, and execute diffusion model inference at meaningful resolution.

The A18 Pro (iPhone 16 Pro) pushes further: the Neural Engine reaches approximately 38 TOPS, and the 6-core CPU on the refined 3nm process closes the gap with Apple's M-series chips still further. An M2 MacBook Air posts Geekbench 6 multi-core scores around 12,000, roughly 60% higher than an A17 Pro. The gap is meaningful but no longer categorical. For inference-heavy, parallelizable, or short-burst workloads, an iPhone 15 Pro is not a novelty compute node — it is a credible one.

RAM remains a constraint: A17 Pro and A18 Pro devices ship with 8 GB. This limits the working set for any compute task — models requiring more than 6 GB of resident memory are impractical given OS overhead.

### Qualcomm Snapdragon 8 Gen 3 / Gen 4

The Snapdragon 8 Gen 3, manufactured on TSMC 4nm, features a 1+3+2+2 CPU core configuration: one prime Cortex-X4 core at 3.3 GHz, three performance Cortex-A720 cores, two mid Cortex-A720 cores, and two efficiency Cortex-A520 cores. Geekbench 6 multi-core scores land between 6,800 and 7,400 depending on thermal conditions and OEM tuning — directly comparable to the A17 Pro in aggregate throughput. The Adreno 750 GPU is exceptionally competitive for compute workloads, and the Hexagon NPU delivers up to **45 TOPS** of AI inference throughput, actually exceeding the A17 Pro's Neural Engine in raw TOPS. The Snapdragon 8 Gen 3 is found in Samsung Galaxy S24, Xiaomi 14, OnePlus 12, and dozens of other flagship Android devices.

The Snapdragon 8 Gen 4 (Oryon CPU cores, first appearing in late 2024 devices) represents a more significant architectural jump: Qualcomm's custom Oryon cores push single-core performance meaningfully higher. Multi-core Geekbench scores for Gen 4 devices push into the 8,500–9,500 range, with NPU throughput reaching upward of 50 TOPS.

### MediaTek Dimensity 9300

MediaTek's Dimensity 9300 took a distinctive architectural approach: an all-big-core design using four Cortex-X4 prime cores and four Cortex-A720 cores, abandoning efficiency cores in favor of raw throughput. This makes it exceptionally strong for burst compute workloads but increases power consumption during sustained load. The Imagia NPU delivers competitive AI inference throughput. Dimensity 9300 devices are particularly prevalent in Asian markets, representing a substantial fraction of the potential SoHoLINK user base.

### Mid-Range Reality: Snapdragon 7s Gen 3 and Apple A16

The median participating device will not be a flagship. The Snapdragon 7s Gen 3, found in devices at the $250–$400 price point, delivers Geekbench 6 multi-core scores in the 3,000–3,800 range — comparable to a 2019-era Intel Core i5 laptop. The Apple A16 (iPhone 15 base, iPhone 14 Pro) posts multi-core scores around 5,400. These are meaningful compute resources. A network that schedules tasks appropriately for mid-range hardware — shorter tasks, lighter memory footprints — can include these devices productively.

**Key conclusion:** Flagship mobile devices (2024–2026 vintage) rival 2020-era laptop CPUs in aggregate throughput. GPU and NPU capabilities are particularly competitive for ML inference workloads. The hardware case for mobile participation is strong; the constraints are architectural and environmental, not silicon-limited.

---

## 3. The Hard Constraints

### 3.1 Battery and Thermal Throttling

Sustained compute load on a smartphone imposes energy draw of approximately **800 mA to 1.5 A** at 3.7–4.2V battery voltage, corresponding to 3–6 W of CPU/SoC power consumption. A typical 4,500 mAh battery (roughly 16–17 Wh) can sustain this for 3–5 hours before depletion under ideal conditions. Real-world sustained compute on a smartphone drains a typical battery in 2–4 hours.

More immediately limiting is thermal throttling. Mobile devices have surface areas of 60–80 cm² and thermal mass of 150–200 grams, compared to a laptop with a fan and heatsink assembly capable of dissipating 15–45 W continuously. Sustained 100% CPU utilization causes die temperatures to rise rapidly, and both iOS and Android implement aggressive thermal governors to prevent damage. On iPhones, iOS begins throttling CPU clock speeds when die temperatures approach 40°C; perceptible performance degradation typically begins within **10–15 minutes** of sustained 100% load in 25°C ambient conditions. In warm environments (30°C+), throttling can begin within 5 minutes.

Android devices implement various thermal governor strategies — step-rate governors, bang-bang governors, and more sophisticated model-predictive controllers on premium SoCs. The practical effect is similar: sustained CPU load beyond 15–20 minutes triggers meaningful performance degradation that makes compute output unreliable for tasks expecting consistent throughput.

**Mitigations — design choices, not workarounds:**
- **Plug-in gate:** Gating on `ACTION_POWER_CONNECTED` solves battery drain and provides passive cooling benefit from ambient air flow around the charging device
- **Thermal API:** Android 10+ `PowerManager.getThermalHeadroom()` enables dynamic throttling before the OS does it involuntarily
- **Micro-segments:** Tasks ≤120 seconds with mandatory cool-down intervals keep sustained thermal load within acceptable bounds

### 3.2 OS Background Processing Restrictions

#### Android

Android's background processing restrictions have progressively tightened from API 23 (Android 6.0, 2015) through present-day versions:

- **Doze Mode (API 23+):** Restricts wakelocks, network access, and JobScheduler execution when the device is stationary, screen-off, and unplugged. In the deepest Doze states, permitted maintenance windows occur only once every several hours.
- **App Standby Buckets (API 28+):** Classifies apps into ACTIVE, WORKING_SET, FREQUENT, RARE, and RESTRICTED buckets based on recent usage. An app the user has not actively used for several days is placed in RARE or RESTRICTED, where scheduled jobs may be deferred by 24 hours or more. A SoHoLINK compute worker app that the user rarely opens would rapidly degrade to RARE bucket status.
- **Background Service Limits (API 26+):** Background services are killed within seconds of the app leaving the foreground unless running as Foreground Services with an active persistent notification.

The viable path is narrow but real: a **Foreground Service with a persistent notification** combined with an explicit battery optimization exemption granted by the user. WorkManager handles scheduled task polling when the foreground service is not active. This is exactly the architecture BOINC on Android uses — and it works — but it requires informed, intentional user action. Google Play policies restrict apps from prompting users directly to grant battery optimization exemptions, so the UX flow requires careful design.

#### iOS

iOS background processing restrictions are categorically more severe and structurally non-negotiable:

- **Application suspension:** When an app moves to the background, it receives at most a few seconds to save state before being suspended — the CPU is paused.
- **BGTaskScheduler (iOS 13+):** `BGProcessingTask` offers approximately 30 seconds of CPU time when the device is plugged in and idle; `BGAppRefreshTask` offers roughly 30 seconds of network access. Both are non-deterministic — iOS decides when they run based on usage patterns, power state, and system load.
- **Silent push notifications:** Can wake an app for approximately 30 seconds, but Apple rate-limits these aggressively and they cannot be used for sustained compute delivery.
- **The sole exception:** On-device ML via Core ML. Apple explicitly supports Neural Engine inference, which can be invoked from the foreground. An iPhone 15 Pro running a quantized inference model while the app is open is a legitimately competitive inference node — but only while the user has the app open.

**The iOS bottom line:** iOS cannot serve as a general-purpose compute worker in any distributed compute architecture. This is a structural property of the iOS platform that Apple has consistently enforced and shows no signs of relaxing. Any SoHoLINK mobile strategy that assumes iOS background compute is viable will fail.

### 3.3 Network Constraints

**Carrier-Grade NAT (CGNAT)** is the default network topology for virtually all 4G LTE and 5G cellular connections. Mobile devices share public IPv4 addresses across thousands of subscribers; there is no path for inbound TCP connections to reach a specific device. Even on IPv6 — which many carriers support internally — firewalls and dynamic address assignment prevent reliable inbound connectivity.

Home WiFi mitigates but does not eliminate this: home networks also typically sit behind NAT, though a single NAT layer is more navigable than CGNAT. STUN and TURN servers can establish peer-to-peer channels through NAT in many configurations, but TURN relay fallback consumes server bandwidth and adds latency.

**The architectural implication is fundamental:** mobile nodes must operate as task-pulling clients rather than task-accepting servers. The SOHO model — advertise an address, accept incoming work — does not apply to mobile. Mobile nodes must poll a signaling server (or maintain a long-lived WebSocket connection to the SoHoLINK coordinator), retrieve available tasks, execute them locally, and push results outbound.

**Data caps** impose a hard practical limit on bandwidth-intensive workloads. A typical cellular plan offers 5–50 GB per month. A WiFi-only gate — refusing to accept compute tasks unless connected to WiFi — is not optional; it is a mandatory safeguard for user trust and financial safety.

### 3.4 App Store Policy Restrictions

**Apple App Store Review Guideline 2.4.2** explicitly prohibits applications that "unnecessarily drain battery" or "put unnecessary burden on device resources." This guideline is broad enough to encompass virtually any sustained background compute application. Any design approximating background resource use risks removal.

**Google Play Policy** prohibits applications that mine cryptocurrency or perform background computation for third parties without user awareness. Following the 2018 enforcement action removing hundreds of cryptomining apps from the Play Store, the precedent is clear: undisclosed background compute participation is grounds for immediate removal. Disclosed, user-consented participation is different — BOINC has remained on Google Play for years — but disclosure must be explicit, prominent, and verifiable in both the app listing and in-app UI.

**The design implication:** Covert mobile participation is impossible to deploy at scale through legitimate app distribution channels. The design must be transparent by architecture: compute only while plugged in, only on WiFi, only with a visible foreground notification, only after explicit user opt-in with clear explanation of what is happening and what the user earns.

---

## 4. Precedent: Systems That Have Tried

**BOINC on Android** is the most instructive precedent. Launched around 2012, it remains available on Google Play with over 100,000 registered Android devices. Its architecture — foreground service, charging detection, WiFi gate, explicit opt-in — is precisely the viable design described in this report. BOINC's default configuration caps CPU usage at "moderate" rather than maximum — a deliberate choice made after early users complained about device heat. **No iOS version of BOINC exists;** the BOINC team has stated publicly that iOS restrictions make it impractical.

**Folding@home on Android**, released 2021 and subsequently discontinued, is a cautionary data point. Despite established brand recognition and scientific mission, sustained thermal and battery issues led to poor user retention and eventual discontinuation. The lesson is not that mobile participation is impossible but that full-intensity sustained compute is untenable — BOINC's moderated-intensity approach has proven more durable.

**Golem Network** treats mobile devices as dashboard clients exclusively — no worker node role exists or is planned. **Render Network's** GPU rendering workloads exceed mobile GPU capabilities for their specific 3D rendering use case, though mobile NPUs are competitive for ML inference workloads Render does not currently target.

**Academic research on mobile volunteer computing (2015–2023)** consistently converges on three findings: users are willing to contribute while plugged in; thermal throttling degrades compute output quality after approximately 15 minutes of sustained load; and task granularity must remain below approximately 5 minutes for reliable completion rates. Experiments with short, stateless microbursts under 60 seconds show completion rates above 94% on Android. Tasks exceeding 10 minutes on mobile nodes show completion rates dropping to ~60%, compared to 99%+ on desktop nodes.

---

## 5. What Roles Mobile Devices CAN Play

### Tier 0 — iOS (Severely Restricted)

- **Neural Engine inference endpoint:** Core ML models can execute on-device Neural Engine while the app is in the foreground. An iPhone 15 Pro running a quantized inference model is a legitimately competitive inference node for that specific use case — but only while the user has the app open.
- **IPFS content pinning:** While the app is open in the foreground.
- **Billing, monitoring, and management client:** Viewing earnings, approving job types, configuring node policies, and observing the globe visualization — the primary and natural iOS role, requiring no background processing.
- **NOT viable:** General CPU compute worker, background storage relay, persistent network node.

### Tier 1 — Android (Constrained but Viable with Explicit User Consent)

When plugged into mains power, connected to WiFi, with explicit battery optimization exemption granted by the user:

- **Compute worker:** Tasks ≤2 minutes execute reliably via Foreground Service. Ideal task types: hash verification, data transformation, lightweight ML inference, WebAssembly workloads, text processing.
- **Storage relay node:** IPFS pinning and relay while plugged in; 128–512 GB internal storage is meaningful capacity; UFS 4.0 storage delivers 4+ GB/s sequential reads.
- **WebAssembly execution environment:** Wasm tasks are architecture-agnostic and run on ARM64 mobile with the same task definition as x86 desktop nodes — the natural portable compute unit for SoHoLINK's mobile integration.

### Tier 2 — Android with Root / Custom ROM

Rooted devices or devices running custom ROMs (GrapheneOS, CalyxOS, LineageOS) can host persistent background daemons, integrate directly with thermal management subsystems, and bypass battery optimization restrictions. This represents the technical ceiling of mobile participation but is not a realistic mainstream deployment target.

### Tier 3 — Android TV / Amazon Fire TV Boxes *(Most Immediately Attractive)*

This category deserves particular emphasis. Android TV and Amazon Fire TV devices run the Android OS stack but with fundamentally different operational characteristics: **always-on, always-plugged-in, passively or actively cooled, no battery at all.** They are never subject to Android's battery optimization or Doze Mode behaviors in the way phones are. They sit connected to home networks continuously.

A typical Fire TV Stick 4K Max or Android TV box provides 2–4 GB RAM, a mid-range ARM SoC, and continuous network connectivity. CPU performance is modest compared to flagship phones, but the **duty cycle is unlimited.** A compute node that can run 24/7 without thermal throttling or battery concerns, even at half the per-task throughput of a flagship phone, outperforms that phone over any 24-hour period by a substantial margin.

For SoHoLINK, Android TV boxes represent the most immediately deployable "mobile OS" compute participant. They require no special user behavior changes, face no App Store policy concerns about background computation, and exist in tens of millions of households that have no traditional SOHO server hardware. They bridge the gap between the mobile ecosystem and always-on SOHO compute.

---

## 6. Architecture for Mobile Participation in SoHoLINK

### 6.1 FedScheduler Adaptation

SoHoLINK's FedScheduler requires several additions to support mobile nodes. A new node class taxonomy:

```go
type NodeClass string

const (
    NodeClassDesktop       NodeClass = "desktop"
    NodeClassMobileAndroid NodeClass = "mobile-android"
    NodeClassMobileIOS     NodeClass = "mobile-ios"
    NodeClassAndroidTV     NodeClass = "android-tv"
)
```

Mobile nodes advertise constraint tags the scheduler uses to filter task assignment:

| Tag | Meaning |
|---|---|
| `"requires-plugged-in": true` | Only assign tasks when node reports mains power |
| `"max-task-duration-seconds": 120` | Scheduler must not assign tasks exceeding this |
| `"arch": "arm64"` | Restricts to ARM-compatible containers (Wasm bypasses this) |
| `"mobile": true` | Triggers replication policy (see §7) |
| `"wifi-only": true` | Node only accepts tasks on WiFi |

The scheduler must implement **preemption tolerance** for mobile nodes. Unlike SOHO desktop nodes that can be assumed available for hours, mobile nodes may vanish mid-task (user unplugs, moves out of WiFi range, thermal shutdown). Tasks assigned to mobile nodes must carry checkpoint/restart metadata enabling another node to resume from the last checkpoint if the mobile node disappears.

### 6.2 Android Client Architecture

The Android client must be built around OS lifecycle realities:

- **ForegroundService** with persistent notification: *"SoHoLINK: earning 0.0042 SATS — tap to pause"*
- **BroadcastReceiver** for `Intent.ACTION_POWER_CONNECTED` / `ACTION_POWER_DISCONNECTED` — auto-start/stop on plug state, no user interaction required
- **ConnectivityManager.NetworkCallback** — pause task acceptance when transitioning from WiFi to cellular
- **`PowerManager.getThermalHeadroom(seconds)`** (Android 10+) — reduce task concurrency as headroom drops below 0.5; pause task acceptance entirely below 0.2
- **WorkManager** — scheduled task polling when the foreground service is not active

### 6.3 Task Design for Mobile Compatibility

Existing SoHoLINK task definitions need decomposition into mobile-compatible micro-segments. A task running 10 minutes on a desktop should be expressible as a sequence of ≤120-second segments with defined checkpoint state serialized to disk between segments, enabling resume if Android kills the foreground service under memory pressure.

**WebAssembly is the natural portable task container.** A Wasm module compiled from Go, Rust, or C runs identically on ARM64 Android, ARM64 iOS (foreground only), x86_64 Linux, and macOS — a single task artifact deployable across the entire SoHoLINK node population. The `wasmer` and `wasmtime` runtimes have mature Go bindings and acceptable overhead for compute-bound tasks.

### 6.4 Payment Rails for Mobile

Lightning Network micropayments are the correct settlement mechanism for mobile compute participation. Individual mobile task completions may be worth sub-cent amounts; Stripe payouts require minimum thresholds ($1–$10+) inappropriate for micro-task granularity. Lightning payments settle in milliseconds with negligible minimum amounts.

The mobile app should maintain a **custodial Lightning wallet** for earnings accumulation, with configurable auto-withdrawal to an external Lightning wallet when balance crosses a user-set threshold. This extends SoHoLINK's existing `internal/payment/lightning.go` to the mobile settlement case.

### 6.5 Network Architecture for Mobile Nodes

All mobile nodes must be treated as **outbound-only pull clients** — CGNAT is assumed regardless of reported network configuration:

- Mobile nodes initiate a long-lived WebSocket connection to the SoHoLINK signaling coordinator
- Coordinator pushes available task descriptors to connected mobile nodes based on their capability profile
- Mobile nodes pull task payloads via the WebSocket channel or direct HTTPS fetch
- Completed results are pushed outbound to the coordinator or directly to the requesting party via WebRTC data channel
- STUN/TURN relay is available for peer-to-peer connections where the coordinator facilitates the initial handshake
- The coordinator **never** attempts inbound connections to mobile nodes

---

## 7. Security Considerations

Mobile nodes present a distinct trust profile from SOHO desktop nodes. SOHO hardware is typically operated by a single owner with full control of the software stack. A mobile device running a sandboxed application in a shared OS environment presents more attack surface, and the incentive for a malicious actor to submit incorrect results to collect payment is real.

**Primary mitigation — optimistic replication with payment gating.** Any task assigned to a mobile node is also assigned to at least one additional trusted node. Results are compared before payment releases. OPA policy extension:

```rego
# configs/policies/resource_sharing.rego

task_replication_factor[node_class] = factor {
    node_class := input.node.class
    factor := {"mobile-android": 2, "android-tv": 1, "desktop": 1}[node_class]
}
```

**Lightning hold invoices (HTLC)** enforce result verification before settlement. The mobile node receives a hold invoice; the hash preimage is released — unlocking payment — only after the coordinator verifies the result against the replication partner. This prevents payment-before-result exploitation without requiring the coordinator to trust the mobile node's self-reporting.

**Reputation scoring** provides a longer-term layer: mobile nodes accumulate a result correctness score over time; nodes with low correctness scores receive higher replication factors or reduced task priority. This creates an economic incentive structure aligned with honest result submission.

---

## 8. Recommendations for SoHoLINK

Listed in implementation priority order:

1. **Build the Android client first.** iOS is too restricted for compute participation. An Android client with foreground service, charging detection, and WiFi gate can deliver real compute capacity to the SoHoLINK network immediately. iOS investment should follow, focused exclusively on the monitoring and management client role.

2. **Implement "Earn While Charging" mode with explicit, transparent gating.** Compute participation activates automatically on `ACTION_POWER_CONNECTED` + WiFi + user opt-in; deactivates automatically on power disconnect or WiFi loss. The user must have explicitly enabled the mode — it must not activate on first install. This design navigates Google Play policy requirements cleanly.

3. **Design all tasks as ≤120-second micro-segments with disk checkpointing.** This is the single most important task design constraint for mobile compatibility. The orchestration layer must support segment-level reassignment to alternative nodes when a mobile node vanishes mid-task.

4. **Adopt WebAssembly as the portable task container standard.** One Wasm artifact runs on ARM64 Android, ARM64 iOS (foreground), x86_64 Linux/macOS/Windows — covering the entire SoHoLINK node spectrum. This investment pays dividends across the full ecosystem, not only mobile.

5. **Add the `mobile` node class to FedScheduler with preemption tolerance.** Capability flags, constraint tags, and the 2× replication policy for mobile nodes must be present in the scheduler before any mobile node can participate safely.

6. **Treat Android TV / Fire TV as a first-class "headless mobile" compute category.** These devices have the most favorable properties of any mobile-ecosystem hardware: always plugged in, thermally unconstrained, no App Store background processing restrictions. An Android TV app could deploy into tens of millions of households where no traditional SOHO hardware exists.

7. **Do not attempt iOS compute participation.** The restrictions are structural, not merely inconvenient. Engineering investment in iOS compute workers will produce unreliable results and risk App Store policy violations. The correct iOS investment is a polished management and earnings monitoring client.

8. **Use Lightning micropayments for all mobile earnings.** Integrate a custodial in-app Lightning wallet with configurable auto-withdrawal. Sub-cent task payments are only economically rational through Lightning.

9. **Integrate thermal headroom monitoring from day one.** Build `PowerManager.getThermalHeadroom()` throttling logic into the Android client's compute loop before launch, not as a post-release hotfix after user complaints about device heat.

---

## 9. Conclusion

The central finding of this report is affirmative but qualified: mobile devices can participate in the SoHoLINK federated compute marketplace in constrained but meaningful ways that justify the architectural investment required. The qualification matters. Mobile participation requires design choices — short task granularity, pull-based network topology, explicit user consent, thermal awareness, and replication-based trust — that differ from the SOHO desktop node model in fundamental ways. These are engineering problems with known solutions, not fundamental barriers.

Android devices operating under the "earn while charging" model — plugged in, on WiFi, foreground service active with user consent — can serve as legitimate compute workers for short-burst tasks, IPFS storage relay nodes, and lightweight ML inference endpoints. The hardware is genuinely capable; the constraints are environmental and policy-driven. Android TV and Amazon Fire TV devices, which eliminate battery and thermal constraints entirely while running the same Android OS stack, represent the single most immediately attractive expansion of the SoHoLINK network into households that possess no traditional SOHO hardware.

iOS devices cannot participate as compute workers under any architecturally sound design. Apple's background processing restrictions are structural, not configurable. The correct iOS strategy is a first-class monitoring, management, and earnings client — an experience that makes SoHoLINK visible and valuable to hundreds of millions of iPhone users, and that serves as the onramp for households where an Android device or Android TV box provides the actual compute contribution.

The aggregate opportunity is not trivial. A marketplace that successfully extends its reach into mobile devices — even capturing a fraction of a percent of the global smartphone base during the plugged-in overnight window — accesses a pool of idle compute capacity that dwarfs any feasible expansion of the traditional SOHO hardware base. The architectural work is substantial but well-defined. The precedents — BOINC on Android, distributed mobile computing research, WebAssembly portability — provide a validated technical foundation. For SoHoLINK, mobile participation is not a distant aspiration; it is the next logical extension of a marketplace built on the premise that the compute capacity people already own, and largely leave idle, is worth aggregating.
