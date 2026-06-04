# dgl-desktop-hub

DG-Lab Coyote v3 を接続し、他アプリから操作できる中継ハブのデスクトップアプリ。
N対Nの接続を中継し、プロトコル変換を行う。

## 概要

```
 アプリ操作側 (OpenDGLab-OpenProtocol)            出力デバイス側
 ┌─────────────────────────┐            ┌──────────────────────────┐
 │ Connect/gRPC/gRPC-Web   │            │ BLE Coyote v3 (直接接続)  │
 │                         │  ──Hub──▶  │ Socket mode (LAN スマホ)  │
 │                         │            │ Demo device (グラフのみ)  │
 └─────────────────────────┘            └──────────────────────────┘
```

- **アプリ操作側**: [OpenDGLab-OpenProtocol](https://github.com/OpenDGLab/OpenDGLab-OpenProtocol) を `connectrpc.com/connect` で実装。
- **出力デバイス側**:
  - **BLE Coyote v3**: PCのbluetoothで直接接続。
  - **Socket mode**: ハブが WebSocket サーバを立て、DG-Lab アプリ (スマホ) が QR をスキャンして接続。
  - **Demo device**: 実出力なし。グラフ表示と動作テスト用。
- **ソフトリミット**: デバイス・チャンネル毎に強度上限を設定。ハブ側で必ず clamp し、上限を超えない。値は再起動後も復元 (BLE の安定 ID で同定)。
- **グラフ**: チャンネル毎に強度を時間軸 (左へ流れる) の棒グラフで表示。
- **波形プリセット**: DG-LAB-OPENSOURCE 由来の 16 種を内蔵。

## アーキテクチャ (パッケージ)

| パッケージ | 役割 |
|---|---|
| `internal/waveform` | 波形の内部表現 (Quad) と V3 hex / V2 / 周波数換算、16 プリセット (埋め込み) |
| `internal/device` | `Device`(共通基底) / `CoyoteDevice`(出力) interface、`DeviceManager`、100ms ticker、ソフトリミット、DemoDevice |
| `internal/coyote/ble` | BLE Coyote v3 (scan / connect / B0 / BF / B1 / battery) |
| `internal/socketserver` | socket mode の WebSocket サーバ + bind FSM + heartbeat + SocketCoyote |
| `internal/appproto` | OpenDGLab の Connect サーバ |
| `internal/hubcore` | アプリ操作側と出力側の仲介、アクセス制御、push 通知 |
| `internal/pb` | proto から生成した Go コード |
| `services` | Wails フロントエンドへ公開するサービス (Wails を import する唯一のパッケージ) |

> 将来 `DG-LAB-OPENSOURCE/PawPrints` (爪印配件 = 入力系) を `device.Device` を満たす
> `Accessory` として追加できるよう、出力系 `CoyoteDevice` と基底 `Device` を分離している。

## 開発

前提: Go 1.25+, Node 24 + pnpm, [Task](https://taskfile.dev), `wails3` CLI, `buf` CLI。

```sh
task dev          # 開発モード (wails3 dev + vite)
task build        # 本番ビルド (bin/ に実行ファイル)
task gen:proto    # proto から Go コード再生成 (buf generate)
task gen:bindings # Wails TS バインディング再生成
go test ./internal/...   # 単体・E2E テスト
```

アプリ操作側サーバは起動時に `0.0.0.0:7330` で待ち受ける
(Connect/gRPC/gRPC-Web: `http://<ip>:7330`)。
