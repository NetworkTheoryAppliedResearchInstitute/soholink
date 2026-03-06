# SoHoLINK Node Dashboard — Flutter App

Native mobile app for monitoring and managing your SoHoLINK federated edge node.
Built in pure Flutter/Dart — no visual editors required.

## Features

| Tab | Description |
|-----|-------------|
| **Dashboard** | Node health, uptime, active rentals, and resource gauges |
| **Peers** | LAN-mesh and mobile federation peers with latency and status |
| **Revenue** | Daily earnings in sats, 30-day bar chart, fee breakdown |
| **Workloads** | Active and recent workloads with resource and earnings summary |
| **Settings** | Node URL configuration and connection test |

## Prerequisites

| Tool | Min Version | Install |
|------|-------------|---------|
| Flutter SDK | 3.19.0 | https://flutter.dev/docs/get-started/install |
| Dart SDK | 3.3.0 | Bundled with Flutter |
| Android SDK | API 21+ | Via Android Studio or `sdkmanager` |
| Running `fedaaa` node | any | `go run ./cmd/fedaaa start` |

## Quick Start

```bash
# 1. Navigate to the app directory
cd mobile/flutter-app

# 2. Install dependencies
flutter pub get

# 3. Connect a device / start emulator, then run
flutter run

# Build release APK
flutter build apk --release
```

## Connecting to Your Node

On first launch the **Setup** screen appears. Enter your node's URL:

| Environment | URL |
|---|---|
| Android emulator | `http://10.0.2.2:8080` |
| Same Wi-Fi | `http://192.168.1.<your-ip>:8080` |
| USB-tethered phone | `http://192.168.42.1:8080` (typical) |

The app sends a `GET /api/health` ping to validate the connection before saving.

## Project Layout

```
lib/
  main.dart                   ← Entry point; chooses Setup or Home
  theme/
    app_theme.dart            ← Dark theme, brand colours
  api/
    soholink_client.dart      ← HTTP client singleton
  models/
    node_status.dart          ← /api/status DTO
    peer_info.dart            ← /api/peers DTO
    revenue.dart              ← /api/revenue DTO
    workload.dart             ← /api/workloads DTO
  widgets/
    stat_card.dart            ← Metric tile
    resource_bar.dart         ← CPU/RAM/disk/net progress bar
    section_header.dart       ← Labelled section divider
    status_dot.dart           ← Animated health indicator
  pages/
    setup_page.dart           ← First-run URL entry
    home_page.dart            ← NavigationBar shell
    dashboard_page.dart       ← Overview tab
    peers_page.dart           ← Federation peers tab
    revenue_page.dart         ← Earnings + chart tab
    workloads_page.dart       ← Active workloads tab
    settings_page.dart        ← Node URL + about tab
android/
  app/src/main/
    AndroidManifest.xml       ← INTERNET + cleartext LAN permission
    res/xml/
      network_security_config.xml ← Allows HTTP to RFC-1918 addresses
```

## API Endpoints Consumed

| Method | Path | Used by |
|--------|------|---------|
| `GET` | `/api/health` | Setup validation, Settings test |
| `GET` | `/api/status` | Dashboard |
| `GET` | `/api/peers` | Peers |
| `GET` | `/api/revenue` | Revenue |
| `GET` | `/api/workloads` | Workloads |

> **Note**: `/api/revenue` and `/api/workloads` endpoints need to be added to
> the Go `httpapi` package. The app degrades gracefully (empty state) when these
> return 404 until the backend is wired.

## Adding Revenue & Workloads Endpoints (Go backend)

Wire these two routes in `internal/httpapi/server.go`:

```go
mux.HandleFunc("/api/revenue",   s.handleRevenue)
mux.HandleFunc("/api/workloads", s.handleWorkloads)
```

Implement `handleRevenue` and `handleWorkloads` in a new file, e.g.
`internal/httpapi/revenue.go` and `internal/httpapi/workloads.go`, following
the same pattern as `handleStatus` in `dashboard.go`.

## Android Release Build

```bash
# Generate a keystore (once)
keytool -genkey -v -keystore soholink.jks -keyAlg RSA -keySize 2048 \
        -validity 10000 -alias soholink

# Build signed APK
flutter build apk --release

# Install on connected device
flutter install
```

## iOS (future)

Run `flutter build ios` from macOS with Xcode installed. No additional
network security changes are needed — ATS allows LAN addresses by default
when the `NSAllowsLocalNetworking` key is set in `Info.plist`.
