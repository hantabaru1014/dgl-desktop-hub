package appproto

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"

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

func send(t *testing.T, c opendglabconnect.OpenDGLabServiceClient, req *pb.DGRequest) *pb.DGResponse {
	t.Helper()
	resp, err := c.Send(context.Background(), connect.NewRequest(req))
	if err != nil {
		t.Fatalf("Send(%v): %v", req.GetEvent(), err)
	}
	return resp.Msg
}

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

// TestExclusiveMode は排他モードで先着アプリのみ操作でき、別アプリが弾かれる
// ことを確認する。
func TestExclusiveMode(t *testing.T) {
	hub, _, id, c1, srv := setup(t)
	hub.SetExclusive(id, true)
	c2 := opendglabconnect.NewOpenDGLabServiceClient(srv.Client(), srv.URL)

	tok1 := send(t, c1, &pb.DGRequest{Version: 1, Event: pb.DGEvent_CONNECT, Connect: &pb.DGRequest_DGConnect{AppName: "a1", Uuid: "u1"}}).GetConnect().GetToken()
	tok2 := send(t, c2, &pb.DGRequest{Version: 1, Event: pb.DGEvent_CONNECT, Connect: &pb.DGRequest_DGConnect{AppName: "a2", Uuid: "u2"}}).GetConnect().GetToken()

	// app1 が先に操作 → owner を claim、成功。
	r1 := sendTok(t, c1, &pb.DGRequest{Version: 1, Event: pb.DGEvent_SETSTRENGTH,
		Device: &pb.DGRequest_DGDeviceID{DeviceId: string(id)}, Strength: &pb.DGRequest_DGStrength{StrengthA: 10}}, tok1)
	if r1.GetEvent() == pb.DGEvent_CANTDOTHIS {
		t.Fatalf("app1 should control, got error %v", r1.GetError())
	}
	// app2 は弾かれる。
	r2 := sendTok(t, c2, &pb.DGRequest{Version: 1, Event: pb.DGEvent_SETSTRENGTH,
		Device: &pb.DGRequest_DGDeviceID{DeviceId: string(id)}, Strength: &pb.DGRequest_DGStrength{StrengthA: 99}}, tok2)
	if r2.GetEvent() != pb.DGEvent_CANTDOTHIS || r2.GetError() != pb.DGError_DEVICENOTLOCKBYYOU {
		t.Errorf("app2 should be blocked, got %v / %v", r2.GetEvent(), r2.GetError())
	}

	ta, _, _, _, _, _ := hub.Mgr.Snapshot(id)
	if ta != 10 {
		t.Errorf("strength = %d, want 10 (only app1)", ta)
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

	resp := send(t, c, &pb.DGRequest{Version: 1, Event: pb.DGEvent_CONNECT,
		Connect: &pb.DGRequest_DGConnect{AppName: "tester", Uuid: "uuid-1"}})
	if resp.GetConnect().GetToken() == "" {
		t.Fatal("CONNECT returned empty token")
	}

	resp = send(t, c, &pb.DGRequest{Version: 1, Event: pb.DGEvent_GETDEVICE})
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

	send(t, c, &pb.DGRequest{Version: 1, Event: pb.DGEvent_SETSTRENGTH,
		Device:   &pb.DGRequest_DGDeviceID{DeviceId: string(id)},
		Strength: &pb.DGRequest_DGStrength{StrengthA: 42, StrengthB: 77}})

	ta, tb, _, _, _, err := hub.Mgr.Snapshot(id)
	if err != nil {
		t.Fatal(err)
	}
	if ta != 42 || tb != 77 {
		t.Errorf("target = (%d,%d), want (42,77)", ta, tb)
	}

	resp := send(t, c, &pb.DGRequest{Version: 1, Event: pb.DGEvent_GETSTRENGTH,
		Device: &pb.DGRequest_DGDeviceID{DeviceId: string(id)}})
	if resp.GetStrength().GetStrengthA() != 42 {
		t.Errorf("GETSTRENGTH A = %d, want 42", resp.GetStrength().GetStrengthA())
	}
}

// TestSoftLimitEnforcedAtOutput は出力ループでソフトリミットが効くことを
// 実 tick (hub.Start) を回して確認する。
func TestSoftLimitEnforcedAtOutput(t *testing.T) {
	hub, d, id, c, _ := setup(t)
	_ = hub.Mgr.SetSoftLimit(id, device.SoftLimit{A: 25, B: 25})

	send(t, c, &pb.DGRequest{Version: 1, Event: pb.DGEvent_SETSTRENGTH,
		Device:   &pb.DGRequest_DGDeviceID{DeviceId: string(id)},
		Strength: &pb.DGRequest_DGStrength{StrengthA: 200, StrengthB: 200}})

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

	resp := send(t, c, &pb.DGRequest{Version: 1, Event: pb.DGEvent_GETWAVELIST})
	names := resp.GetWaveList().GetWave()
	if len(names) != 16 {
		t.Fatalf("wave list = %d, want 16", len(names))
	}

	send(t, c, &pb.DGRequest{Version: 1, Event: pb.DGEvent_SETWAVE,
		Device: &pb.DGRequest_DGDeviceID{DeviceId: string(id), DeviceChannel: pb.DGDeviceChannel_CHANNEL_A},
		Wave:   &pb.DGRequest_DGWave{WaveName: "Breathing"}})

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

	send(t, c1, &pb.DGRequest{Version: 1, Event: pb.DGEvent_CONNECT, Connect: &pb.DGRequest_DGConnect{AppName: "app1", Uuid: "u1"}})
	send(t, c2, &pb.DGRequest{Version: 1, Event: pb.DGEvent_CONNECT, Connect: &pb.DGRequest_DGConnect{AppName: "app2", Uuid: "u2"}})

	send(t, c1, &pb.DGRequest{Version: 1, Event: pb.DGEvent_SETSTRENGTH,
		Device: &pb.DGRequest_DGDeviceID{DeviceId: string(id)}, Strength: &pb.DGRequest_DGStrength{StrengthA: 10}})
	send(t, c2, &pb.DGRequest{Version: 1, Event: pb.DGEvent_SETSTRENGTH,
		Device: &pb.DGRequest_DGDeviceID{DeviceId: string(id)}, Strength: &pb.DGRequest_DGStrength{StrengthA: 20}})

	ta, _, _, _, _, _ := hub.Mgr.Snapshot(id)
	if ta != 20 {
		t.Errorf("last-writer-wins target A = %d, want 20", ta)
	}
	if got := len(hub.Apps()); got != 2 {
		t.Errorf("connected apps = %d, want 2", got)
	}
}
