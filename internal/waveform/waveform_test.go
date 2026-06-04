package waveform

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestPresetsLoad は 16 プリセットが読み込めることを確認する。
func TestPresetsLoad(t *testing.T) {
	ps, err := Presets()
	if err != nil {
		t.Fatalf("Presets() error: %v", err)
	}
	if len(ps) != 16 {
		t.Fatalf("preset count = %d, want 16", len(ps))
	}
	for _, p := range ps {
		if p.Name == "" {
			t.Errorf("preset has empty name")
		}
		if p.Waveform.Len() == 0 {
			t.Errorf("preset %q has no frames", p.Name)
		}
	}
}

// TestHexRoundTrip は全プリセットの expectedV3 hex が
// HexToQuad -> QuadToHex で完全に一致する (gold-master) ことを確認する。
func TestHexRoundTrip(t *testing.T) {
	var raws []presetRaw
	if err := json.Unmarshal(presetsJSON, &raws); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, r := range raws {
		for i, h := range r.ExpectedV3 {
			want := strings.ToUpper(strings.TrimSpace(h))
			q, err := HexToQuad(h)
			if err != nil {
				t.Fatalf("preset %q frame %d: HexToQuad(%q): %v", r.Name, i, h, err)
			}
			got := QuadToHex(q)
			if got != want {
				t.Errorf("preset %q frame %d: round-trip = %q, want %q", r.Name, i, got, want)
			}
		}
	}
}

// TestHexToQuadFields は既知のフレームの分解が正しいことを確認する。
func TestHexToQuadFields(t *testing.T) {
	q, err := HexToQuad("0A0A0A0A14141414")
	if err != nil {
		t.Fatal(err)
	}
	if q.Freq != [4]uint8{10, 10, 10, 10} {
		t.Errorf("Freq = %v, want all 10", q.Freq)
	}
	if q.Intensity != [4]uint8{20, 20, 20, 20} {
		t.Errorf("Intensity = %v, want all 20", q.Intensity)
	}
	if q.MeanIntensity() != 20 {
		t.Errorf("MeanIntensity = %d, want 20", q.MeanIntensity())
	}
}

func TestHexToQuadErrors(t *testing.T) {
	if _, err := HexToQuad("0A0A"); err == nil {
		t.Error("expected error for short hex")
	}
	if _, err := HexToQuad("zzzzzzzzzzzzzzzz"); err == nil {
		t.Error("expected error for non-hex")
	}
}

// TestMapFreqToDevice は周波数換算の区分線形が仕様通りであることを確認する。
func TestMapFreqToDevice(t *testing.T) {
	cases := []struct {
		in   uint16
		want uint8
	}{
		{5, 10},     // 範囲外 -> 10
		{10, 10},    // 下端
		{50, 50},    // そのまま
		{100, 100},  // 区間1上端
		{101, 100},  // 区間2 (101-100)/5+100 = 100
		{105, 101},  // (105-100)/5+100 = 101
		{600, 200},  // 区間2上端 (600-100)/5+100 = 200
		{601, 200},  // 区間3 (601-600)/10+200 = 200
		{610, 201},  // (610-600)/10+200 = 201
		{1000, 240}, // 区間3上端 (1000-600)/10+200 = 240
		{2000, 10},  // 範囲外
	}
	for _, c := range cases {
		if got := MapFreqToDevice(c.in); got != c.want {
			t.Errorf("MapFreqToDevice(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestPresetByName(t *testing.T) {
	if _, ok := PresetByName("Breathing"); !ok {
		t.Error(`PresetByName("Breathing") not found`)
	}
	if _, ok := PresetByName("does-not-exist"); ok {
		t.Error("unexpected preset found")
	}
}
