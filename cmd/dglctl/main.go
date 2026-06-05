// dglctl は OpenDGLab プロトコル経由でハブを操作する簡易 CLI (動作確認用)。
//
//	go run ./cmd/dglctl            # 接続してデバイス一覧を表示
//	go run ./cmd/dglctl drive      # 先頭デバイスをプリセット+強度ランプで駆動
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"connectrpc.com/connect"

	pb "github.com/hantabaru1014/dgl-desktop-hub/internal/pb/com/github/opendglab"
	"github.com/hantabaru1014/dgl-desktop-hub/internal/pb/com/github/opendglab/opendglabconnect"
)

const baseURL = "http://127.0.0.1:7330"

func main() {
	cmd := "list"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	c := opendglabconnect.NewOpenDGLabServiceClient(http.DefaultClient, baseURL)
	ctx := context.Background()

	var token string
	send := func(req *pb.DGRequest) *pb.DGResponse {
		r := connect.NewRequest(req)
		if token != "" {
			r.Header().Set("X-DGLab-Token", token)
		}
		resp, err := c.Send(ctx, r)
		if err != nil {
			fmt.Println("ERROR:", err)
			os.Exit(1)
		}
		return resp.Msg
	}

	// CONNECT
	cr := send(&pb.DGRequest{Version: 1, Event: pb.DGEvent_CONNECT,
		Connect: &pb.DGRequest_DGConnect{AppName: "dglctl", Uuid: "dglctl-1"}})
	token = cr.GetConnect().GetToken()
	if token == "" {
		fmt.Println("接続が拒否されました (ハブ側で「許可」されませんでした)。")
		os.Exit(1)
	}
	fmt.Println("connected, token:", token)

	// GETDEVICE
	dl := send(&pb.DGRequest{Version: 1, Event: pb.DGEvent_GETDEVICE})
	devs := dl.GetDeviceList().GetDevices()
	fmt.Printf("devices (%d):\n", len(devs))
	for _, d := range devs {
		fmt.Printf("  - %s (lockedByMe=%v)\n", d.GetId(), d.GetIsLockedByMe())
	}
	if len(devs) == 0 {
		fmt.Println("デバイスがありません。UI で Demo デバイスを追加してください。")
		return
	}

	// WAVELIST
	wl := send(&pb.DGRequest{Version: 1, Event: pb.DGEvent_GETWAVELIST})
	waves := wl.GetWaveList().GetWave()
	fmt.Printf("presets (%d): %v\n", len(waves), waves)

	if cmd != "drive" {
		return
	}

	id := devs[0].GetId()
	fmt.Println("driving device:", id)

	// 両チャンネルに2つ目の波形を設定。
	send(&pb.DGRequest{Version: 1, Event: pb.DGEvent_SETWAVE,
		Device: &pb.DGRequest_DGDeviceID{DeviceId: id},
		Wave:   &pb.DGRequest_DGWave{WaveName: waves[1]}})
	fmt.Println("set wave: Tide (A+B)")

	// 強度を 0→120 へランプ (ソフトリミットで頭打ちになる様子も観察できる)。
	for v := int32(0); v <= 120; v += 10 {
		send(&pb.DGRequest{Version: 1, Event: pb.DGEvent_SETSTRENGTH,
			Device:   &pb.DGRequest_DGDeviceID{DeviceId: id},
			Strength: &pb.DGRequest_DGStrength{StrengthA: v, StrengthB: v}})
		gs := send(&pb.DGRequest{Version: 1, Event: pb.DGEvent_GETSTRENGTH,
			Device: &pb.DGRequest_DGDeviceID{DeviceId: id}})
		fmt.Printf("  set A=B=%d -> reported target A=%d B=%d\n",
			v, gs.GetStrength().GetStrengthA(), gs.GetStrength().GetStrengthB())
		time.Sleep(700 * time.Millisecond)
	}
	fmt.Println("done. グラフが動いていれば成功です。")
}
