// Package device は出力デバイス (Coyote) の抽象化と、それらを束ねて
// 100ms 周期で駆動する DeviceManager を提供する。
//
// Device は全デバイス共通の基底 interface。将来追加予定の PawSrints
// (爪印配件 = 入力系アクセサリ) も Device を満たす想定で、出力系の
// Coyote は CoyoteDevice (Device + 出力メソッド) として分けている。
package device

import (
	"context"

	"github.com/hantabaru1014/dgl-desktop-hub/internal/waveform"
)

// DeviceID はデバイスの一意な識別子。
type DeviceID string

// Kind はデバイス種別。
type Kind int

const (
	KindDemo   Kind = iota // 疑似デバイス (実出力なし)
	KindBLE                // BLE 直接接続の Coyote v3
	KindSocket             // socket mode (LAN 内スマホ経由) の Coyote
)

func (k Kind) String() string {
	switch k {
	case KindDemo:
		return "demo"
	case KindBLE:
		return "ble"
	case KindSocket:
		return "socket"
	default:
		return "unknown"
	}
}

// Channel はチャンネル A / B。
type Channel int

const (
	ChannelA Channel = 0
	ChannelB Channel = 1
)

// Status は接続状態。
type Status int

const (
	StatusDisconnected Status = iota
	StatusConnecting
	StatusConnected
)

func (s Status) String() string {
	switch s {
	case StatusConnecting:
		return "connecting"
	case StatusConnected:
		return "connected"
	default:
		return "disconnected"
	}
}

// BatteryUnknown は電池残量不明を表す値。
const BatteryUnknown uint8 = 255

// Info はデバイスの表示用情報のスナップショット。
type Info struct {
	ID      DeviceID
	Kind    Kind
	Name    string
	Status  Status
	Battery uint8 // 0..100、不明なら BatteryUnknown
}

// StrengthMode は強度設定の解釈方法。
type StrengthMode int

const (
	StrengthAbsolute    StrengthMode = iota // 絶対値に設定
	StrengthRelativeInc                     // 相対増加
	StrengthRelativeDec                     // 相対減少
)

// OutputCommand は ticker が 100ms ごとに各デバイスへ渡す出力指令。
// 強度はソフトリミット適用済みの絶対値。各デバイス実装は前回送信値との
// 差分を取り、プロトコルに応じた実出力 (B0 / pulse / 記録) を行う。
type OutputCommand struct {
	StrengthA uint8 // 0..200 (clamp 済み)
	StrengthB uint8 // 0..200 (clamp 済み)
	QuadA     waveform.Quad
	QuadB     waveform.Quad
}

// Device は全デバイス共通の基底 interface。
type Device interface {
	ID() DeviceID
	Info() Info
	// Connect は接続を確立する。既に接続済みなら何もしない。
	Connect(ctx context.Context) error
	// Disconnect は接続を切る。再接続可能な状態にする。
	Disconnect() error
	// Close はリソースを解放する。以降このインスタンスは利用不可。
	Close() error
}

// CoyoteDevice は出力 (脈衝) を行う Coyote デバイスの interface。
type CoyoteDevice interface {
	Device
	// Output は 1 周期分 (100ms) の出力指令を適用する。ticker から呼ばれ、
	// ブロックしてはならない (内部でバッファリング/drop すること)。
	Output(cmd OutputCommand) error
	// OnStrengthReport はデバイスが実強度を報告した際 (BLE の B1 / socket の
	// strength 報告) に呼ばれるコールバックを登録する。
	OnStrengthReport(fn func(a, b uint8))
}

// SoftLimitAware は、デバイス側にもソフトリミットを反映できるデバイスが
// 実装する任意 interface (BLE Coyote は BF 指令を送る)。
// ハブ側 clamp が常に真であり、これは追加の安全策。
type SoftLimitAware interface {
	ApplySoftLimit(a, b uint8) error
}
