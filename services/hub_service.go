// Package services は Wails フロントエンドへ公開するサービスを提供する。
// Wails を import する唯一のパッケージ。
package services

import (
	"context"
	"fmt"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/hantabaru1014/dgl-desktop-hub/internal/appproto"
	"github.com/hantabaru1014/dgl-desktop-hub/internal/coyote/ble"
	"github.com/hantabaru1014/dgl-desktop-hub/internal/device"
	"github.com/hantabaru1014/dgl-desktop-hub/internal/hubcore"
	"github.com/hantabaru1014/dgl-desktop-hub/internal/socketserver"
	"github.com/hantabaru1014/dgl-desktop-hub/internal/waveform"
)

// イベント名。フロントエンドはこれらを購読する。
const (
	EventDevicesChanged  = "devices:changed"
	EventGraphFrame      = "graph:frame"
	EventAppsChanged     = "apps:changed"
	EventApprovalRequest = "app:approval"
)

// HubService はハブ全体をフロントエンドへ公開する Wails サービス。
type HubService struct {
	hub       *hubcore.Hub
	appSrv    *appproto.Server
	socketSrv *socketserver.Server
	settings  *settingsStore
}

// NewHubService は HubService を生成し、ハブのイベントシンクを配線する。
func NewHubService(hub *hubcore.Hub) *HubService {
	s := &HubService{hub: hub, appSrv: appproto.NewServer(hub), settings: newSettingsStore()}
	s.socketSrv = socketserver.NewServer(socketserver.Callbacks{
		OnConnect:    func(c device.CoyoteDevice) { s.hub.Mgr.Add(c) },
		OnDisconnect: func(id device.DeviceID) { _ = s.hub.Mgr.Remove(id) },
	})
	hub.SetEventSink(hubcore.EventSink{
		OnGraph:           s.emitGraph,
		OnDevicesChanged:  s.emitDevicesChanged,
		OnAppsChanged:     s.emitAppsChanged,
		OnApprovalRequest: s.emitApprovalRequest,
	})
	return s
}

// ServiceName は Wails 用のサービス名。
func (s *HubService) ServiceName() string { return "HubService" }

// StartAppServer はアプリ操作側 (OpenDGLab) サーバを永続化ポートで起動する。
// アプリ起動時に main から呼ばれる。
func (s *HubService) StartAppServer() error {
	return s.appSrv.Start(s.settings.AppPort())
}

// SetAppServerPort はアプリ操作側ポートを変更し永続化、サーバを再起動する。
func (s *HubService) SetAppServerPort(port int) (ServerInfoDTO, error) {
	if port <= 0 || port > 65535 {
		return ServerInfoDTO{}, fmt.Errorf("invalid port: %d", port)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = s.appSrv.Stop(ctx)
	if err := s.appSrv.Start(port); err != nil {
		return ServerInfoDTO{}, err
	}
	s.settings.SetAppPort(port)
	return s.GetServerInfo(), nil
}

// ServiceShutdown はアプリ終了時に呼ばれる。
func (s *HubService) ServiceShutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = s.appSrv.Stop(ctx)
	_ = s.socketSrv.Stop()
	s.hub.Stop()
	return nil
}

// StartSocketServer は socket mode サーバを起動し、QR と待ち受け情報を返す。
// port<=0 なら永続化された socket ポートを使う。
func (s *HubService) StartSocketServer(port int) (SocketServerDTO, error) {
	if port <= 0 {
		port = s.settings.SocketPort()
	}
	if err := s.socketSrv.Start("", port); err != nil {
		return SocketServerDTO{}, err
	}
	s.settings.SetSocketPort(port)
	return s.socketServerDTO(), nil
}

// StopSocketServer は socket mode サーバを停止する。
func (s *HubService) StopSocketServer() error {
	return s.socketSrv.Stop()
}

// GetSocketServerInfo は socket mode サーバの現在状態を返す。
func (s *HubService) GetSocketServerInfo() SocketServerDTO {
	return s.socketServerDTO()
}

func (s *HubService) socketServerDTO() SocketServerDTO {
	host := s.settings.SocketHost()
	if host == "" {
		host = socketserver.LANIP()
	}
	return SocketServerDTO{
		Running:      s.socketSrv.Running(),
		Addr:         s.socketSrv.Addr(),
		QR:           s.socketSrv.QR(),
		Port:         s.settings.SocketPort(),
		Host:         host,
		ControllerID: s.socketSrv.ControllerID(),
	}
}

// SetSocketHost は socket QR に使うホストを保存する。
func (s *HubService) SetSocketHost(host string) error {
	s.settings.SetSocketHost(host)
	return nil
}

// --- イベント emit ---

func emit(name string, data any) {
	if app := application.Get(); app != nil {
		app.Event.Emit(name, data)
	}
}

func (s *HubService) emitGraph(samples []device.GraphSample) {
	emit(EventGraphFrame, samples)
}

func (s *HubService) emitDevicesChanged(_ []device.Info) {
	emit(EventDevicesChanged, s.ListDevices())
}

func (s *HubService) emitAppsChanged(apps []hubcore.AppInfo) {
	dtos := make([]AppDTO, 0, len(apps))
	for _, a := range apps {
		dtos = append(dtos, appDTO(a))
	}
	emit(EventAppsChanged, dtos)
}

func (s *HubService) emitApprovalRequest(r hubcore.ApprovalRequest) {
	emit(EventApprovalRequest, ApprovalRequestDTO{RequestID: r.RequestID, AppName: r.AppName, UUID: r.UUID})
}

// ApproveConnection はモーダルでの承認/拒否を反映する。
func (s *HubService) ApproveConnection(requestID string, approve bool) error {
	s.hub.ResolveApproval(requestID, approve)
	return nil
}

// --- デバイス操作 ---

// ListDevices は全デバイスを返す。
func (s *HubService) ListDevices() []DeviceDTO {
	infos := s.hub.Mgr.List()
	out := make([]DeviceDTO, 0, len(infos))
	for _, in := range infos {
		out = append(out, s.buildDTO(in))
	}
	return out
}

// buildDTO はデバイス情報 + ソフトリミット + 現在の強度/波形を集めて DTO 化する。
func (s *HubService) buildDTO(in device.Info) DeviceDTO {
	lim, _ := s.hub.Mgr.SoftLimitOf(in.ID)
	d := deviceDTO(in, lim)
	ta, tb, _, _, _, err := s.hub.Mgr.Snapshot(in.ID)
	if err == nil {
		d.StrengthA, d.StrengthB = int(ta), int(tb)
	}
	d.Exclusive = s.hub.IsExclusive(in.ID)
	d.Owner = s.hub.OwnerName(in.ID)
	if wa, wb, err := s.hub.Mgr.WaveNames(in.ID); err == nil {
		d.WaveA, d.WaveB = wa, wb
	}
	return d
}

// AddDemoDevice は疑似デバイスを追加して返す。
func (s *HubService) AddDemoDevice(name string) (DeviceDTO, error) {
	d := device.NewDemoDevice(name)
	id := s.hub.Mgr.Add(d)
	s.applyStoredLimit(id)
	return s.deviceDTOByID(id), nil
}

// RemoveDevice はデバイスを削除する。
func (s *HubService) RemoveDevice(id string) error {
	return s.hub.Mgr.Remove(device.DeviceID(id))
}

// ScanBLE は seconds 秒間 BLE をスキャンし、Coyote v3 候補を返す。
func (s *HubService) ScanBLE(seconds int) ([]ScanResultDTO, error) {
	if seconds <= 0 {
		seconds = 5
	}
	if seconds > 30 {
		seconds = 30
	}
	results, err := ble.Scan(time.Duration(seconds) * time.Second)
	if err != nil {
		return nil, err
	}
	out := make([]ScanResultDTO, 0, len(results))
	for _, r := range results {
		out = append(out, ScanResultDTO{Address: r.Address, Name: r.Name, RSSI: r.RSSI})
	}
	return out, nil
}

// ConnectBLE はアドレスを指定して BLE Coyote へ接続し、デバイスを追加する。
// 直前に ScanBLE を実行している必要がある。
func (s *HubService) ConnectBLE(addr, name string) (DeviceDTO, error) {
	dev, err := ble.NewBLECoyote(addr, name)
	if err != nil {
		return DeviceDTO{}, err
	}
	if err := dev.Connect(context.Background()); err != nil {
		return DeviceDTO{}, err
	}
	id := s.hub.Mgr.Add(dev)
	s.applyStoredLimit(id)
	return s.deviceDTOByID(id), nil
}

func (s *HubService) deviceDTOByID(id device.DeviceID) DeviceDTO {
	for _, in := range s.hub.Mgr.List() {
		if in.ID == id {
			return s.buildDTO(in)
		}
	}
	return DeviceDTO{ID: string(id)}
}

// SetSoftLimit はチャンネル毎の強度ソフトリミットを設定し、永続化する。
func (s *HubService) SetSoftLimit(id string, a, b int) error {
	if err := s.hub.Mgr.SetSoftLimit(device.DeviceID(id), device.SoftLimit{
		A: device.ClampStrength(a), B: device.ClampStrength(b),
	}); err != nil {
		return err
	}
	s.settings.SetSoftLimit(id, int(device.ClampStrength(a)), int(device.ClampStrength(b)))
	return nil
}

// applyStoredLimit は保存済みソフトリミット/排他モードがあればデバイスへ適用する。
func (s *HubService) applyStoredLimit(id device.DeviceID) {
	if a, b, ok := s.settings.GetSoftLimit(string(id)); ok {
		_ = s.hub.Mgr.SetSoftLimit(id, device.SoftLimit{A: device.ClampStrength(a), B: device.ClampStrength(b)})
	}
	if v, ok := s.settings.GetExclusive(string(id)); ok {
		s.hub.SetExclusive(id, v)
	}
}

// SetDeviceExclusive はデバイスの排他モードを設定し永続化する。
func (s *HubService) SetDeviceExclusive(id string, exclusive bool) error {
	s.hub.SetExclusive(device.DeviceID(id), exclusive)
	s.settings.SetExclusive(id, exclusive)
	// UI へ反映。
	s.emitDevicesChanged(nil)
	return nil
}

// SetStrength は強度を設定する。channel: 0=A,1=B。mode: 0=絶対,1=相対増,2=相対減。
func (s *HubService) SetStrength(id string, channel, mode, val int) error {
	ch, err := toChannel(channel)
	if err != nil {
		return err
	}
	m, err := toMode(mode)
	if err != nil {
		return err
	}
	return s.hub.Mgr.SetStrength(device.DeviceID(id), ch, m, device.ClampStrength(val))
}

// SetWaveformPreset はプリセット名で波形を設定する。channel: 0=A,1=B,-1=両方。
func (s *HubService) SetWaveformPreset(id string, channel int, presetName string) error {
	p, ok := waveform.PresetByName(presetName)
	if !ok {
		return fmt.Errorf("preset not found: %s", presetName)
	}
	for _, ch := range channelList(channel) {
		if err := s.hub.Mgr.SetWaveform(device.DeviceID(id), ch, p.Waveform); err != nil {
			return err
		}
	}
	return nil
}

// ClearWaveform は波形を無出力にする。channel: 0=A,1=B,-1=両方。
func (s *HubService) ClearWaveform(id string, channel int) error {
	for _, ch := range channelList(channel) {
		if err := s.hub.Mgr.ClearWaveform(device.DeviceID(id), ch); err != nil {
			return err
		}
	}
	return nil
}

// --- プリセット / アクセスモード / サーバ情報 ---

// GetPresets は波形プリセット一覧を返す。
func (s *HubService) GetPresets() []PresetDTO {
	names, _ := waveform.PresetNames()
	out := make([]PresetDTO, len(names))
	for i, n := range names {
		out[i] = PresetDTO{Name: n}
	}
	return out
}

// GetAccessMode は現在のアクセスモード (接続自動許可) を返す。
func (s *HubService) GetAccessMode() AccessModeDTO {
	return AccessModeDTO{AutoApprove: s.hub.AutoApprove()}
}

// SetAutoApprove は接続自動許可を設定する。
func (s *HubService) SetAutoApprove(autoApprove bool) error {
	s.hub.SetAutoApprove(autoApprove)
	return nil
}

// ListApps は接続中アプリ一覧を返す。
func (s *HubService) ListApps() []AppDTO {
	apps := s.hub.Apps()
	out := make([]AppDTO, 0, len(apps))
	for _, a := range apps {
		out = append(out, appDTO(a))
	}
	return out
}

// GetServerInfo はアプリ操作側の待ち受け情報を返す。
// 表示 URL は既定で localhost (サーバは 0.0.0.0 で待ち受けるため LAN からも到達可)。
func (s *HubService) GetServerInfo() ServerInfoDTO {
	running := s.appSrv.Running()
	port := s.settings.AppPort() // 設定値 (編集ボックスの既定)
	if running {
		port = s.appSrv.Port()
	}
	addr := fmt.Sprintf("localhost:%d", port)
	return ServerInfoDTO{
		Connect: "http://" + addr,
		Port:    port,
		Running: running,
	}
}

// --- helpers ---

func toChannel(c int) (device.Channel, error) {
	switch c {
	case 0:
		return device.ChannelA, nil
	case 1:
		return device.ChannelB, nil
	default:
		return 0, fmt.Errorf("invalid channel: %d", c)
	}
}

func channelList(c int) []device.Channel {
	switch c {
	case 0:
		return []device.Channel{device.ChannelA}
	case 1:
		return []device.Channel{device.ChannelB}
	default:
		return []device.Channel{device.ChannelA, device.ChannelB}
	}
}

func toMode(m int) (device.StrengthMode, error) {
	switch m {
	case 0:
		return device.StrengthAbsolute, nil
	case 1:
		return device.StrengthRelativeInc, nil
	case 2:
		return device.StrengthRelativeDec, nil
	default:
		return 0, fmt.Errorf("invalid mode: %d", m)
	}
}
