# dgl-desktop-hub

A desktop relay-hub app that connects a DG-LAB Coyote v3 and lets other apps control it.
It relays N-to-N connections and performs protocol conversion.

> [!NOTE]
> This is an unofficial app and is not affiliated with or endorsed by dungeon-lab.

## Overview

```
 App control side (OpenDGLab-OpenProtocol)        Output device side
 ┌─────────────────────────┐            ┌──────────────────────────┐
 │ Connect/gRPC/gRPC-Web   │            │ BLE Coyote v3 (direct)    │
 │                         │  ──Hub──▶  │ Socket mode (LAN phone)   │
 │                         │            │ Demo device (graph only)  │
 └─────────────────────────┘            └──────────────────────────┘
```

- **App control side**: Implements [OpenDGLab-OpenProtocol](https://github.com/OpenDGLab/OpenDGLab-OpenProtocol) with `connectrpc.com/connect`.
- **Output device side**:
  - **BLE Coyote v3**: Direct connection over the PC's Bluetooth.
  - **Socket mode**: The hub runs a WebSocket server, and the DG-LAB app (smartphone) scans a QR code to connect.
  - **Demo device**: No real output. For graph display and behavior testing.
- **Soft limit**: A per-device, per-channel intensity cap. The hub always clamps so the cap is never exceeded. Values are restored after a restart (identified by the BLE stable ID).
- **Graph**: Displays per-channel intensity as a bar graph along a time axis (flowing to the left).
- **Waveform presets**: 16 built-in presets derived from DG-LAB-OPENSOURCE.

## Architecture (packages)

| Package | Role |
|---|---|
| `internal/waveform` | Internal waveform representation (Quad), V3 hex / V2 / frequency conversion, 16 presets (embedded) |
| `internal/device` | `Device` (common base) / `CoyoteDevice` (output) interfaces, `DeviceManager`, 100ms ticker, soft limit, DemoDevice |
| `internal/coyote/ble` | BLE Coyote v3 (scan / connect / B0 / BF / B1 / battery) |
| `internal/socketserver` | Socket-mode WebSocket server + bind FSM + heartbeat + SocketCoyote |
| `internal/appproto` | OpenDGLab Connect server |
| `internal/hubcore` | Mediation between app control side and output side, access control, push notifications |
| `internal/pb` | Go code generated from proto |
| `services` | Services exposed to the Wails frontend (the only package that imports Wails) |

## Development

Prerequisites: Go 1.25+, Node 24 + pnpm, [Task](https://taskfile.dev), the `wails3` CLI, the `buf` CLI.

```sh
task dev          # Development mode (wails3 dev + vite)
task build        # Production build (executable in bin/)
task gen:proto    # Regenerate Go code from proto (buf generate)
task gen:bindings # Regenerate Wails TS bindings
go test ./internal/...   # Unit / E2E tests
```

The app-control-side server listens on `0.0.0.0:7330` at startup
(Connect/gRPC/gRPC-Web: `http://<ip>:7330`).
