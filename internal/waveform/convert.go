package waveform

import (
	"encoding/hex"
	"fmt"
	"strings"
)

// QuadToHex は Quad を socket mode の V3 hex 文字列 (16 桁、大文字) へ変換する。
// 先頭 4byte が波形周波数、後続 4byte が波形強度。
// 例: Freq{10,10,10,10}, Intensity{20,20,20,20} -> "0A0A0A0A14141414"。
func QuadToHex(q Quad) string {
	b := []byte{
		q.Freq[0], q.Freq[1], q.Freq[2], q.Freq[3],
		q.Intensity[0], q.Intensity[1], q.Intensity[2], q.Intensity[3],
	}
	return strings.ToUpper(hex.EncodeToString(b))
}

// HexToQuad は socket mode の V3 hex 文字列 (16 桁 = 8byte) を Quad へ変換する。
func HexToQuad(s string) (Quad, error) {
	b, err := hex.DecodeString(strings.TrimSpace(s))
	if err != nil {
		return Quad{}, fmt.Errorf("waveform: invalid hex %q: %w", s, err)
	}
	if len(b) != 8 {
		return Quad{}, fmt.Errorf("waveform: hex must be 8 bytes, got %d (%q)", len(b), s)
	}
	return Quad{
		Freq:      [4]uint8{b[0], b[1], b[2], b[3]},
		Intensity: [4]uint8{b[4], b[5], b[6], b[7]},
	}, nil
}

// HexFramesToWaveform は V3 hex 文字列の列を Waveform へ変換する。
func HexFramesToWaveform(name string, hexes []string) (Waveform, error) {
	frames := make([]Quad, 0, len(hexes))
	for _, h := range hexes {
		q, err := HexToQuad(h)
		if err != nil {
			return Waveform{}, err
		}
		frames = append(frames, q)
	}
	return Waveform{Name: name, Frames: frames}, nil
}

// V2FrameToQuad は OpenDGLab の自定義波形 (3byte, V2 形式) を 1 つの Quad へ
// 変換する。
//
// 注意: V2 の脈衝データは X(周波数)/Y/Z(強度) をビットパックした形式で、
// 周波数の厳密な復元は複雑。現状は安全側の近似として
//   - 周波数: 既定値 10 (有効範囲の下端)
//   - 強度  : 第3バイト * 10 を 100 で飽和
// とする。多くのプリセットで第3バイト*10 が V3 強度に一致する。
// Phase 4 で gold ベクタ (presets.json の expectedV2/expectedV3) に
// 合わせて精緻化予定。
func V2FrameToQuad(f [3]byte) Quad {
	intensity := int(f[2]) * 10
	if intensity > 100 {
		intensity = 100
	}
	iv := uint8(intensity)
	return Quad{
		Freq:      [4]uint8{10, 10, 10, 10},
		Intensity: [4]uint8{iv, iv, iv, iv},
	}
}

// V2FramesToWaveform は V2 3byte フレーム列を Waveform へ変換する。
func V2FramesToWaveform(name string, frames [][3]byte) Waveform {
	out := make([]Quad, 0, len(frames))
	for _, f := range frames {
		out = append(out, V2FrameToQuad(f))
	}
	return Waveform{Name: name, Frames: out}
}

// WaveformToHexFrames は Waveform を V3 hex 文字列の列へ変換する。
func WaveformToHexFrames(w Waveform) []string {
	out := make([]string, 0, len(w.Frames))
	for _, q := range w.Frames {
		out = append(out, QuadToHex(q))
	}
	return out
}
