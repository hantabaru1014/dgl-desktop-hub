package services

import (
	"github.com/hantabaru1014/dgl-desktop-hub/internal/device"
	"github.com/hantabaru1014/dgl-desktop-hub/internal/hubcore"
)

// DeviceDTO はフロントエンドへ渡すデバイス情報 (フラット)。
type DeviceDTO struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	Battery    int    `json:"battery"` // 0..100、不明なら -1
	SoftLimitA int    `json:"softLimitA"`
	SoftLimitB int    `json:"softLimitB"`
	StrengthA  int    `json:"strengthA"` // 現在の目標強度 (clamp 前)
	StrengthB  int    `json:"strengthB"`
	WaveA      string `json:"waveA"` // 現在の波形名 (なければ空)
	WaveB      string `json:"waveB"`
	Exclusive  bool   `json:"exclusive"` // 排他モード (true で単一アプリのみ操作可)
	Owner      string `json:"owner"`     // 排他モード時に専有しているアプリ名 (なければ空)
}

// PresetDTO は波形プリセット情報。
type PresetDTO struct {
	Name string `json:"name"`
}

// AccessModeDTO はアクセスモード設定。
type AccessModeDTO struct {
	AutoApprove bool `json:"autoApprove"`
}

// ApprovalRequestDTO は接続許可要求 (UI モーダル用)。
type ApprovalRequestDTO struct {
	RequestID string `json:"requestId"`
	AppName   string `json:"appName"`
	UUID      string `json:"uuid"`
}

// AppDTO は接続中アプリ情報。
type AppDTO struct {
	Token   string `json:"token"`
	AppName string `json:"appName"`
	UUID    string `json:"uuid"`
}

// ScanResultDTO は BLE スキャンで見つかったデバイス。
type ScanResultDTO struct {
	Address string `json:"address"`
	Name    string `json:"name"`
	RSSI    int    `json:"rssi"`
}

// SocketServerDTO は socket mode サーバの状態。
type SocketServerDTO struct {
	Running      bool   `json:"running"`
	Addr         string `json:"addr"`
	QR           string `json:"qr"`
	Port         int    `json:"port"`
	Host         string `json:"host"`         // QR に使うホスト (ユーザー編集可)
	ControllerID string `json:"controllerId"` // QR 文字列をフロントで組むための ID
}

// ServerInfoDTO はアプリ操作側の待ち受け情報。
type ServerInfoDTO struct {
	Connect string `json:"connect"` // Connect/gRPC/gRPC-Web のベース URL
	Port    int    `json:"port"`
	Running bool   `json:"running"`
}

func batteryToInt(b uint8) int {
	if b == device.BatteryUnknown {
		return -1
	}
	return int(b)
}

func deviceDTO(in device.Info, lim device.SoftLimit) DeviceDTO {
	return DeviceDTO{
		ID:         string(in.ID),
		Kind:       in.Kind.String(),
		Name:       in.Name,
		Status:     in.Status.String(),
		Battery:    batteryToInt(in.Battery),
		SoftLimitA: int(lim.A),
		SoftLimitB: int(lim.B),
	}
}

func appDTO(a hubcore.AppInfo) AppDTO {
	return AppDTO{Token: a.Token, AppName: a.AppName, UUID: a.UUID}
}
