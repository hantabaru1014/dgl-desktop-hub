package ble

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/hantabaru1014/dgl-desktop-hub/internal/waveform"
)

func hexUpper(b []byte) string { return strings.ToUpper(hex.EncodeToString(b)) }

// TestEncodeB0 は仕様の例 No.1-1 と一致することを確認する。
// 0xB0+0b0000+0b0000+0+0+{10,10,10,10}+{0,10,20,30}+{0,0,0,0}+{0,0,0,101}
// = 0xB00000000A0A0A0A000A141E0000000000000065
func TestEncodeB0Spec(t *testing.T) {
	qa := waveform.Quad{Freq: [4]uint8{10, 10, 10, 10}, Intensity: [4]uint8{0, 10, 20, 30}}
	qb := waveform.Quad{Freq: [4]uint8{0, 0, 0, 0}, Intensity: [4]uint8{0, 0, 0, 101}}
	b := EncodeB0(0, ParseNoChange, ParseNoChange, 0, 0, qa, qb)
	got := hexUpper(b[:])
	want := "B00000000A0A0A0A000A141E0000000000000065"
	if got != want {
		t.Errorf("EncodeB0 = %s, want %s", got, want)
	}
}

// TestEncodeB0Seq は仕様の例 No.2-1 (seq=0, methodA=相対増 0b01, setA=5)。
// 0xB0+0b0000+0b0100+5+0+... = 0xB00405000A0A0A0A000A141E0000000000000065
func TestEncodeB0Method(t *testing.T) {
	qa := waveform.Quad{Freq: [4]uint8{10, 10, 10, 10}, Intensity: [4]uint8{0, 10, 20, 30}}
	qb := waveform.Quad{Freq: [4]uint8{0, 0, 0, 0}, Intensity: [4]uint8{0, 0, 0, 101}}
	b := EncodeB0(0, ParseRelativeInc, ParseNoChange, 5, 0, qa, qb)
	got := hexUpper(b[:])
	want := "B00405000A0A0A0A000A141E0000000000000065"
	if got != want {
		t.Errorf("EncodeB0 = %s, want %s", got, want)
	}
}

func TestEncodeB0SeqAndMethod(t *testing.T) {
	// seq=1, methodA=絶対(0b11), methodB=相対減(0b10) -> byte1 = 0x1<<4 | (0b11<<2|0b10) = 0x10|0x0E = 0x1E
	b := EncodeB0(1, ParseAbsolute, ParseRelativeDec, 0, 0, waveform.ZeroQuad(), waveform.ZeroQuad())
	if b[1] != 0x1E {
		t.Errorf("byte1 = %#x, want 0x1E", b[1])
	}
}

func TestEncodeBF(t *testing.T) {
	b := EncodeBF(150, 30, 0, 0, 0, 0)
	if b[0] != 0xBF || b[1] != 150 || b[2] != 30 {
		t.Errorf("EncodeBF = % x", b)
	}
	if len(b) != 7 {
		t.Errorf("BF len = %d, want 7", len(b))
	}
}

func TestDecodeB1(t *testing.T) {
	msg, ok := DecodeB1([]byte{0xB1, 0x01, 25, 30})
	if !ok {
		t.Fatal("DecodeB1 ok=false")
	}
	if msg.Seq != 1 || msg.StrengthA != 25 || msg.StrengthB != 30 {
		t.Errorf("DecodeB1 = %+v", msg)
	}
	if _, ok := DecodeB1([]byte{0xB0, 0, 0, 0}); ok {
		t.Error("DecodeB1 should reject non-B1 header")
	}
	if _, ok := DecodeB1([]byte{0xB1, 0}); ok {
		t.Error("DecodeB1 should reject short buffer")
	}
}
