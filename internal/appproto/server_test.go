package appproto

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/hantabaru1014/dgl-desktop-hub/internal/device"
	"github.com/hantabaru1014/dgl-desktop-hub/internal/hubcore"
	pb "github.com/hantabaru1014/dgl-desktop-hub/internal/pb/com/github/opendglab"
	"github.com/hantabaru1014/dgl-desktop-hub/internal/pb/com/github/opendglab/opendglabconnect"
)

// setup はハブ + demo デバイス + httptest サーバ + connect クライアントを用意する。
func setup(t *testing.T) (*hubcore.Hub, *device.DemoDevice, device.DeviceID, opendglabconnect.OpenDGLabServiceClient, *httptest.Server) {
	t.Helper()
	hub := hubcore.NewHub()
	d := device.NewDemoDevice("e2e")
	id := hub.Mgr.Add(d)
	_ = hub.Mgr.SetSoftLimit(id, device.SoftLimit{A: 200, B: 200})

	srv := httptest.NewServer(NewServer(hub).Handler())
	t.Cleanup(srv.Close)

	client := opendglabconnect.NewOpenDGLabServiceClient(srv.Client(), srv.URL)
	return hub, d, id, client, srv
}

// send は X-DGLab-Token なしで単発リクエストを送信する。
func send(t *testing.T, c opendglabconnect.OpenDGLabServiceClient, req *pb.DGRequest) *pb.DGResponse {
	t.Helper()
	resp, err := c.Send(context.Background(), connect.NewRequest(req))
	if err != nil {
		t.Fatalf("Send(%v): %v", req.GetEvent(), err)
	}
	return resp.Msg
}

// sendTok は X-DGLab-Token ヘッダ付きで単発リクエストを送信する。
func sendTok(t *testing.T, c opendglabconnect.OpenDGLabServiceClient, req *pb.DGRequest, token string) *pb.DGResponse {
	t.Helper()
	r := connect.NewRequest(req)
	if token != "" {
		r.Header().Set("X-DGLab-Token", token)
	}
	resp, err := c.Send(context.Background(), r)
	if err != nil {
		t.Fatalf("Send(%v): %v", req.GetEvent(), err)
	}
	return resp.Msg
}

// connectApp は CONNECT を送ってトークンを取得するヘルパー。
func connectApp(t *testing.T, c opendglabconnect.OpenDGLabServiceClient, name, uuid string) string {
	t.Helper()
	resp := send(t, c, &pb.DGRequest{
		Version: 1,
		Event:   pb.DGEvent_CONNECT,
		Connect: &pb.DGRequest_DGConnect{AppName: name, Uuid: uuid},
	})
	tok := resp.GetConnect().GetToken()
	if tok == "" {
		t.Fatalf("connectApp(%s): got empty token", name)
	}
	return tok
}

// TestExclusiveMode は排他モードの新仕様を確認する:
// (a) ロック前の SETSTRENGTH は CANTDOTHIS+DEVICENOTLOCK
// (b) LOCKDEVICE 後は IsLockedByMe=true
// (c) ロック取得アプリの SETSTRENGTH は成功
// (d) 別アプリの SETSTRENGTH は CANTDOTHIS+DEVICENOTLOCKBYYOU
// (e) 最終強度はロック取得アプリが設定した値
func TestExclusiveMode(t *testing.T) {
	hub, _, id, c1, srv := setup(t)
	hub.SetExclusive(id, true)
	c2 := opendglabconnect.NewOpenDGLabServiceClient(srv.Client(), srv.URL)

	tok1 := connectApp(t, c1, "a1", "u1")
	tok2 := connectApp(t, c2, "a2", "u2")

	// (a) ロック前の SETSTRENGTH は CANTDOTHIS+DEVICENOTLOCK
	pre := sendTok(t, c1, &pb.DGRequest{
		Version:  1,
		Event:    pb.DGEvent_SETSTRENGTH,
		Device:   &pb.DGRequest_DGDeviceID{DeviceId: string(id)},
		Strength: &pb.DGRequest_DGStrength{StrengthA: proto.Int32(5)},
	}, tok1)
	if pre.GetEvent() != pb.DGEvent_CANTDOTHIS || pre.GetError() != pb.DGError_DEVICENOTLOCK {
		t.Errorf("(a) want CANTDOTHIS+DEVICENOTLOCK, got event=%v error=%v", pre.GetEvent(), pre.GetError())
	}

	// (b) app1 が LOCKDEVICE → IsLockedByMe=true
	lockResp := sendTok(t, c1, &pb.DGRequest{
		Version: 1,
		Event:   pb.DGEvent_LOCKDEVICE,
		Device:  &pb.DGRequest_DGDeviceID{DeviceId: string(id)},
	}, tok1)
	if !lockResp.GetDevice().GetIsLockedByMe() {
		t.Errorf("(b) after LOCKDEVICE, IsLockedByMe should be true")
	}

	// (c) app1 の SETSTRENGTH 成功
	r1 := sendTok(t, c1, &pb.DGRequest{
		Version:  1,
		Event:    pb.DGEvent_SETSTRENGTH,
		Device:   &pb.DGRequest_DGDeviceID{DeviceId: string(id)},
		Strength: &pb.DGRequest_DGStrength{StrengthA: proto.Int32(10)},
	}, tok1)
	if r1.GetEvent() == pb.DGEvent_CANTDOTHIS {
		t.Fatalf("(c) app1 SETSTRENGTH after LOCKDEVICE should succeed, got error %v", r1.GetError())
	}

	// (d) app2 の SETSTRENGTH は CANTDOTHIS+DEVICENOTLOCKBYYOU
	r2 := sendTok(t, c2, &pb.DGRequest{
		Version:  1,
		Event:    pb.DGEvent_SETSTRENGTH,
		Device:   &pb.DGRequest_DGDeviceID{DeviceId: string(id)},
		Strength: &pb.DGRequest_DGStrength{StrengthA: proto.Int32(99)},
	}, tok2)
	if r2.GetEvent() != pb.DGEvent_CANTDOTHIS || r2.GetError() != pb.DGError_DEVICENOTLOCKBYYOU {
		t.Errorf("(d) app2 should be blocked, got %v / %v", r2.GetEvent(), r2.GetError())
	}

	// (e) 最終強度は app1 の値
	ta, _, _, _, _, _ := hub.Mgr.Snapshot(id)
	if ta != 10 {
		t.Errorf("(e) strength = %d, want 10 (only app1)", ta)
	}
}

// TestApprovalApprove は autoApprove=false で承認するとトークンが発行される
// ことを確認する。
func TestApprovalApprove(t *testing.T) {
	hub, _, _, c, _ := setup(t)
	hub.SetAutoApprove(false)
	hub.SetEventSink(hubcore.EventSink{
		OnApprovalRequest: func(r hubcore.ApprovalRequest) { hub.ResolveApproval(r.RequestID, true) },
	})
	resp := send(t, c, &pb.DGRequest{Version: 1, Event: pb.DGEvent_CONNECT,
		Connect: &pb.DGRequest_DGConnect{AppName: "ask", Uuid: "u"}})
	if resp.GetConnect().GetToken() == "" {
		t.Error("approved connection should return a token")
	}
}

// TestApprovalDeny は拒否すると空トークンが返ることを確認する。
func TestApprovalDeny(t *testing.T) {
	hub, _, _, c, _ := setup(t)
	hub.SetAutoApprove(false)
	hub.SetEventSink(hubcore.EventSink{
		OnApprovalRequest: func(r hubcore.ApprovalRequest) { hub.ResolveApproval(r.RequestID, false) },
	})
	resp := send(t, c, &pb.DGRequest{Version: 1, Event: pb.DGEvent_CONNECT,
		Connect: &pb.DGRequest_DGConnect{AppName: "ask", Uuid: "u"}})
	if resp.GetConnect().GetToken() != "" {
		t.Error("denied connection should return empty token")
	}
}

func TestConnectAndGetDevice(t *testing.T) {
	_, _, id, c, _ := setup(t)

	tok := connectApp(t, c, "tester", "uuid-1")

	resp := sendTok(t, c, &pb.DGRequest{Version: 1, Event: pb.DGEvent_GETDEVICE}, tok)
	devs := resp.GetDeviceList().GetDevices()
	if len(devs) != 1 {
		t.Fatalf("device count = %d, want 1", len(devs))
	}
	if devs[0].GetId() != string(id) {
		t.Errorf("device id = %s, want %s", devs[0].GetId(), id)
	}
	if !devs[0].GetIsLockedByMe() {
		t.Error("auto-approve+multiApp: device should report isLockedByMe=true")
	}
}

func TestSetStrengthDrivesDevice(t *testing.T) {
	hub, _, id, c, _ := setup(t)

	tok := connectApp(t, c, "tester", "uuid-2")

	sendTok(t, c, &pb.DGRequest{Version: 1, Event: pb.DGEvent_SETSTRENGTH,
		Device:   &pb.DGRequest_DGDeviceID{DeviceId: string(id)},
		Strength: &pb.DGRequest_DGStrength{StrengthA: proto.Int32(42), StrengthB: proto.Int32(77)}}, tok)

	ta, tb, _, _, _, err := hub.Mgr.Snapshot(id)
	if err != nil {
		t.Fatal(err)
	}
	if ta != 42 || tb != 77 {
		t.Errorf("target = (%d,%d), want (42,77)", ta, tb)
	}

	resp := sendTok(t, c, &pb.DGRequest{Version: 1, Event: pb.DGEvent_GETSTRENGTH,
		Device: &pb.DGRequest_DGDeviceID{DeviceId: string(id)}}, tok)
	if resp.GetStrength().GetStrengthA() != 42 {
		t.Errorf("GETSTRENGTH A = %d, want 42", resp.GetStrength().GetStrengthA())
	}
}

// TestSetStrengthSingleChannel は片方のチャンネルだけ送ったとき
// もう片方の値が維持されることを確認する (DGStrength の optional 化)。
func TestSetStrengthSingleChannel(t *testing.T) {
	hub, _, id, c, _ := setup(t)
	tok := connectApp(t, c, "tester", "uuid-single")

	// まず A=30, B=40 を設定。
	sendTok(t, c, &pb.DGRequest{Version: 1, Event: pb.DGEvent_SETSTRENGTH,
		Device:   &pb.DGRequest_DGDeviceID{DeviceId: string(id)},
		Strength: &pb.DGRequest_DGStrength{StrengthA: proto.Int32(30), StrengthB: proto.Int32(40)}}, tok)

	// A だけ 55 に更新。B は維持されるべき。
	sendTok(t, c, &pb.DGRequest{Version: 1, Event: pb.DGEvent_SETSTRENGTH,
		Device:   &pb.DGRequest_DGDeviceID{DeviceId: string(id)},
		Strength: &pb.DGRequest_DGStrength{StrengthA: proto.Int32(55)}}, tok)
	if ta, tb, _, _, _, _ := hub.Mgr.Snapshot(id); ta != 55 || tb != 40 {
		t.Errorf("after A-only update: (%d,%d), want (55,40)", ta, tb)
	}

	// B だけ 0 に更新。A は維持されるべき (0 値も明示送信で反映される)。
	sendTok(t, c, &pb.DGRequest{Version: 1, Event: pb.DGEvent_SETSTRENGTH,
		Device:   &pb.DGRequest_DGDeviceID{DeviceId: string(id)},
		Strength: &pb.DGRequest_DGStrength{StrengthB: proto.Int32(0)}}, tok)
	if ta, tb, _, _, _, _ := hub.Mgr.Snapshot(id); ta != 55 || tb != 0 {
		t.Errorf("after B-only zero: (%d,%d), want (55,0)", ta, tb)
	}
}

// TestSetStrengthPercent は %指定がソフトリミット基準の絶対値に換算されること、
// 100超は 100% (=リミット) に、100未満は比率に従い適用されることを確認する。
// percent と absolute が同一チャンネルで両方指定された場合は percent を優先する。
func TestSetStrengthPercent(t *testing.T) {
	hub, _, id, c, _ := setup(t)
	// setup() で limit=(200,200)。テストしやすいよう A=100, B=50 に設定。
	_ = hub.Mgr.SetSoftLimit(id, device.SoftLimit{A: 100, B: 50})
	tok := connectApp(t, c, "tester", "uuid-pct")

	// A=50% (→ 50), B=100% (→ 50)
	sendTok(t, c, &pb.DGRequest{Version: 1, Event: pb.DGEvent_SETSTRENGTH,
		Device: &pb.DGRequest_DGDeviceID{DeviceId: string(id)},
		Strength: &pb.DGRequest_DGStrength{
			StrengthAPercent: proto.Int32(50),
			StrengthBPercent: proto.Int32(100),
		}}, tok)
	if ta, tb, _, _, _, _ := hub.Mgr.Snapshot(id); ta != 50 || tb != 50 {
		t.Errorf("after percent: (%d,%d), want (50,50)", ta, tb)
	}

	// 200% (clamp で 100%) は limit までで頭打ち。
	sendTok(t, c, &pb.DGRequest{Version: 1, Event: pb.DGEvent_SETSTRENGTH,
		Device: &pb.DGRequest_DGDeviceID{DeviceId: string(id)},
		Strength: &pb.DGRequest_DGStrength{
			StrengthAPercent: proto.Int32(200),
		}}, tok)
	if ta, _, _, _, _, _ := hub.Mgr.Snapshot(id); ta != 100 {
		t.Errorf("after 200%% on A: %d, want 100 (clamped to limit)", ta)
	}

	// 同一チャンネルで absolute と percent を両方指定 → percent 優先。
	// A: absolute=10, percent=80 (→ 80) / B: absolute だけ → そのまま反映。
	sendTok(t, c, &pb.DGRequest{Version: 1, Event: pb.DGEvent_SETSTRENGTH,
		Device: &pb.DGRequest_DGDeviceID{DeviceId: string(id)},
		Strength: &pb.DGRequest_DGStrength{
			StrengthA:        proto.Int32(10),
			StrengthAPercent: proto.Int32(80),
			StrengthB:        proto.Int32(7),
		}}, tok)
	if ta, tb, _, _, _, _ := hub.Mgr.Snapshot(id); ta != 80 || tb != 7 {
		t.Errorf("after mixed: (%d,%d), want (80,7)", ta, tb)
	}
}

// TestSoftLimitEnforcedAtOutput は出力ループでソフトリミットが効くことを
// 実 tick (hub.Start) を回して確認する。
func TestSoftLimitEnforcedAtOutput(t *testing.T) {
	hub, d, id, c, _ := setup(t)
	_ = hub.Mgr.SetSoftLimit(id, device.SoftLimit{A: 25, B: 25})

	tok := connectApp(t, c, "tester", "uuid-3")

	sendTok(t, c, &pb.DGRequest{Version: 1, Event: pb.DGEvent_SETSTRENGTH,
		Device:   &pb.DGRequest_DGDeviceID{DeviceId: string(id)},
		Strength: &pb.DGRequest_DGStrength{StrengthA: proto.Int32(200), StrengthB: proto.Int32(200)}}, tok)

	hub.Start()
	defer hub.Stop()
	time.Sleep(250 * time.Millisecond)

	cmd := d.LastCommand()
	if cmd.StrengthA != 25 || cmd.StrengthB != 25 {
		t.Errorf("clamped output = (%d,%d), want (25,25)", cmd.StrengthA, cmd.StrengthB)
	}
}

func TestSetWavePreset(t *testing.T) {
	hub, _, id, c, _ := setup(t)

	tok := connectApp(t, c, "tester", "uuid-4")

	resp := sendTok(t, c, &pb.DGRequest{Version: 1, Event: pb.DGEvent_GETWAVELIST}, tok)
	names := resp.GetWaveList().GetWave()
	if len(names) != 16 {
		t.Fatalf("wave list = %d, want 16", len(names))
	}

	sendTok(t, c, &pb.DGRequest{Version: 1, Event: pb.DGEvent_SETWAVE,
		Device: &pb.DGRequest_DGDeviceID{DeviceId: string(id), DeviceChannel: pb.DGDeviceChannel_CHANNEL_A},
		Wave:   &pb.DGRequest_DGWave{WaveName: "Breathing"}}, tok)

	a, _, err := hub.Mgr.WaveNames(id)
	if err != nil {
		t.Fatal(err)
	}
	if a != "Breathing" {
		t.Errorf("channel A wave = %q, want Breathing", a)
	}
}

func TestMultiAppControl(t *testing.T) {
	hub, _, id, c1, srv := setup(t)
	c2 := opendglabconnect.NewOpenDGLabServiceClient(srv.Client(), srv.URL)

	tok1 := connectApp(t, c1, "app1", "u1")
	tok2 := connectApp(t, c2, "app2", "u2")

	sendTok(t, c1, &pb.DGRequest{Version: 1, Event: pb.DGEvent_SETSTRENGTH,
		Device: &pb.DGRequest_DGDeviceID{DeviceId: string(id)}, Strength: &pb.DGRequest_DGStrength{StrengthA: proto.Int32(10)}}, tok1)
	sendTok(t, c2, &pb.DGRequest{Version: 1, Event: pb.DGEvent_SETSTRENGTH,
		Device: &pb.DGRequest_DGDeviceID{DeviceId: string(id)}, Strength: &pb.DGRequest_DGStrength{StrengthA: proto.Int32(20)}}, tok2)

	ta, _, _, _, _, _ := hub.Mgr.Snapshot(id)
	if ta != 20 {
		t.Errorf("last-writer-wins target A = %d, want 20", ta)
	}
	if got := len(hub.Apps()); got != 2 {
		t.Errorf("connected apps = %d, want 2", got)
	}
}

// TestQueryTokenAuth は ?token= クエリでの認証が X-DGLab-Token ヘッダと
// 同様に通ることを確認する (HTTP ヘッダを付与できないアプリ向け)。
// connect クライアントは baseURL にクエリを付けられないため、Connect の
// JSON unary プロトコルを生 HTTP POST で直接叩く。
func TestQueryTokenAuth(t *testing.T) {
	_, _, id, c, srv := setup(t)

	tok := connectApp(t, c, "tester", "uuid-query")

	body, err := protojson.Marshal(&pb.DGRequest{
		Version: 1,
		Event:   pb.DGEvent_GETDEVICE,
		Device:  &pb.DGRequest_DGDeviceID{DeviceId: string(id)},
	})
	if err != nil {
		t.Fatal(err)
	}

	url := srv.URL + opendglabconnect.OpenDGLabServiceSendProcedure + "?token=" + tok
	httpResp, err := srv.Client().Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", httpResp.StatusCode)
	}

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		t.Fatal(err)
	}
	var resp pb.DGResponse
	if err := protojson.Unmarshal(respBody, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.GetEvent() == pb.DGEvent_CANTDOTHIS {
		t.Fatalf("query-token auth should succeed, got CANTDOTHIS+%v", resp.GetError())
	}
	if devs := resp.GetDeviceList().GetDevices(); len(devs) != 1 {
		t.Fatalf("device count = %d, want 1", len(devs))
	}
}

// TestUnauthenticatedSend は CONNECT なし (トークンなし) の GETDEVICE が
// CANTDOTHIS+UNAUTHED になることを確認する。
func TestUnauthenticatedSend(t *testing.T) {
	_, _, id, c, _ := setup(t)

	resp := send(t, c, &pb.DGRequest{
		Version: 1,
		Event:   pb.DGEvent_GETDEVICE,
		Device:  &pb.DGRequest_DGDeviceID{DeviceId: string(id)},
	})
	if resp.GetEvent() != pb.DGEvent_CANTDOTHIS || resp.GetError() != pb.DGError_UNAUTHED {
		t.Errorf("want CANTDOTHIS+UNAUTHED, got event=%v error=%v", resp.GetEvent(), resp.GetError())
	}
}

// TestBadVersion は Version: 2 のリクエストが CANTDOTHIS+UNKNOWN になることを確認する。
func TestBadVersion(t *testing.T) {
	_, _, _, c, _ := setup(t)

	resp := send(t, c, &pb.DGRequest{
		Version: 2,
		Event:   pb.DGEvent_PING,
	})
	if resp.GetEvent() != pb.DGEvent_CANTDOTHIS || resp.GetError() != pb.DGError_UNKNOWN {
		t.Errorf("want CANTDOTHIS+UNKNOWN, got event=%v error=%v", resp.GetEvent(), resp.GetError())
	}
}

// TestCustomWaveInvalidFrame は 3 バイト以外のフレームを含む CUSTOMWAVE が
// CANTDOTHIS+UNKNOWN になることを確認する。
func TestCustomWaveInvalidFrame(t *testing.T) {
	_, _, id, c, _ := setup(t)

	tok := connectApp(t, c, "tester", "uuid-cwif")

	resp := sendTok(t, c, &pb.DGRequest{
		Version: 1,
		Event:   pb.DGEvent_CUSTOMWAVE,
		Device:  &pb.DGRequest_DGDeviceID{DeviceId: string(id)},
		CustomWave: []*pb.DGRequest_DGCustomWave{
			{Bytes: []byte{0x01, 0x02}}, // 2 バイト: 不正
		},
	}, tok)
	if resp.GetEvent() != pb.DGEvent_CANTDOTHIS || resp.GetError() != pb.DGError_UNKNOWN {
		t.Errorf("want CANTDOTHIS+UNKNOWN for invalid frame, got event=%v error=%v", resp.GetEvent(), resp.GetError())
	}
}

// TestSubscribeUnauthed はトークンなし・CONNECT でないリクエストで Subscribe を
// 開始すると最初の受信が CANTDOTHIS+UNAUTHED でストリームが終了することを確認する。
func TestSubscribeUnauthed(t *testing.T) {
	_, _, _, c, _ := setup(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := connect.NewRequest(&pb.DGRequest{
		Version: 1,
		Event:   pb.DGEvent_GETDEVICE,
	})
	// トークンなし (ヘッダ設定しない)

	stream, err := c.Subscribe(ctx, req)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer stream.Close()

	// 最初のメッセージが CANTDOTHIS+UNAUTHED であること。
	ok := stream.Receive()
	if !ok {
		t.Fatal("Subscribe: expected one message before EOF, got none")
	}
	msg := stream.Msg()
	if msg.GetEvent() != pb.DGEvent_CANTDOTHIS || msg.GetError() != pb.DGError_UNAUTHED {
		t.Errorf("want CANTDOTHIS+UNAUTHED, got event=%v error=%v", msg.GetEvent(), msg.GetError())
	}

	// ストリームが終了していること (次の Receive は false)。
	if stream.Receive() {
		t.Error("expected stream to end after CANTDOTHIS+UNAUTHED")
	}
}

// TestSubscribeConnect は Subscribe の最初のリクエストを CONNECT にすると
// 最初の受信が CONNECT レスポンスでトークンが空でないことを確認する。
func TestSubscribeConnect(t *testing.T) {
	_, _, _, c, _ := setup(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := connect.NewRequest(&pb.DGRequest{
		Version: 1,
		Event:   pb.DGEvent_CONNECT,
		Connect: &pb.DGRequest_DGConnect{AppName: "sub-tester", Uuid: "uuid-sub"},
	})

	stream, err := c.Subscribe(ctx, req)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer stream.Close()

	// 最初のメッセージが CONNECT レスポンスでトークンが空でないこと。
	if !stream.Receive() {
		t.Fatal("Subscribe: expected CONNECT response, got EOF")
	}
	msg := stream.Msg()
	if msg.GetEvent() != pb.DGEvent_CONNECT {
		t.Errorf("want CONNECT event, got %v", msg.GetEvent())
	}
	if tok := msg.GetConnect().GetToken(); tok == "" {
		t.Error("CONNECT response should have non-empty token")
	}

	// ctx をキャンセルしてストリームを終了。
	cancel()
}
