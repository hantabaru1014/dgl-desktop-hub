package device

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
)

var demoCounter atomic.Uint64

// DemoDevice はどこにも出力しない疑似デバイス。グラフ表示と動作テストに使う。
// ticker からの OutputCommand は記録するだけ。常に接続状態。
type DemoDevice struct {
	id   DeviceID
	name string

	mu       sync.Mutex
	last     OutputCommand
	report   func(a, b uint8)
}

// NewDemoDevice は新しい疑似デバイスを生成する。
func NewDemoDevice(name string) *DemoDevice {
	n := demoCounter.Add(1)
	if name == "" {
		name = fmt.Sprintf("Demo Device %d", n)
	}
	return &DemoDevice{
		id:   DeviceID(fmt.Sprintf("demo-%d", n)),
		name: name,
	}
}

func (d *DemoDevice) ID() DeviceID { return d.id }

func (d *DemoDevice) Info() Info {
	return Info{
		ID:      d.id,
		Kind:    KindDemo,
		Name:    d.name,
		Status:  StatusConnected,
		Battery: BatteryUnknown,
	}
}

func (d *DemoDevice) Connect(context.Context) error { return nil }
func (d *DemoDevice) Disconnect() error             { return nil }
func (d *DemoDevice) Close() error                  { return nil }

// Output は指令を記録し、報告コールバックへ「実強度=指令強度」を返す。
func (d *DemoDevice) Output(cmd OutputCommand) error {
	d.mu.Lock()
	d.last = cmd
	report := d.report
	d.mu.Unlock()
	if report != nil {
		report(cmd.StrengthA, cmd.StrengthB)
	}
	return nil
}

func (d *DemoDevice) OnStrengthReport(fn func(a, b uint8)) {
	d.mu.Lock()
	d.report = fn
	d.mu.Unlock()
}

// LastCommand は直近に適用された指令を返す (テスト/introspection 用)。
func (d *DemoDevice) LastCommand() OutputCommand {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.last
}
