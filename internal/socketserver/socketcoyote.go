package socketserver

import (
	"context"
	"sync"

	"github.com/hantabaru1014/dgl-desktop-hub/internal/device"
	"github.com/hantabaru1014/dgl-desktop-hub/internal/waveform"
)

// SocketCoyote は socket mode で接続したスマホ上の Coyote を表す
// device.CoyoteDevice 実装。出力指令を WebSocket メッセージへ変換して送る。
type SocketCoyote struct {
	id           device.DeviceID
	name         string
	controllerID string
	phoneID      string

	send  func(Envelope) error
	close func() error

	mu       sync.Mutex
	status   device.Status
	battery  uint8
	report   func(a, b uint8)
	lastA    uint8
	lastB    uint8
	haveLast bool
}

func newSocketCoyote(controllerID, phoneID string, send func(Envelope) error, closeFn func() error) *SocketCoyote {
	return &SocketCoyote{
		id:           device.DeviceID("socket-" + phoneID),
		name:         "Socket Coyote " + shortID(phoneID),
		controllerID: controllerID,
		phoneID:      phoneID,
		send:         send,
		close:        closeFn,
		status:       device.StatusConnected,
		battery:      device.BatteryUnknown,
	}
}

func (c *SocketCoyote) ID() device.DeviceID { return c.id }

func (c *SocketCoyote) Info() device.Info {
	c.mu.Lock()
	defer c.mu.Unlock()
	return device.Info{
		ID:      c.id,
		Kind:    device.KindSocket,
		Name:    c.name,
		Status:  c.status,
		Battery: c.battery,
	}
}

func (c *SocketCoyote) Connect(context.Context) error { return nil }

func (c *SocketCoyote) Disconnect() error {
	c.mu.Lock()
	c.status = device.StatusDisconnected
	cl := c.close
	c.mu.Unlock()
	if cl != nil {
		return cl()
	}
	return nil
}

func (c *SocketCoyote) Close() error { return c.Disconnect() }

// Output は強度差分と波形を WebSocket メッセージとしてスマホへ送る。
func (c *SocketCoyote) Output(cmd device.OutputCommand) error {
	c.mu.Lock()
	sendStrengthA := !c.haveLast || cmd.StrengthA != c.lastA
	sendStrengthB := !c.haveLast || cmd.StrengthB != c.lastB
	c.lastA, c.lastB = cmd.StrengthA, cmd.StrengthB
	c.haveLast = true
	send := c.send
	controllerID, phoneID := c.controllerID, c.phoneID
	c.mu.Unlock()

	emit := func(msg string) {
		_ = send(Envelope{Type: "msg", ClientID: controllerID, TargetID: phoneID, Message: msg})
	}

	// 強度 (絶対値) を変化時のみ送る。ch:1=A,2=B mode:2=絶対。
	if sendStrengthA {
		emit(StrengthCommand(1, 2, int(cmd.StrengthA)))
	}
	if sendStrengthB {
		emit(StrengthCommand(2, 2, int(cmd.StrengthB)))
	}
	// 波形は毎周期送る (100ms 分 = 1 hex フレーム)。
	emit(PulseCommand("A", []string{waveform.QuadToHex(cmd.QuadA)}))
	emit(PulseCommand("B", []string{waveform.QuadToHex(cmd.QuadB)}))
	return nil
}

func (c *SocketCoyote) OnStrengthReport(fn func(a, b uint8)) {
	c.mu.Lock()
	c.report = fn
	c.mu.Unlock()
}

// onFeedback はアプリからの強度報告を反映する。
func (c *SocketCoyote) onFeedback(fb StrengthFeedback) {
	c.mu.Lock()
	report := c.report
	c.mu.Unlock()
	if report != nil {
		report(device.ClampStrength(fb.A), device.ClampStrength(fb.B))
	}
}

func shortID(s string) string {
	if len(s) > 6 {
		return s[:6]
	}
	return s
}
