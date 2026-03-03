# Machine Learning Approaches for Intelligent Load Balancing in SoHoLINK

**Research Report — SoHoLINK Federated Compute Marketplace**
**Date:** 2026-03-02
**Scope:** ML-driven scheduling for heterogeneous SOHO node federation

---

## Abstract

SoHoLINK's FedScheduler employs static, heuristic-weighted scoring across five dimensions (cost, latency, reputation, capacity, reliability) with fixed coefficients and round-robin mobile dispatch. This approach is structurally incapable of adapting to the multi-modal, high-churn dynamics of a SOHO federation where nodes range from always-on AMD64 desktops to thermally-constrained ARM64 Android smartphones that vanish mid-task. This report surveys and evaluates six machine learning paradigms for replacing or augmenting the current scheduler, provides concrete feature engineering guidance, describes training infrastructure requirements, and delivers a prioritized, phased implementation roadmap grounded in SoHoLINK's actual codebase architecture.

---

## 1. Problem Framing

### 1.1 Why Static Heuristic Schedulers Break

The current `Placer.ScoreNodes` function in `internal/orchestration/placer.go` computes a composite score using hardcoded weights:

```
Score = 0.30·cost + 0.20·latency + 0.20·reputation + 0.15·capacity + 0.15·reliability
```

This design has three fatal assumptions that do not hold in SoHoLINK:

**Stationarity assumption:** Fixed weights assume that a desktop node with 80% CPU headroom is uniformly preferable to a lightly-loaded Android TV. In reality, a desktop running video encoding may exhibit high thermal variance, while an Android TV at 3 AM local time is categorically idle. The optimal weight vector changes continuously with time-of-day, workload class, and system-wide queue depth.

**Independence assumption:** The score treats each node in isolation. But in a shadow-replicated mobile workflow — where `ScheduleMobile` dispatches to a mobile primary and `assignWithReplication` dispatches a shadow to a desktop — the two placements are correlated. Poor joint placement (e.g., both on the same network segment or sharing a bottleneck) inflates failure probability in ways a per-node score cannot capture.

**Reactivity assumption:** `AutoScaler.Evaluate` responds to current CPU utilization, but mobile nodes in SoHoLINK are available in probabilistic windows driven by human behavior (charging overnight, commuting, working from home). A scheduler that cannot predict availability windows will perpetually react to churn rather than anticipate it.

The 90-second heartbeat timeout in the `WorkloadMonitor` is the system's only defense against disappearing nodes. When a mobile node drops and `PreemptMobileWorkload` fires, the task segment is lost and must restart from the last checkpoint. Under high mobile churn this creates cascading resubmissions that saturate the `PendingQueue`.

### 1.2 Multi-Objective Nature

The scheduling problem is genuinely multi-objective with non-commensurable objectives:

| Objective | Metric | Current Handling |
|---|---|---|
| Task completion latency | Segment wall-clock time | Latency score (linear, static weight) |
| Cost efficiency | Cents per GFLOP-second | Cost score (linear, static weight) |
| Node utilization | Active CPU fraction across fleet | Capacity score (headroom proxy only) |
| Reliability | HTLC settle rate, shadow match rate | Failure rate (historical, not predictive) |
| Energy efficiency | Watt-hours per completed segment | Not modeled |
| Thermal safety | Android `getThermalHeadroom()` | Not modeled |
| Shadow pair coherence | Probability both replicas survive | Not modeled |

No linear weighted sum can Pareto-optimize across these simultaneously. The Pareto frontier shifts as workload mix changes (batch vs. latency-sensitive), as mobile node population changes (peak evening hours vs. overnight), and as market conditions change (more providers joining depresses prices, lowering the importance of the cost objective).

### 1.3 What Makes SOHO Federations Uniquely Hard

**Absence of SLAs:** Commercial clouds (AWS, GCP) back node availability with contractual uptime guarantees. SoHoLINK nodes participate voluntarily. A mobile-android node can withdraw consent at any moment via the `Plugged`/`WiFi`/user-consent triple gate. There is no penalty for node dropout beyond LBTAS score degradation, which is lagged and mild.

**CGNAT and connectivity asymmetry:** SOHO nodes typically sit behind carrier-grade NAT. The coordinator cannot probe them directly; it can only receive heartbeats and WebSocket connections that originate from the node. This means the coordinator's view of node health is always stale by at least one heartbeat period (90 seconds). An ML scheduler must reason under this observational delay.

**Thermal variability:** Android's thermal governor operates on a 0.0–1.0 headroom scale. Headroom degrades non-linearly under sustained load, and recovery is slow (minutes). The current scheduler has no model of thermal headroom dynamics. A node that reports `thermalHeadroom = 0.7` at dispatch time may throttle to `0.2` by segment completion, extending wall-clock time and degrading HTLC settlement probability.

**HTLC-gated payments:** Lightning HTLCs (`CreateHoldInvoice` / `SettleHoldInvoice` / `CancelHoldInvoice` in `internal/payment/htlc.go`) create a non-standard reward signal. Payment settles only after the shadow replica verifies result hash equality. This means a node can complete a task but not receive payment if the shadow is delayed or fails. The effective reward for a scheduling decision is delayed by the shadow verification latency, which is itself a function of desktop node availability.

**Voluntary participation under LBTAS:** LBTAS scoring in `internal/lbtas/score.go` uses a simple weighted rolling average with diminishing weight for experienced nodes. A node with 50+ transactions contributes only 5% per new rating, making the score highly inertial. An ML scheduler that treats the LBTAS score as a live signal for exploitation will over-exploit nodes whose scores have not yet reflected recent degradation.

### 1.4 Formal Problem Statement

Let `N = {n_1, ..., n_k}` be the set of currently-available nodes, partitioned into classes `C = {desktop, android-tv, mobile-android}`. Let `T = {t_1, ..., t_m}` be the set of pending task segments, each with resource requirements `r(t)`, estimated duration `d(t)`, deadline `δ(t)`, and payment value `v(t)`.

Define the assignment function `π: T → N ∪ {defer}` that maps each task to a node or defers it. The scheduling problem is:

```
minimize   Σ_t [ α·L(π(t), t) + β·C(π(t), t) + γ·(1 - Q(π(t), t)) + δ·E(π(t), t) ]

subject to:
  Σ_{t: π(t)=n} r(t).cpu    ≤ cap(n).cpu         ∀ n ∈ N
  Σ_{t: π(t)=n} r(t).mem    ≤ cap(n).mem         ∀ n ∈ N
  d(t) ≤ MaxDuration(class(π(t)))                 ∀ t ∈ T (mobile constraint)
  shadow(t) ∈ N_desktop     if class(π(t)) = mobile-android
  π(t) ∈ N_eligible(t)      (OPA policy gates)
  E[HTLC_settle(π(t))] ≥ θ_settle                (reliability floor)
```

Where:
- `L(n, t)` = expected completion latency (affected by thermal headroom, current load)
- `C(n, t)` = cost (price × duration)
- `Q(n, t)` = probability of HTLC settlement (completion quality)
- `E(n, t)` = energy cost
- `α, β, γ, δ` = objective weights (themselves a function of system state)
- `θ_settle` = minimum acceptable settlement probability (e.g., 0.90)

This is a stochastic, constrained, multi-objective combinatorial optimization problem. It is NP-hard in the general case and additionally complicated by the fact that `cap(n)`, `class(n)`, and `N` itself are all stochastic processes, not static inputs.

---

## 2. Candidate ML Paradigms

### 2.A Reinforcement Learning

#### State Space Design

The RL state vector must capture enough information to distinguish good from bad scheduling decisions. For SoHoLINK, the state `s_t` at decision time should be a concatenation of:

**Node-level features** (per-node, flattened or embedded):
```
node_features = [
  class_onehot(4),          // desktop | android-tv | mobile-android | mobile-ios
  thermal_headroom(1),       // 0.0–1.0 from Android getThermalHeadroom()
  battery_pct_normalized(1), // 0–1; -1 for non-battery nodes
  battery_trend(1),          // derivative: charging (+), draining (-), stable (~0)
  wifi_rssi_normalized(1),   // 0–1 (proxy for packet loss risk)
  lbtas_score_normalized(1), // 0–1
  historical_completion_rate(1), // rolling 30-task window
  current_cpu_util(1),       // 0–1
  current_mem_util(1),       // 0–1
  active_job_count(1),       // integer, normalized
  uptime_tod_embedding(4),   // hour-of-day availability prior, compressed
  last_heartbeat_age_s(1),   // seconds since last heartbeat
  arch_onehot(2),            // amd64 | arm64
]
// dim per node: ~20; with 100 nodes: 2000-dim (need embedding)
```

**Workload-level features** (per task segment):
```
task_features = [
  wasm_size_log(1),          // log(bytes) — proxy for data transfer time
  estimated_flops_log(1),    // log(FLOPs)
  segment_index(1),          // which segment in a multi-segment task
  segment_count(1),          // total segments
  payment_value_sat_log(1),  // log(satoshis)
  deadline_slack_s(1),       // seconds until deadline
  requires_shadow(1),        // binary: mobile-android → 1
  arch_requirement(2),       // amd64 | arm64
]
// dim: ~9
```

**System-level features**:
```
system_features = [
  pending_queue_depth(1),
  mobile_node_count(1),
  desktop_node_count(1),
  atv_node_count(1),
  htlc_cancel_rate_5min(1),  // recent cancellation pressure
  shadow_mismatch_rate(1),   // recent verification failures
  global_utilization(1),     // mean utilization across fleet
]
// dim: ~7
```

Total raw state dimension: `~20·|N| + 9 + 7`. For federations with up to 500 nodes this is 10,000+ dimensions, necessitating a node-embedding approach (see Section 2.C on GNNs).

#### Action Space

The action space is `A = N ∪ {defer}` — a discrete action per task segment. For a 500-node federation this is a 501-dimensional discrete action space. Standard DQN scales poorly here; Pointer Networks or attention-based action selection (as in the Decima scheduler) are more appropriate.

For multi-segment mobile tasks, the action space must also include the shadow assignment jointly: `A_joint = N_primary × N_shadow`, constrained so `shadow ∈ N_desktop`. This joint space grows as `|N_mobile| × |N_desktop|`, which is manageable (e.g., 50 × 200 = 10,000) but still requires attention-based selection.

#### Reward Function Design

The reward signal must bridge the HTLC settlement delay. A candidate composite reward:

```
R(t+k) = w1 · completion_speed_bonus(t)
        - w2 · normalized_cost(t)
        + w3 · htlc_settled(t+k)          // delayed; k = shadow verification lag
        - w4 · htlc_cancelled(t+k)        // delayed penalty
        - w5 · node_dropout_penalty(t)    // immediate: did the node drop during execution?
        - w6 · energy_cost(t)
        + w7 · shadow_match_bonus(t+k)    // verification succeeded
```

The HTLC settlement signal arrives minutes after dispatch. This is a **sparse, delayed reward** problem. Techniques:

- **Reward shaping:** Add dense intermediate rewards for heartbeat continuity (node survived to next heartbeat checkpoint during task execution).
- **Return discounting:** Use `γ ≈ 0.95` per 30-second interval so the HTLC reward at 3–5 minutes still has meaningful present value.
- **Hindsight experience replay (HER):** Treat HTLC cancellation outcomes as relabeled successes for a hypothetical task that would have fit the node's capacity at the moment of failure, to extract learning signal from failures.

#### Algorithm Comparison

| Algorithm | Pros for SoHoLINK | Cons |
|---|---|---|
| **DQN** | Simple, stable with replay buffer; handles discrete actions | Discrete action space scaling; value overestimation in sparse reward |
| **PPO** | On-policy, good for non-stationary distributions (node churn); clip prevents catastrophic updates | Requires large on-policy batches; sample-inefficient with sparse HTLC rewards |
| **SAC** | Off-policy, entropy regularization prevents premature convergence; handles exploration well | Continuous action version requires action discretization (Gumbel-softmax) |
| **A3C** | Asynchronous workers match the concurrent nature of scheduling; good for wall-clock throughput | More complex to implement; gradient staleness under high node churn |
| **Decima-style** | Designed specifically for DAG scheduling; uses graph embedding + policy gradient | Complex; requires DAG representation of workloads |

**Recommendation for SoHoLINK:** SAC with Gumbel-softmax action discretization and hindsight experience replay. SAC's entropy maximization is particularly well-suited: it naturally encourages the scheduler to maintain exploration across multiple equivalent nodes (good for load distribution) rather than always exploiting the current best node.

#### Multi-Agent RL: CTDE Architecture

Given the four node classes with fundamentally different dynamics, a **Centralized Training, Decentralized Execution (CTDE)** multi-agent setup is well-motivated:

- **Agent per class:** `π_desktop`, `π_atv`, `π_mobile_android` each learn a class-specific dispatch policy.
- **Centralized critic:** A global critic sees all agents' states and actions, enabling coordination signals (e.g., the desktop critic knows that a mobile task will need a shadow, factoring this into joint value estimation).
- **Decentralized execution:** Each class agent dispatches tasks from its class-specific queue using only locally observable node features plus a shared system embedding.

The CTDE pattern maps naturally onto SoHoLINK's existing architecture: `ScheduleMobile` and `scheduleWorkload` are already separate code paths. Each can be replaced by a class-specific policy inference call.

---

### 2.B Multi-Armed Bandit / Contextual Bandit

The contextual bandit framing is simpler than full RL and arguably more appropriate for the **stateless dispatch decision** — given a task segment, pick the best node now, without modeling future state transitions.

**Framing:** At each dispatch, the context `x_t = [task_features ∥ node_features]` is observed for each node-arm. The reward `y_t` is the HTLC settlement outcome (1 or 0), optionally combined with completion speed. The bandit picks arm `n*` to maximize expected reward.

**LinUCB:** Models expected reward as `E[y | x] = θ^T x` with upper confidence bound. Computationally lightweight (`O(d²)` per arm update, `d` ≈ feature dimension ~30 after node embedding). Works well when the reward function is approximately linear in features — a reasonable approximation for desktop nodes with predictable behavior, but less so for thermal-variable mobile nodes.

**NeuralLinear:** Trains a neural network as a feature extractor, then applies LinUCB in the last-layer embedding space. Better at capturing non-linear thermal dynamics. Requires a small network (2–3 layers, 64–128 units) small enough for <5ms inference budget.

**Thompson Sampling:** Maintains a Bayesian posterior over node quality. Samples from the posterior at each decision, providing principled exploration. Particularly effective for the **cold-start problem**: new nodes start with a diffuse prior (high uncertainty = high exploration), naturally receiving trial tasks without a separate exploration strategy.

**Cold-start:** When a new node joins (via `NodeDiscovery.DiscoverLoop`), its feature vector is known but reward history is empty. Thompson Sampling draws from the prior, giving the node a high probability of selection proportional to its uncertain upside. LinUCB provides an optimistic upper confidence bound. Both handle cold-start more gracefully than the current heuristic, which relies solely on LBTAS score (defaulting to 50 for new nodes).

**Why bandits may outperform full RL here:** The dispatch decision is approximately Markovian only at the segment level. The full task multi-step dependency (segment 0 → segment 1 → ... → HTLC settle) is better modeled by RL, but a single segment dispatch, conditioned on current system state, has weak dependency on the specific sequence of prior dispatch decisions. For stateless workloads (single-segment desktop tasks), a contextual bandit is theoretically sufficient and is dramatically simpler to train, serve, and debug.

---

### 2.C Graph Neural Networks

The federation is naturally a dynamic graph `G = (V, E)` where:
- **Vertices V:** Nodes (desktops, Android TVs, mobile phones) with feature vectors `h_v`
- **Edges E:** Communication topology (measured latency, shared network segments, geographic proximity)
- **Task nodes:** Tasks can be represented as additional vertices connected to their current candidate nodes, forming a bipartite subgraph

**Why GNNs:** The shadow replication constraint requires joint reasoning about a primary placement and a shadow placement. A GNN that propagates information across the graph can learn that two nodes sharing a home network segment have correlated availability (both go offline when the user's router reboots), making them a poor shadow pair despite individually high scores.

**Graph Attention Networks (GAT) for dynamic topology:** As nodes join/leave (every heartbeat cycle, node states update; the 90s timeout marks nodes as unavailable), the GAT's attention weights over neighbors naturally downweight stale or high-latency connections. The attention mechanism is computed as:

```
α_ij = softmax_j( LeakyReLU( a^T [W·h_i || W·h_j] ) )
h'_i = σ( Σ_j α_ij · W·h_j )
```

This produces a node embedding `h'_i` that reflects both the node's own state and its connectivity context in the federation graph — directly encoding the kind of topology-aware reasoning that static scoring cannot capture.

**Message Passing for Congestion Propagation:** When a desktop node becomes saturated (high CPU utilization), message passing propagates a "congestion signal" to neighboring nodes in the graph. Neighboring nodes whose embeddings absorb this signal will see lower placement scores for tasks that would otherwise flow to the congested region. This implements a soft form of backpressure routing without explicit queue management.

**Practical architecture for SoHoLINK:**

```
Input: Node feature matrix X ∈ R^{|N| × 20}, Adjacency A ∈ R^{|N| × |N|}
Layer 1: GAT, 3 attention heads, hidden dim 64 → output dim 48 per node
Layer 2: GAT, 2 attention heads, hidden dim 48 → output dim 32 per node
Task embedding: MLP(task_features) → R^32
Scoring head: MLP( h'_node || task_embedding ) → scalar score per node
Output: Softmax over candidate nodes → placement distribution
```

Total parameters: approximately 80,000. Inference time for 500 nodes: ~2ms on CPU (well within the 5ms budget). The graph is sparse (typical SOHO federation has geographic clustering; each node has O(10) topological neighbors), making sparse GAT efficient.

---

### 2.D Time Series Forecasting and Predictive Scaling

The most immediately actionable ML addition to SoHoLINK is **availability prediction** for mobile nodes.

**Human behavioral patterns:** Mobile-android nodes follow predictable cycles. A smartphone is typically:
- Plugged in and on WiFi between 22:00–07:00 (overnight charging)
- Disconnected from charger during work hours (09:00–17:00)
- Intermittently plugged during evenings (17:00–22:00)

These patterns are learnable from just 7–14 days of heartbeat logs. The `MobileHeartbeat` struct already carries `BatteryPct`, `Plugged`, and `WiFi` at each heartbeat interval.

**LSTM for node availability forecasting:**

```
Input: Multivariate time series per node [battery_pct, plugged, wifi, heartbeat_age] 
       over trailing 24 hours at 90s resolution = 960 time steps × 4 features
Architecture: 2-layer LSTM, hidden=128, dropout=0.2
Output: P(node_available | next_15min, next_60min, next_4hr)
```

This is a sequence-to-scalar multi-horizon forecast. Training data is generated directly from the coordinator's heartbeat log: label each 15/60/240-minute window with 1 (node remained connected) or 0 (node dropped).

**Temporal Fusion Transformer (TFT) for multi-variate telemetry:** For higher accuracy, TFT combines static node metadata (class, historical patterns), past observations (thermal, battery, WiFi RSSI), and known future inputs (time-of-day, day-of-week) to produce calibrated probability forecasts with uncertainty intervals. The uncertainty interval is especially valuable: a task with a 4-hour deadline should only be assigned to a mobile node if the node's predicted availability covers the full execution window with high confidence.

**Thermal headroom degradation forecasting:** Android thermal headroom degrades under sustained load following an approximately exponential decay curve:

```
headroom(t+Δ) ≈ headroom(t) · exp(-λ·load(t)·Δ) + ε

where λ is device-specific (learned from historical data)
```

A simple single-layer LSTM per device class can fit this curve from logs of `(thermal_headroom_t, cpu_load_t, headroom_{t+1})` tuples. The scheduler can then predict: "if I assign a 90-second Wasm segment to this node, what will thermal headroom be at completion?" and reject the assignment if predicted headroom at completion falls below a threshold (e.g., 0.3).

**Workload demand forecasting:** The `PendingQueue` depth over time follows predictable diurnal patterns for a marketplace (tasks submitted during business hours; background batch during off-peak). A 5/15/60-minute demand forecast enables proactive scaling:

```go
// Pseudocode: proactive scale-up integration with AutoScaler
func (a *AutoScaler) ProactiveEvaluate(ctx context.Context, forecast DemandForecast) {
    predicted15m := forecast.PredictedQueueDepth(15 * time.Minute)
    currentCapacity := a.scheduler.totalAvailableCapacity()
    if predicted15m > currentCapacity * 1.2 {
        // Pre-warm: emit scale event now, before demand arrives
        a.scheduler.scalingQueue <- ScaleEvent{
            WorkloadID:     "fleet_preemptive",
            TargetReplicas: computeTargetFromForecast(predicted15m),
        }
    }
}
```

---

### 2.E Anomaly Detection

**Byzantine and underperforming node detection** is critical before HTLC settlement. A node that consistently accepts tasks and then disappears (without returning `MobileTaskResult`) is either genuinely unstable or potentially gaming the system for exploration of HTLC timing attacks.

**Isolation Forest for telemetry anomalies:** Isolation Forest is a tree-based anomaly detector that works in near-real-time and handles the mixed continuous/discrete feature space (thermal, battery, latency, completion rate) naturally. For each node, build an Isolation Forest over its historical telemetry vectors. Nodes whose current telemetry falls into a low-density region (high anomaly score) are flagged for reduced dispatch probability or quarantine.

```
anomaly_score = 2^(-E(h(x)) / c(n))
where E(h(x)) = expected path length in isolation trees, c(n) = normalization
```

**LSTM-Autoencoder for sequential anomalies:** Some gaming behaviors manifest as sequence anomalies rather than point anomalies. A node might show normal individual telemetry readings but an abnormal sequence (e.g., thermal headroom that drops suspiciously fast after a task is assigned, suggesting the node is throttling deliberately to avoid completion). An LSTM-Autoencoder trained on normal behavior has high reconstruction error on anomalous sequences.

```
Encoder: LSTM(input=node_telemetry_sequence, hidden=64) → z ∈ R^32
Decoder: LSTM(z, hidden=64) → reconstructed_sequence
Anomaly score = MSE(input, reconstructed)
```

**Integration with LBTAS:** The current `ApplyPenalty` function in `internal/lbtas/score.go` accepts a point deduction. ML-detected anomalies can feed directly into this:

```go
// Integration hook: anomaly detector → LBTAS penalty
func (d *AnomalyDetector) OnScoreComputed(nodeDID string, score float64) {
    if score > anomalyThreshold {
        severity := int((score - anomalyThreshold) / (1.0 - anomalyThreshold) * 20)
        lbtasManager.ApplyPenalty(ctx, nodeDID, severity)
        log.Printf("[anomaly] applied %d-point LBTAS penalty to %s (score=%.3f)", severity, nodeDID, score)
    }
}
```

**Coordinated gaming attack detection:** A group of colluding nodes might collectively accept tasks at a scheduled time to saturate the coordinator's capacity, then simultaneously drop, forcing costly re-scheduling. Detecting this requires **cross-node correlation analysis**: if the dropout rate across multiple nodes spikes simultaneously beyond Poisson expectation, a coordination event is likely. A simple control chart (CUSUM) on the fleet-wide dropout rate per 5-minute window is sufficient for first-line detection.

---

### 2.F Federated Learning for the Scheduler Itself

The deepest architectural direction is using **Federated Learning (FL)** to train the scheduling policy in a privacy-preserving manner, where each node contributes to improving the scheduler without revealing raw telemetry.

**Local model per node:** Each coordinator node (or node cluster) trains a local mini-model on its own telemetry data — completion rates, thermal curves, availability windows — and contributes gradient updates to the central coordinator. The coordinator aggregates updates via FedAvg:

```
θ_global ← θ_global + (1/|S|) Σ_{k∈S} (θ_k - θ_global)
```

**Class-specific aggregation:** Desktop, Android TV, and Mobile Android nodes have fundamentally different feature distributions. A global model trained on all classes will be dominated by the majority class (desktops). Use **clustered federated learning**: maintain separate global models per class (`θ_desktop`, `θ_atv`, `θ_mobile`), aggregating only within class. This prevents the desktop model from corrupting the mobile model's thermal curve learning.

**Differential Privacy:** Node operators may reasonably object to contributing raw gradient updates that could leak usage patterns. Apply the Gaussian mechanism with `ε = 1.0, δ = 1e-5`:

```
gradient_noised = clip(gradient, C) + Normal(0, σ²·I)
where σ = C · √(2·log(1.25/δ)) / ε
```

**Communication efficiency for mobile nodes:** Mobile Android nodes have constrained uplink bandwidth. Apply gradient compression:
- **Top-k sparsification:** Only upload the top 1% of gradient components by magnitude
- **Quantization:** 8-bit fixed-point quantization of gradients (4× compression vs. float32)

Given typical gradient sizes for a 80K-parameter model (~320KB float32), top-k + quantization brings per-round communication to approximately 800 bytes per mobile node — acceptable over WiFi.

**Practical constraint:** FL for the scheduler requires careful coordination. Gradient contributions from mobile nodes arrive asynchronously (only when the node is online). Use asynchronous FedAvg with a staleness threshold: gradients more than `max_staleness = 3` rounds old are discarded. This naturally bounds gradient variance from nodes that were offline for extended periods.

---

## 3. Feature Engineering for SoHoLINK's Node Dynamics

### 3.1 Node-Level Feature Pipeline

The raw signals available from `MobileHeartbeat` and `Node` structs need transformation before ML use:

| Raw Signal | Transformation | Rationale |
|---|---|---|
| `BatteryPct` (0–100) | `battery_norm = pct/100`; `battery_trend = (pct_t - pct_{t-1}) / Δt` | Trend distinguishes charging from draining |
| `ThermalHeadroom` (0.0–1.0) | Raw + 5-step rolling mean + rate of change | Mean smooths noise; rate detects degradation onset |
| `WiFi` (bool) | Raw + rolling 10-window average | Average encodes connection stability |
| `LatencyMs` | `log(1 + latency)` + z-score normalization | Log-transform reduces skew from CGNAT-induced spikes |
| `ReputationScore` (0–100) | `/100` + `score_velocity` (recent slope) | Velocity detects score trajectories, not just point values |
| `UptimePercent` | Raw; hour-of-day histogram (24-dim) | Histogram encodes behavioral patterns |
| `FailureRate` | Logit transform + node-class z-score | Separate normalization per class avoids mobile/desktop conflation |
| `LastHeartbeat` | `age_seconds = now - LastHeartbeat`; clip at 90s | Direct proxy for staleness of node state |

**Wasm execution speed benchmarking:** The `Image` field in `WorkloadSpec` carries the Wasm CID. The coordinator should maintain a benchmark database: for each (node, wasm_hash) pair that has previously executed, store `(actual_duration_ms, estimated_flops, segment_index)`. This builds a **node × workload compatibility matrix** that contextual bandits and RL models can exploit. Cold entries (new node or new Wasm) fall back to class-level priors.

### 3.2 Workload-Level Features

The `MobileTaskDescriptor` and `WorkloadSpec` provide direct inputs:

```
task_feature_vector = [
  log(len(WasmCID_content)),   // indirect proxy via benchmark DB lookup
  log(estimated_flops),         // from workload profiling
  log(segment_count),
  segment_index / segment_count, // normalized position in sequence
  log(payment_value_sat + 1),
  deadline_slack / max_deadline, // normalized urgency
  requires_shadow_bool,
  arch_is_arm64,
  task_type_onehot(4),          // container | vm | function | service
]
```

### 3.3 System-Level Features

```
system_feature_vector = [
  log(len(PendingQueue) + 1),
  mobile_android_count / total_node_count,
  desktop_count / total_node_count,
  atv_count / total_node_count,
  htlc_cancel_rate_5min,         // recent HTLC cancellations / total dispatches
  shadow_mismatch_rate,           // hash mismatches in last 100 completions
  mean_fleet_cpu_utilization,
  p90_completion_latency_30min,   // proxy for current system health
  time_of_day_sin,                // sin(2π·hour/24) for cyclical encoding
  time_of_day_cos,                // cos(2π·hour/24)
  day_of_week_sin,                // sin(2π·dow/7)
]
```

### 3.4 Concept Drift Detection

Node population in a SOHO federation evolves: new hardware joins, old hardware retires, mobile phones are replaced by newer models with different thermal characteristics. Static models trained on older data will drift.

Use **Page-Hinkley Test** (PHT) on the rolling prediction error of the contextual bandit or RL value function:
```
PHT signals drift when: max_t(Σ_{i≤t} (error_i - μ_min - λ)) > threshold
```

When PHT fires on a node class, trigger an accelerated exploration phase (increase UCB coefficient or entropy bonus) to re-learn that class's distribution. Log the drift event to the accounting system for operator visibility.

---

## 4. Training Infrastructure

### 4.1 Online vs. Offline Training

| Dimension | Online Training | Offline Training |
|---|---|---|
| Data freshness | Immediate; adapts to new nodes instantly | Lags production by training cycle |
| Training stability | Risk of catastrophic forgetting; distribution shift | Stable; can use regularization freely |
| Resource cost | CPU/GPU on coordinator continuously | Batch processing; off-peak scheduling |
| Safety | Direct policy changes during operation | Safe rollout via A/B |
| **Recommendation** | Bandit models (lightweight, low risk) | RL + GNN (complex, high impact) |

The practical approach is a **hybrid pipeline**: offline pre-training on historical logs, with online fine-tuning of lightweight bandit models operating in shadow mode before promotion to primary dispatch.

### 4.2 Simulation Environment

SoHoLINK's existing data provides rich simulation material:

- **HTLC settlement logs** (from `internal/payment/htlc.go` events): settle vs. cancel outcomes with timestamps give a labeled dataset for HTLC probability modeling.
- **Placement logs** (from `store.CreatePlacement`): node × workload × outcome tuples.
- **LBTAS transaction history**: completion quality signals.

**Replay buffer construction:**
```
Event: (timestamp, workload_features, node_features_at_dispatch, node_selected, 
        htlc_settled, htlc_cancel_lag, completion_duration_ms, shadow_matched)
```

Build a simulator that replays these events with counterfactual replacement: "what would have happened if we had dispatched to node B instead of node A?" Use **doubly robust estimation** (DRE) to correct for the propensity of the original heuristic policy when computing counterfactual reward estimates.

### 4.3 Cold Start Strategy

**Phase 0 (Week 1–2):** Add telemetry collection hooks (see Section 5). No ML yet. Build the replay buffer.

**Phase 1 (Week 3–4):** Pre-train on synthetic traces generated from the simulator using the existing heuristic as a behavioral cloning teacher. The cloned policy matches the heuristic baseline.

**Phase 2 (Week 5+):** Fine-tune using offline RL (Conservative Q-Learning / CQL) on the replay buffer. CQL adds a conservatism penalty that prevents the policy from exploiting out-of-distribution actions not represented in the historical data. This is critical: the historical data comes from a round-robin + heuristic policy; a standard offline RL method will generate unrealistically optimistic value estimates for nodes that were rarely selected.

### 4.4 Model Serving: Inference Latency

The `scheduleWorkload` function must return within 5ms to avoid blocking the `PendingQueue`. Model size constraints:

| Model | Parameters | CPU Inference (500 nodes) | Fits Budget? |
|---|---|---|---|
| LinUCB | ~1K | <0.1ms | Yes |
| NeuralLinear (2-layer MLP) | ~50K | ~0.5ms | Yes |
| GAT (3-layer, sparse) | ~80K | ~2ms | Yes |
| Full SAC (MLP critic/actor) | ~500K | ~8ms | Marginal |
| TFT (time series) | ~2M | ~30ms (async) | No (run asynchronously) |

**Architecture:** Serve bandit/GNN models synchronously in the scheduling critical path. Run RL and time series models asynchronously, writing their outputs (pre-computed node rankings, availability forecasts) to an in-memory cache that the synchronous path reads. The synchronous path then resolves from cache with a fallback to the heuristic if the cache is stale.

```go
// Integration pseudocode: ML-augmented placer
type MLPlacer struct {
    heuristic   *Placer        // existing Placer as fallback
    bandits     *BanditModels  // per-class contextual bandits
    nodeCache   *NodeScoreCache // pre-computed GNN scores, refreshed async
    forecaster  *NodeForecaster // availability predictions, refreshed async
}

func (p *MLPlacer) ScoreNodes(nodes []*Node, w *Workload) map[string]float64 {
    ctx, cancel := context.WithTimeout(context.Background(), 4*time.Millisecond)
    defer cancel()
    
    // Try ML scoring first
    if scores, err := p.bandits.Score(ctx, nodes, w); err == nil {
        return scores
    }
    // Fall back to heuristic on timeout or model error
    log.Printf("[mlplacer] falling back to heuristic for workload %s", w.WorkloadID)
    return p.heuristic.ScoreNodes(nodes, w)
}
```

### 4.5 Safe Rolling Updates

Use a **shadow mode** strategy:
1. Deploy new model in shadow mode: it scores nodes but does not affect actual dispatch.
2. Compare shadow scores vs. actual heuristic dispatch outcomes over 48 hours.
3. If shadow model's simulated HTLC settle rate (from counterfactual evaluation) exceeds heuristic by >2%, promote to 10% traffic split (A/B experiment).
4. Ramp: 10% → 25% → 50% → 100% over 2-week intervals, monitoring for regression.

A/B assignment: hash `workload_id` modulo 100. Assignments below the treatment fraction go to ML scheduler; remainder to heuristic. This ensures consistent assignment (same workload always goes to same variant) and avoids confounding from workload mixing.

---

## 5. Specific Recommendations for SoHoLINK (Prioritized)

### Priority 1: Data Collection Infrastructure (Immediate — Week 1)

Before any ML can be trained, telemetry hooks must be added to the existing codebase. The current scheduler emits no structured logs of scheduling decisions and their outcomes.

**Required hooks:**

```go
// Add to FedScheduler: structured event emission
type SchedulingEvent struct {
    Timestamp      time.Time
    WorkloadID     string
    NodeDID        string
    NodeClass      NodeClass
    Score          float64
    ThermalHeadroom float64        // mobile only; 0 for desktop
    BatteryPct     int             // mobile only; -1 for desktop
    Plugged        bool
    WiFi           bool
    LBTASScore     int
    CPUUtilAtDispatch float64
    DispatchLatencyMs int64
}

type OutcomeEvent struct {
    Timestamp      time.Time
    WorkloadID     string
    NodeDID        string
    HTLCSettled    bool
    HTLCCancelledAt *time.Time    // nil if not cancelled
    ShadowMatched  *bool         // nil if not applicable
    CompletionMs   int64
    SegmentIndex   int
}
```

These events should be written to an append-only log (SQLite table or structured JSON log file) that forms the replay buffer for Phase 1 training.

**Expected implementation time:** 2 days. **No ML model required.** This is pure instrumentation.

### Priority 2: Contextual Bandit with Thompson Sampling (Week 3–6)

The contextual bandit is the highest ratio of impact to implementation complexity. It directly addresses the round-robin mobile dispatch problem (the most visible scheduler deficiency) and integrates cleanly with the existing `ScheduleMobile` function.

**Model architecture:** NeuralLinear bandit.
- Feature extractor: 2-layer MLP, input=30, hidden=64, output=32.
- LinUCB head: per-arm `(A, b)` matrices in the 32-dim embedding space.
- Parameters: ~50K total.
- Update frequency: Online, after each HTLC outcome event.

**Integration point in `internal/orchestration/scheduler.go`:**

```go
// Replace the round-robin selection in ScheduleMobile:
// Old:
target := candidates[int(time.Now().UnixNano())%len(candidates)]

// New:
ctx := buildNodeContext(candidates, w, s.systemFeatures())
scores := s.bandit.Score(ctx)
target := selectByThompsonSample(candidates, scores)
```

**Estimated impact:** Based on contextual bandit literature applied to similar dispatch problems (Mao et al., 2019; Decima):
- HTLC cancel rate: -15% to -25% (better node selection reduces mid-task dropout)
- Mean task completion latency: -10% to -20% (thermal-aware dispatch avoids throttled nodes)
- Mobile node utilization improvement: +20% (exploration finds underutilized reliable nodes)

### Priority 3: LSTM Availability Forecaster (Week 6–10)

Implementing the 15-minute and 60-minute availability prediction for mobile nodes. This directly addresses the "node disappears mid-task" problem by refusing to dispatch to nodes with low predicted availability over the task's estimated duration.

**Integration with AutoScaler:** Feed 60-minute demand forecast to `AutoScaler.EvaluateLoop` for proactive pre-warming of desktop capacity before mobile churn periods (morning unplugging).

**Model:** Single LSTM per node class, trained on heartbeat history.
- Input: 24h × 90s = 960-step multivariate time series
- Hidden: 128 units, 2 layers
- Output: P(available, t+15min), P(available, t+60min), P(available, t+4hr)
- Parameters: ~200K per class model (3 models total)

### Priority 4: GAT-Based Topology-Aware Placement (Week 10–16)

The GNN is the most architecturally significant change and requires the most careful integration. It replaces the per-node independent scoring in `Placer.ScoreNodes` with a graph-aware embedding.

**Integration point:** Add a `GraphAwarePlacer` that wraps the existing `Placer`:

```go
type GraphAwarePlacer struct {
    fallback    *Placer
    gat         *GATModel
    graph       *FederationGraph    // maintained by NodeDiscovery updates
    embedCache  map[string][]float32 // node DID → embedding vector
}

func (p *GraphAwarePlacer) ScoreNodes(nodes []*Node, w *Workload) map[string]float64 {
    embeddings := p.gat.Embed(p.graph, nodes)  // ~2ms
    taskEmbed := p.gat.EmbedTask(w)
    scores := make(map[string]float64, len(nodes))
    for _, n := range nodes {
        scores[n.DID] = cosineSimilarity(embeddings[n.DID], taskEmbed) * 100
    }
    return scores
}
```

**Shadow pair joint scoring:** For mobile-android workloads, the GAT jointly scores primary + shadow pairs:

```go
func (p *GraphAwarePlacer) ScoreShadowPair(primary, shadow *Node, w *Workload) float64 {
    e_p := p.embedCache[primary.DID]
    e_s := p.embedCache[shadow.DID]
    // Penalize pairs that share a network segment (correlated failure risk)
    coupling := p.graph.NetworkCoupling(primary.DID, shadow.DID) // 0=independent, 1=same LAN
    pair_score := cosineSimilarity(e_p, e_s) - 0.3*coupling
    return pair_score
}
```

### Priority 5: Anomaly Detection for Byzantine Nodes (Week 12–18)

Isolation Forest deployed per node class, updated weekly from the HTLC outcome log. Anomaly scores feed directly into `lbtas.ApplyPenalty`.

This is a **defensive capability** that becomes more important as the platform grows. Gaming attacks are rare at small scale but scale with marketplace size.

---

## 6. Risk Analysis

### 6.1 Model Misbehavior in Production

**Failure mode:** The ML scheduler consistently assigns tasks to a subset of high-reputation nodes, starving newer nodes and creating a monopoly. Counter-measure: add a minimum exploration fraction (5–10% of dispatches go to random eligible candidates regardless of score), and monitor the Gini coefficient of task distribution across nodes.

**Failure mode:** The bandit exploits a spurious correlation (e.g., nodes in timezone UTC-5 have high historical settle rates due to overnight scheduling, not inherent capability). Counter-measure: include time-of-day features explicitly so the model learns the confound rather than attributing it to node identity.

**Fallback guarantee:** The `MLPlacer` wrapper must always fall back to the existing `Placer.ScoreNodes` heuristic within the 5ms inference timeout. This is not optional: a scheduler deadlock due to ML model failure would block the entire `PendingQueue`. Instrument every ML call with a deadline context and a fallback counter metric.

### 6.2 Gaming and Adversarial Nodes

Sophisticated node operators could attempt to **game the bandit** by providing manipulated telemetry. For example, a mobile node could report `thermalHeadroom = 0.95` at registration time, accumulating high dispatch rates, then deliver corrupted results that still pass a naive hash check.

Counter-measures:
- **Shadow verification as ground truth:** The HTLC settlement is gated on shadow hash match, which is independent of node-reported telemetry. A node that delivers corrupted results will see HTLCs cancelled regardless of its self-reported state, which is then reflected in the bandit's reward signal.
- **Telemetry attestation:** Consider requiring mobile nodes to sign their heartbeat data with their DID key. This prevents replay attacks (a node replaying high-headroom readings from a past session).
- **Cross-node correlation anomaly detection (Section 2.E):** Coordinated manipulation by multiple colluding nodes is detectable via dropout correlation analysis.

### 6.3 Privacy Concerns

The feature engineering pipeline collects granular temporal patterns of user behavior (charging cycles, WiFi presence patterns). These are **behavioral biometrics** and may be subject to GDPR Article 9 (sensitive data) arguments in some EU jurisdictions.

**Mitigations:**
- Store telemetry only in aggregate (per-node, not per-user), with retention limits (30-day rolling window).
- Apply differential privacy to any telemetry that leaves the coordinator (e.g., when sharing with the federated learning aggregator).
- Provide node operators with a telemetry opt-out that falls back to the heuristic scheduler for their nodes (accepting lower dispatch priority as the tradeoff).
- Include explicit consent in the wizard onboarding flow (already implemented in `internal/wizard/`) for telemetry collection, with a machine-readable record stored in `configs/policies/resource_sharing.rego`.

### 6.4 Regulatory Considerations

- **GDPR (EU nodes):** Data minimization applies. Collect only the features listed in Section 3; do not collect IP addresses, precise geolocation, or user identity. The LBTAS DID is already a pseudonymous identifier. Ensure right-to-erasure: deleting a node's DID from the LBTAS store must also purge it from the bandit's reward history and the GNN's node embeddings.
- **Data residency:** If EU nodes' telemetry flows to a US-based coordinator, GDPR Chapter V (international transfers) applies. Consider running a EU-regional coordinator shard for EU nodes.
- **CCPA (California nodes):** Similar data minimization requirements; node operators in California have right to know what telemetry is collected.

---

## 7. Related Work and Precedents

### 7.1 Kubernetes Scheduling Extensions

The Kubernetes community has explored ML-augmented scheduling through several mechanisms:

**Trimaran (bin-packing scheduler):** Uses real-time CPU utilization (not just requested) for bin-packing. The analogue for SoHoLINK is using measured throughput rather than requested CPU cores for node selection. SoHoLINK's `scoreCapacity` function already has the structure; it needs real-time utilization as input rather than static `AvailableCPU`.

**Scheduler Extender:** K8s provides a webhook interface where an external HTTP server can influence placement decisions. SoHoLINK's architecture could adopt the same pattern: the `Placer` calls an internal HTTP endpoint backed by the ML model, with a timeout fallback to heuristic scoring. This decouples the ML model lifecycle from the Go scheduler binary.

**Descheduler:** Kubernetes' descheduler re-evaluates placements and evicts pods that have drifted from optimal placement. The SoHoLINK analogue is `PreemptMobileWorkload` — it already handles mid-task preemption. An ML-driven descheduler could proactively migrate tasks from nodes showing early thermal degradation before they drop.

### 7.2 Alibaba Optimus and Google Borg ML Extensions

Alibaba's **Optimus** (OSDI 2018) applies an RL-based approach to deep learning job scheduling, dynamically adjusting resource allocation as a job's requirements change during training. The key insight applicable to SoHoLINK: resource requirements for Wasm tasks are not static; thermal throttling on Android nodes effectively reduces available compute mid-task. Optimus' dynamic re-allocation strategy maps to SoHoLINK's `handleScaleEvent` + `PreemptMobileWorkload` pipeline.

Google's **Borg** has incorporated ML for capacity planning (predicting job resource requirements from historical usage), not for per-placement decisions. The lesson: ML for prediction (demand forecasting, resource estimation) is more mature and lower-risk than ML for online decision-making (placement selection). This supports the phased approach in Section 5.

### 7.3 Decima (MIT) — RL for DAG Scheduling

Mao et al.'s **Decima** (SIGCOMM 2019) is the most directly relevant academic precedent. Decima trains an RL agent using a graph neural network to schedule DAG-structured jobs in a Spark cluster. The key contributions applicable to SoHoLINK:

- **Graph embedding for heterogeneous topology:** Decima represents each job as a DAG and uses GNN message passing to embed both the job structure and the cluster state. SoHoLINK's federation graph (nodes as vertices, latency as edges, tasks as hyperedges) is structurally analogous.
- **Pointer-network action selection:** Decima uses a softmax over graph-embedded job nodes to select the next task to schedule, handling variable-length action spaces. SoHoLINK needs the same for variable-size node populations.
- **Improvement over heuristics:** Decima demonstrated 21% mean job completion time reduction over the best heuristic baseline (Tetris) on production Alibaba traces.

### 7.4 DeepMind Data Center Cooling

DeepMind's application of RL to Google data center cooling (Evan et al., 2018) demonstrates that RL can optimize multi-objective control problems (cooling efficiency vs. thermal safety vs. energy cost) in production systems with delayed, sparse rewards. The data center cooling problem has structural parallels to SoHoLINK's thermal management:

- **Delayed reward:** Cooling decisions take minutes to affect server temperatures, just as HTLC settlement is delayed by verification lag.
- **Safety constraints:** The data center uses constraint satisfaction (minimum/maximum temperature bounds) as hard stops that override the RL policy. SoHoLINK should implement analogous hard stops: if a node's thermal headroom drops below 0.2, force preemption regardless of ML recommendation.
- **Human oversight loop:** DeepMind maintained operator override capability throughout production deployment. SoHoLINK's ML scheduler should always expose a "force heuristic mode" operator flag.

### 7.5 Flower Framework for Federated Learning

The **Flower (flwr)** framework provides a production-ready implementation of federated learning that supports asynchronous client updates, differential privacy, and gradient compression. For SoHoLINK's FL-based scheduler (Section 2.F), Flower's `flwr.server.strategy.FedAvg` with custom client selection (only aggregate from nodes that have been online for the current round) provides an off-the-shelf aggregation strategy.

Flower's client library is available in Python, Java, and C++. A Go gRPC bridge would be needed to integrate with SoHoLINK's Go coordinator. Alternatively, implement a simplified FedAvg aggregation directly in Go using `github.com/gorgonia/gorgonia` for tensor operations.

### 7.6 Multi-Agent Competitive Environments (OpenAI)

OpenAI's work on multi-agent competition (e.g., hide-and-seek, multi-agent particle environments) demonstrated that **competitive self-play** drives emergent strategies that exceed any fixed heuristic. In SoHoLINK's context, a simulation where **node agents** try to maximize earnings by strategically accepting and dropping tasks while a **scheduler agent** tries to maximize completion rate creates an adversarial training dynamic. The resulting scheduler policy is robustified against the gaming strategies that real-world nodes will attempt. This is a long-term research direction (Phase 3+) rather than an immediate implementation priority.

---

## Conclusion

SoHoLINK's scheduling problem sits at the intersection of three well-studied but individually hard domains: heterogeneous compute federation, stochastic resource management, and payment-gated verification. The static, linear-weighted heuristic currently in `Placer.ScoreNodes` is a reasonable starting point but will degrade as the federation grows and node population diversifies.

The recommended implementation sequence:

1. **Immediately:** Add structured telemetry instrumentation — zero ML, pure data collection.
2. **Month 1–2:** Deploy NeuralLinear contextual bandit for mobile dispatch, replacing round-robin. Highest impact-to-effort ratio.
3. **Month 2–3:** LSTM availability forecaster for mobile nodes. Directly reduces mid-task preemption costs.
4. **Month 3–5:** GAT-based topology-aware placer, including joint shadow pair scoring.
5. **Month 4–6:** Anomaly detection pipeline feeding LBTAS penalty system.
6. **Ongoing:** Federated learning for privacy-preserving model improvement as the node population grows internationally.

The key architectural invariant throughout: **every ML component must have a synchronous fallback to the existing heuristic**, enforced by a hard inference timeout. The federation's economic integrity depends on the scheduler never deadlocking, regardless of ML model health.

---

*Report prepared for the SoHoLINK project. Module: `github.com/NetworkTheoryAppliedResearchInstitute/soholink`. Architecture version as of 2026-03-02.*