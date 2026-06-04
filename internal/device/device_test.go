package device

import (
	"testing"

	"github.com/hantabaru1014/dgl-desktop-hub/internal/waveform"
)

func TestSoftLimitClamp(t *testing.T) {
	m := NewDeviceManager(Hooks{})
	d := NewDemoDevice("t")
	id := m.Add(d)

	if err := m.SetSoftLimit(id, SoftLimit{A: 30, B: 50}); err != nil {
		t.Fatal(err)
	}
	// 目標 100 だがソフトリミット 30 で頭打ちになるはず。
	if err := m.SetStrength(id, ChannelA, StrengthAbsolute, 100); err != nil {
		t.Fatal(err)
	}
	if err := m.SetStrength(id, ChannelB, StrengthAbsolute, 100); err != nil {
		t.Fatal(err)
	}

	m.tickAll() // 1 周期実行

	cmd := d.LastCommand()
	if cmd.StrengthA != 30 {
		t.Errorf("StrengthA = %d, want 30 (clamped)", cmd.StrengthA)
	}
	if cmd.StrengthB != 50 {
		t.Errorf("StrengthB = %d, want 50 (clamped)", cmd.StrengthB)
	}
}

func TestRelativeStrength(t *testing.T) {
	m := NewDeviceManager(Hooks{})
	d := NewDemoDevice("t")
	id := m.Add(d)
	_ = m.SetSoftLimit(id, SoftLimit{A: 200, B: 200})

	_ = m.SetStrength(id, ChannelA, StrengthAbsolute, 10)
	_ = m.SetStrength(id, ChannelA, StrengthRelativeInc, 5)
	_ = m.SetStrength(id, ChannelA, StrengthRelativeInc, 5)
	_ = m.SetStrength(id, ChannelA, StrengthRelativeDec, 3)

	ta, _, _, _, _, err := m.Snapshot(id)
	if err != nil {
		t.Fatal(err)
	}
	if ta != 17 {
		t.Errorf("target A = %d, want 17", ta)
	}
}

func TestWaveformPlayback(t *testing.T) {
	m := NewDeviceManager(Hooks{})
	d := NewDemoDevice("t")
	id := m.Add(d)
	_ = m.SetSoftLimit(id, SoftLimit{A: 200, B: 200})
	_ = m.SetStrength(id, ChannelA, StrengthAbsolute, 100)

	// 2 フレームの波形: 強度 0 -> 強度 100
	w := waveform.Waveform{Name: "test", Frames: []waveform.Quad{
		{Freq: [4]uint8{10, 10, 10, 10}, Intensity: [4]uint8{0, 0, 0, 0}},
		{Freq: [4]uint8{10, 10, 10, 10}, Intensity: [4]uint8{100, 100, 100, 100}},
	}}
	_ = m.SetWaveform(id, ChannelA, w)

	m.tickAll()
	if got := d.LastCommand().QuadA.MeanIntensity(); got != 0 {
		t.Errorf("frame0 amp = %d, want 0", got)
	}
	m.tickAll()
	if got := d.LastCommand().QuadA.MeanIntensity(); got != 100 {
		t.Errorf("frame1 amp = %d, want 100", got)
	}
	m.tickAll() // ループして先頭へ
	if got := d.LastCommand().QuadA.MeanIntensity(); got != 0 {
		t.Errorf("frame2 (looped) amp = %d, want 0", got)
	}
}

func TestGraphHook(t *testing.T) {
	var got []GraphSample
	m := NewDeviceManager(Hooks{OnGraph: func(s []GraphSample) { got = s }})
	d := NewDemoDevice("t")
	id := m.Add(d)
	_ = m.SetSoftLimit(id, SoftLimit{A: 200, B: 200})
	_ = m.SetStrength(id, ChannelA, StrengthAbsolute, 50)
	w := waveform.Waveform{Name: "x", Frames: []waveform.Quad{
		{Freq: [4]uint8{10, 10, 10, 10}, Intensity: [4]uint8{100, 100, 100, 100}},
	}}
	_ = m.SetWaveform(id, ChannelA, w)

	m.tickAll()
	if len(got) != 1 {
		t.Fatalf("samples = %d, want 1", len(got))
	}
	if got[0].DeviceID != id {
		t.Errorf("deviceId = %s, want %s", got[0].DeviceID, id)
	}
	// 実効出力 = 50 * 100 / 100 = 50
	if got[0].A.Output != 50 {
		t.Errorf("A.Output = %d, want 50", got[0].A.Output)
	}
}

func TestRemoveDevice(t *testing.T) {
	m := NewDeviceManager(Hooks{})
	d := NewDemoDevice("t")
	id := m.Add(d)
	if !m.Has(id) {
		t.Fatal("device should exist")
	}
	if err := m.Remove(id); err != nil {
		t.Fatal(err)
	}
	if m.Has(id) {
		t.Error("device should be removed")
	}
}
