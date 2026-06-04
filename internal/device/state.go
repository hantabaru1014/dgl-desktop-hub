package device

import "github.com/hantabaru1014/dgl-desktop-hub/internal/waveform"

// MaxStrength は Coyote のチャンネル強度の絶対上限。
const MaxStrength uint8 = 200

// SoftLimit はチャンネルごとの強度ソフトリミット (0..200)。
// ハブはこの値を超える強度を決して出力しない。
type SoftLimit struct {
	A uint8
	B uint8
}

// DefaultSoftLimit は新規デバイスの初期ソフトリミット。
// 安全側に倒し、控えめな値から始める。
func DefaultSoftLimit() SoftLimit {
	return SoftLimit{A: 20, B: 20}
}

// clampStrength は target を [0, limit] かつ [0, MaxStrength] に収める。
func clampStrength(target, limit uint8) uint8 {
	if limit > MaxStrength {
		limit = MaxStrength
	}
	if target > limit {
		return limit
	}
	return target
}

// channelState は 1 チャンネルの可変状態。
type channelState struct {
	target   uint8 // ユーザ/アプリが要求する目標強度 (clamp 前, 0..200)
	reported uint8 // デバイスが報告した実強度 (BLE B1 等)
	wave     waveform.Waveform
	playhead int
}

// applyStrength は mode に従って target を更新する。
func (c *channelState) applyStrength(mode StrengthMode, val uint8) {
	switch mode {
	case StrengthAbsolute:
		c.target = capStrength(int(val))
	case StrengthRelativeInc:
		c.target = capStrength(int(c.target) + int(val))
	case StrengthRelativeDec:
		c.target = capStrength(int(c.target) - int(val))
	}
}

// nextQuad は現在の再生位置の Quad を返し、再生位置を 1 進める。
func (c *channelState) nextQuad() waveform.Quad {
	q := c.wave.FrameAt(c.playhead)
	c.playhead++
	if n := c.wave.Len(); n > 0 {
		c.playhead %= n
	} else {
		c.playhead = 0
	}
	return q
}

// setWaveform は波形を差し替え、再生位置を先頭へ戻す。
func (c *channelState) setWaveform(w waveform.Waveform) {
	c.wave = w
	c.playhead = 0
}

func capStrength(v int) uint8 {
	if v < 0 {
		return 0
	}
	if v > int(MaxStrength) {
		return MaxStrength
	}
	return uint8(v)
}
