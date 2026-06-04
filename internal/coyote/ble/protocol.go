// Package ble は Coyote v3 への BLE 直接接続を実装する。
// protocol.go は B0/BF/B1 指令のエンコード/デコードを行う純粋関数群で、
// ハードウェア非依存・単体テスト可能。
package ble

import "github.com/hantabaru1014/dgl-desktop-hub/internal/waveform"

// NamePrefix は Coyote v3.0 (脉冲主机 3.0) の BLE 広告名 prefix。
const NamePrefix = "47L121000"

// ParseMethod は B0 指令の強度値解読方式 (2bit)。
type ParseMethod uint8

const (
	ParseNoChange    ParseMethod = 0b00 // 変更なし
	ParseRelativeInc ParseMethod = 0b01 // 相対増加
	ParseRelativeDec ParseMethod = 0b10 // 相対減少
	ParseAbsolute    ParseMethod = 0b11 // 絶対設定
)

// EncodeB0 は B0 指令 (20byte) を生成する。
//
//	byte0      : 0xB0
//	byte1      : (seq<<4) | (methodA<<2 | methodB)
//	byte2      : A 通道強度設定値 (0..200)
//	byte3      : B 通道強度設定値 (0..200)
//	byte4..7   : A 通道波形周波数 4 条 (10..240)
//	byte8..11  : A 通道波形強度 4 条 (0..100)
//	byte12..15 : B 通道波形周波数 4 条
//	byte16..19 : B 通道波形強度 4 条
func EncodeB0(seq uint8, methodA, methodB ParseMethod, setA, setB uint8, qa, qb waveform.Quad) [20]byte {
	var b [20]byte
	b[0] = 0xB0
	b[1] = (seq&0x0F)<<4 | (uint8(methodA&0b11)<<2 | uint8(methodB&0b11))
	b[2] = setA
	b[3] = setB
	af, ai := qa.FreqBytes(), qa.IntensityBytes()
	bf, bi := qb.FreqBytes(), qb.IntensityBytes()
	copy(b[4:8], af[:])
	copy(b[8:12], ai[:])
	copy(b[12:16], bf[:])
	copy(b[16:20], bi[:])
	return b
}

// DisabledIntensity は波形を無効化する強度 quad (1 つだけ >100 を入れる)。
// 強度 0 を送ると「波形は有効だが振幅 0」になるが、これは出力なしと同義。
// 明示的にチャンネルを無効化したい場合に使う。
func DisabledIntensity() [4]byte { return [4]byte{0, 0, 0, 101} }

// EncodeBF は BF 指令 (7byte) を生成する。
//
//	byte0 : 0xBF
//	byte1 : A 通道強度ソフト上限 (0..200)
//	byte2 : B 通道強度ソフト上限 (0..200)
//	byte3 : A 通道波形周波数平衡参数 (0..255)
//	byte4 : B 通道波形周波数平衡参数
//	byte5 : A 通道波形強度平衡参数 (0..255)
//	byte6 : B 通道波形強度平衡参数
func EncodeBF(limA, limB, freqBalA, freqBalB, intBalA, intBalB uint8) [7]byte {
	return [7]byte{0xBF, limA, limB, freqBalA, freqBalB, intBalA, intBalB}
}

// B1Message は B1 回応消息 (強度フィードバック)。
type B1Message struct {
	Seq       uint8
	StrengthA uint8
	StrengthB uint8
}

// DecodeB1 は 0x150B の通知バイト列を B1 メッセージとして解釈する。
// 先頭が 0xB1 で 4byte 以上のときのみ ok=true。
func DecodeB1(buf []byte) (B1Message, bool) {
	if len(buf) < 4 || buf[0] != 0xB1 {
		return B1Message{}, false
	}
	return B1Message{Seq: buf[1], StrengthA: buf[2], StrengthB: buf[3]}, true
}
