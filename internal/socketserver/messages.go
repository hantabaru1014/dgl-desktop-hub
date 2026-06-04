package socketserver

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Envelope は socket mode の統一メッセージ形式。
type Envelope struct {
	Type     string `json:"type"`
	ClientID string `json:"clientId"`
	TargetID string `json:"targetId"`
	Message  string `json:"message"`
}

// QRString は DG-Lab アプリがスキャンする QR 文字列を組み立てる。
//
//	https://www.dungeon-lab.com/app-download.php#DGLAB-SOCKET#ws://<host>:<port>/<clientId>
func QRString(host string, port int, clientID string) string {
	return fmt.Sprintf("https://www.dungeon-lab.com/app-download.php#DGLAB-SOCKET#ws://%s:%d/%s", host, port, clientID)
}

// StrengthFeedback はアプリからの強度報告 "strength-<A>+<B>+<Alimit>+<Blimit>"。
type StrengthFeedback struct {
	A, B       int
	LimitA     int
	LimitB     int
}

// ParseStrengthFeedback はアプリ報告メッセージを解析する。
func ParseStrengthFeedback(msg string) (StrengthFeedback, bool) {
	const prefix = "strength-"
	if !strings.HasPrefix(msg, prefix) {
		return StrengthFeedback{}, false
	}
	parts := strings.Split(strings.TrimPrefix(msg, prefix), "+")
	if len(parts) != 4 {
		return StrengthFeedback{}, false
	}
	nums := make([]int, 4)
	for i, p := range parts {
		v, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			return StrengthFeedback{}, false
		}
		nums[i] = v
	}
	return StrengthFeedback{A: nums[0], B: nums[1], LimitA: nums[2], LimitB: nums[3]}, true
}

// StrengthCommand はアプリへ送る強度指令 "strength-<ch>+<mode>+<val>" を作る。
// ch: 1=A,2=B。mode: 0=減,1=増,2=絶対。
func StrengthCommand(ch, mode, val int) string {
	return fmt.Sprintf("strength-%d+%d+%d", ch, mode, val)
}

// PulseCommand はアプリへ送る波形指令 "pulse-<ch>:[\"hex\",...]" を作る。
// ch: "A" または "B"。
func PulseCommand(ch string, hexFrames []string) string {
	quoted := make([]string, len(hexFrames))
	for i, h := range hexFrames {
		quoted[i] = strconv.Quote(h)
	}
	return fmt.Sprintf("pulse-%s:[%s]", ch, strings.Join(quoted, ","))
}

func (e Envelope) encode() ([]byte, error) { return json.Marshal(e) }
