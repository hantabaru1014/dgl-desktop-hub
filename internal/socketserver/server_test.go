package socketserver

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/hantabaru1014/dgl-desktop-hub/internal/device"
)

func TestParseStrengthFeedback(t *testing.T) {
	fb, ok := ParseStrengthFeedback("strength-11+7+100+35")
	if !ok {
		t.Fatal("parse failed")
	}
	if fb.A != 11 || fb.B != 7 || fb.LimitA != 100 || fb.LimitB != 35 {
		t.Errorf("got %+v", fb)
	}
	if _, ok := ParseStrengthFeedback("pulse-A:[]"); ok {
		t.Error("should not parse non-strength")
	}
}

func TestPulseCommand(t *testing.T) {
	got := PulseCommand("A", []string{"0A0A0A0A64646464"})
	want := `pulse-A:["0A0A0A0A64646464"]`
	if got != want {
		t.Errorf("PulseCommand = %q, want %q", got, want)
	}
}

func TestQRString(t *testing.T) {
	got := QRString("192.168.1.5", 9999, "abc")
	want := "https://www.dungeon-lab.com/app-download.php#DGLAB-SOCKET#ws://192.168.1.5:9999/abc"
	if got != want {
		t.Errorf("QR = %q", got)
	}
}

// TestSocketBindAndControl はスマホ役の WebSocket クライアントで bind し、
// ハブからの強度/波形メッセージが届くこと、強度フィードバックが反映される
// ことを確認する。
func TestSocketBindAndControl(t *testing.T) {
	mgr := device.NewDeviceManager(device.Hooks{})
	srv := NewServer(Callbacks{
		OnConnect:    func(c device.CoyoteDevice) { mgr.Add(c) },
		OnDisconnect: func(id device.DeviceID) { _ = mgr.Remove(id) },
	})
	if err := srv.Start("127.0.0.1", 0); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer srv.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	url := "ws://" + srv.Addr()
	ws, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer ws.Close(websocket.StatusNormalClosure, "")

	// 1) サーバから自身の ID を受け取る。
	first := readEnvelope(t, ctx, ws)
	if first.Type != "bind" || first.Message != "targetId" {
		t.Fatalf("first message = %+v", first)
	}
	phoneID := first.ClientID

	// 2) bind 要求 (clientId=controllerID, targetId=自身)。
	writeEnvelope(t, ctx, ws, Envelope{Type: "bind", ClientID: srv.controllerID, TargetID: phoneID, Message: "bind"})

	// 3) 200 を受け取る。
	second := readEnvelope(t, ctx, ws)
	if second.Type != "bind" || second.Message != "200" {
		t.Fatalf("bind reply = %+v", second)
	}

	// デバイスが登録されたはず。
	var id device.DeviceID
	for i := 0; i < 50; i++ {
		list := mgr.List()
		if len(list) == 1 {
			id = list[0].ID
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if id == "" {
		t.Fatal("socket device was not registered")
	}

	// 強度を設定して出力ループを開始。
	_ = mgr.SetSoftLimit(id, device.SoftLimit{A: 200, B: 200})
	_ = mgr.SetStrength(id, device.ChannelA, device.StrengthAbsolute, 50)
	mgr.Start()
	defer mgr.Stop()

	// strength と pulse メッセージを受信できること。
	gotStrength, gotPulse := false, false
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && (!gotStrength || !gotPulse) {
		e := readEnvelope(t, ctx, ws)
		if strings.HasPrefix(e.Message, "strength-1+2+50") {
			gotStrength = true
		}
		if strings.HasPrefix(e.Message, "pulse-A:") {
			gotPulse = true
		}
	}
	if !gotStrength {
		t.Error("did not receive strength-1+2+50")
	}
	if !gotPulse {
		t.Error("did not receive pulse-A")
	}

	// 強度フィードバックを送り、reported に反映されることを確認。
	writeEnvelope(t, ctx, ws, Envelope{Type: "msg", ClientID: srv.controllerID, TargetID: phoneID, Message: "strength-30+40+100+100"})
	ok := false
	for i := 0; i < 50; i++ {
		_, _, ra, rb, _, _ := mgr.Snapshot(id)
		if ra == 30 && rb == 40 {
			ok = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !ok {
		t.Error("strength feedback not reflected in reported values")
	}
}

func readEnvelope(t *testing.T, ctx context.Context, ws *websocket.Conn) Envelope {
	t.Helper()
	_, data, err := ws.Read(ctx)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var e Envelope
	if err := json.Unmarshal(data, &e); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return e
}

func writeEnvelope(t *testing.T, ctx context.Context, ws *websocket.Conn, e Envelope) {
	t.Helper()
	data, _ := json.Marshal(e)
	if err := ws.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("write: %v", err)
	}
}
