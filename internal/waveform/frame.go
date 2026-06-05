// Package waveform は Coyote の波形データの内部表現と、各プロトコル
// (Coyote V3 BLE の B0、socket mode の V3 hex、OpenDGLab v2 3byte) 間の
// 変換を提供する。外部依存を持たない純粋パッケージ。
package waveform

// Quad は 1 チャンネル分の 100ms の波形スライス。
// 4 組の周波数/強度から成り、各組は 25ms の出力を表す (4 x 25ms = 100ms)。
//
// この構造は Coyote V3 の B0 指令におけるチャンネル波形データ
// (波形周波数4byte + 波形強度4byte) および socket mode の V3 hex
// ("FFFFFFFFIIIIIIII") とそのまま対応する。
type Quad struct {
	// Freq は波形周波数。デバイス単位 (10..240)。
	Freq [4]uint8
	// Intensity は波形強度 (0..100)。
	Intensity [4]uint8
}

// ZeroQuad は無出力 (強度 0) の Quad を返す。周波数は最小有効値 10。
func ZeroQuad() Quad {
	return Quad{
		Freq:      [4]uint8{10, 10, 10, 10},
		Intensity: [4]uint8{0, 0, 0, 0},
	}
}

// MeanIntensity はこの Quad の平均波形強度 (0..100) を返す。
// グラフ表示用の振幅として利用する。
func (q Quad) MeanIntensity() uint8 {
	sum := int(q.Intensity[0]) + int(q.Intensity[1]) + int(q.Intensity[2]) + int(q.Intensity[3])
	return uint8(sum / 4)
}

// Waveform は時系列に並んだ Quad 列。各 Quad は 100ms を表す。
// 空の場合は無出力とみなす。再生はループする。
type Waveform struct {
	Name   string
	Frames []Quad
}

// FrameAt は再生位置 idx (0 以上) に対応する Quad を返す。
// フレーム列が空なら ZeroQuad を返す。idx はフレーム数で循環する。
func (w Waveform) FrameAt(idx int) Quad {
	if len(w.Frames) == 0 {
		return ZeroQuad()
	}
	return w.Frames[idx%len(w.Frames)]
}

// Len はフレーム数を返す。
func (w Waveform) Len() int { return len(w.Frames) }
