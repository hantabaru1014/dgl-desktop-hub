package main

import (
	"embed"
	"errors"
	"log"
	"net"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/hantabaru1014/dgl-desktop-hub/internal/device"
	"github.com/hantabaru1014/dgl-desktop-hub/internal/hubcore"
	"github.com/hantabaru1014/dgl-desktop-hub/services"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// ハブ中核を構築。
	hub := hubcore.NewHub()

	hubService := services.NewHubService(hub)

	// アプリ操作側 (OpenDGLab Connect + 素の POST) のサーバを永続化ポートで起動。
	if err := hubService.StartAppServer(); err != nil {
		log.Printf("failed to start app proto server: %v", err)
	}

	// イベントの型登録 (Wails の TS バインディング生成に使われる)。
	application.RegisterEvent[[]services.DeviceDTO](services.EventDevicesChanged)
	application.RegisterEvent[[]device.GraphSample](services.EventGraphFrame)
	application.RegisterEvent[[]services.AppDTO](services.EventAppsChanged)
	application.RegisterEvent[services.ApprovalRequestDTO](services.EventApprovalRequest)

	app := application.New(application.Options{
		Name:        "dgl-desktop-hub",
		Description: "DG-LAB Coyote v3 を中継・操作するハブ",
		Services: []application.Service{
			application.NewService(hubService),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	// 出力ループを開始。
	hub.Start()

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "DG-LAB Desktop Hub",
		Width:            1100,
		Height:           760,
		BackgroundColour: application.NewRGB(17, 24, 39),
		URL:              "/",
	})

	if err := app.Run(); err != nil && !errors.Is(err, net.ErrClosed) {
		log.Fatal(err)
	}
}
