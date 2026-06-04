package hubcore

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	"github.com/hantabaru1014/dgl-desktop-hub/internal/device"
	pb "github.com/hantabaru1014/dgl-desktop-hub/internal/pb/com/github/opendglab"
	"github.com/hantabaru1014/dgl-desktop-hub/internal/waveform"
)

// HandleSend は OpenDGLab プロトコルの 1 リクエストを処理する。transport 非依存で、
// Connect ハンドラと素の HTTP POST shim の両方から呼ばれる。
// sessionToken はストリーミングセッションのトークン (無ければ空文字)。
func (h *Hub) HandleSend(_ context.Context, req *pb.DGRequest, sessionToken string) (*pb.DGResponse, error) {
	// トークン付きリクエストはアプリの最終通信時刻を更新 (アイドル切断回避)。
	h.touchApp(sessionToken)
	switch req.GetEvent() {
	case pb.DGEvent_CONNECT:
		return h.handleConnect(req), nil
	case pb.DGEvent_PING:
		return &pb.DGResponse{Version: 1, Event: pb.DGEvent_PING}, nil
	case pb.DGEvent_GETDEVICE:
		return h.handleGetDevice(sessionToken), nil
	case pb.DGEvent_LOCKDEVICE:
		return h.handleLock(req, sessionToken, true), nil
	case pb.DGEvent_UNLOCKDEVICE:
		return h.handleLock(req, sessionToken, false), nil
	case pb.DGEvent_GETSTRENGTH:
		return h.handleGetStrength(req), nil
	case pb.DGEvent_SETSTRENGTH:
		return h.handleSetStrength(req, sessionToken), nil
	case pb.DGEvent_GETWAVELIST:
		return h.handleGetWaveList(), nil
	case pb.DGEvent_GETWAVE:
		return h.handleGetWave(req), nil
	case pb.DGEvent_SETWAVE:
		return h.handleSetWave(req, sessionToken), nil
	case pb.DGEvent_CUSTOMWAVE:
		return h.handleCustomWave(req, sessionToken), nil
	case pb.DGEvent_CLEARCUSTOM:
		return h.handleClearCustom(req, sessionToken), nil
	default:
		return cantDoThis(pb.DGError_UNKNOWN, ""), nil
	}
}

func (h *Hub) handleConnect(req *pb.DGRequest) *pb.DGResponse {
	c := req.GetConnect()
	existing := c.GetToken()

	// 非 auto-approve で、未承認の新規接続ならユーザーへ許可を尋ねる。
	// 既存トークンでの再接続は承認済みとみなす。
	if !h.AutoApprove() && !h.isKnownApp(existing) {
		if !h.requestApproval(c.GetAppName(), c.GetUuid()) {
			// 拒否/タイムアウト: 空トークンを返す (OpenDGLab 仕様の拒否)。
			return &pb.DGResponse{
				Version: 1,
				Event:   pb.DGEvent_CONNECT,
				Connect: &pb.DGResponse_DGConnect{Token: ""},
			}
		}
	}

	token := existing
	if token == "" {
		token = newToken()
	}
	h.registerApp(AppInfo{Token: token, AppName: c.GetAppName(), UUID: c.GetUuid()})
	return &pb.DGResponse{
		Version: 1,
		Event:   pb.DGEvent_CONNECT,
		Connect: &pb.DGResponse_DGConnect{Token: token},
	}
}

func (h *Hub) handleGetDevice(sessionToken string) *pb.DGResponse {
	infos := h.Mgr.List()
	h.mu.RLock()
	devs := make([]*pb.DGResponse_DGDevice, 0, len(infos))
	for _, in := range infos {
		var lockedByRemote, lockedByMe bool
		if !h.exclusive[in.ID] {
			// 非排他: 全アプリが操作可能。
			lockedByMe = true
		} else {
			owner := h.owners[in.ID]
			lockedByRemote = owner != "" && owner != sessionToken
			lockedByMe = owner != "" && owner == sessionToken
		}
		devs = append(devs, &pb.DGResponse_DGDevice{
			Id:               string(in.ID),
			IsLockedByRemote: lockedByRemote,
			IsLockedByMe:     lockedByMe,
		})
	}
	h.mu.RUnlock()
	return &pb.DGResponse{
		Version:    1,
		Event:      pb.DGEvent_GETDEVICE,
		DeviceList: &pb.DGResponse_DGDeviceList{Devices: devs},
	}
}

func (h *Hub) handleLock(req *pb.DGRequest, sessionToken string, lock bool) *pb.DGResponse {
	id := device.DeviceID(req.GetDevice().GetDeviceId())
	if !h.Mgr.Has(id) {
		return cantDoThis(pb.DGError_DEVICEOFFLINE, string(id))
	}
	h.mu.Lock()
	if !h.exclusive[id] {
		// 非排他: lock は no-op 成功、常に自分が保持していると返す。
		h.mu.Unlock()
		return lockedDeviceResp(id, false, true)
	}
	owner := h.owners[id]
	if lock {
		if owner != "" && owner != sessionToken {
			h.mu.Unlock()
			return cantDoThis(pb.DGError_DEVICENOTLOCKBYYOU, string(id))
		}
		h.owners[id] = sessionToken
		h.mu.Unlock()
		h.notifyDevicesChanged()
		return lockedDeviceResp(id, true, true)
	}
	// unlock
	if owner != "" && owner != sessionToken {
		h.mu.Unlock()
		return cantDoThis(pb.DGError_DEVICENOTLOCKBYYOU, string(id))
	}
	delete(h.owners, id)
	h.mu.Unlock()
	h.notifyDevicesChanged()
	return lockedDeviceResp(id, false, false)
}

// canControl は SETSTRENGTH/SETWAVE 等の操作可否を判定する。
// 排他モードで owner 未設定なら先着で claim する。
func (h *Hub) canControl(id device.DeviceID, sessionToken string) bool {
	h.mu.Lock()
	if !h.exclusive[id] {
		h.mu.Unlock()
		return true
	}
	owner := h.owners[id]
	if owner == "" {
		claimed := false
		if sessionToken != "" {
			h.owners[id] = sessionToken
			claimed = true
		}
		h.mu.Unlock()
		if claimed {
			h.notifyDevicesChanged() // 専有アプリ表示を更新
		}
		return true
	}
	allowed := owner == sessionToken
	h.mu.Unlock()
	return allowed
}

func (h *Hub) handleGetStrength(req *pb.DGRequest) *pb.DGResponse {
	id := device.DeviceID(req.GetDevice().GetDeviceId())
	ta, tb, _, _, _, err := h.Mgr.Snapshot(id)
	if err != nil {
		return cantDoThis(pb.DGError_DEVICEOFFLINE, string(id))
	}
	return &pb.DGResponse{
		Version:  1,
		Event:    pb.DGEvent_GETSTRENGTH,
		DeviceId: &pb.DGResponse_DGDeviceID{DeviceId: string(id)},
		Strength: &pb.DGResponse_DGDeviceStrength{StrengthA: int32(ta), StrengthB: int32(tb)},
	}
}

func (h *Hub) handleSetStrength(req *pb.DGRequest, sessionToken string) *pb.DGResponse {
	id := device.DeviceID(req.GetDevice().GetDeviceId())
	if !h.Mgr.Has(id) {
		return cantDoThis(pb.DGError_DEVICEOFFLINE, string(id))
	}
	if !h.canControl(id, sessionToken) {
		return cantDoThis(pb.DGError_DEVICENOTLOCKBYYOU, string(id))
	}
	s := req.GetStrength()
	_ = h.Mgr.SetStrength(id, device.ChannelA, device.StrengthAbsolute, clampByte(s.GetStrengthA()))
	_ = h.Mgr.SetStrength(id, device.ChannelB, device.StrengthAbsolute, clampByte(s.GetStrengthB()))
	// 他アプリへ強度変化を push。
	h.broadcast(h.handleGetStrength(req))
	return okResp(pb.DGEvent_SETSTRENGTH)
}

func (h *Hub) handleGetWaveList() *pb.DGResponse {
	names, _ := waveform.PresetNames()
	return &pb.DGResponse{
		Version:  1,
		Event:    pb.DGEvent_GETWAVELIST,
		WaveList: &pb.DGResponse_DGWaveList{Wave: names},
	}
}

func (h *Hub) handleGetWave(req *pb.DGRequest) *pb.DGResponse {
	id := device.DeviceID(req.GetDevice().GetDeviceId())
	a, b, err := h.Mgr.WaveNames(id)
	if err != nil {
		return cantDoThis(pb.DGError_DEVICEOFFLINE, string(id))
	}
	name := a
	if req.GetDevice().GetDeviceChannel() == pb.DGDeviceChannel_CHANNEL_B {
		name = b
	}
	return &pb.DGResponse{
		Version:  1,
		Event:    pb.DGEvent_GETWAVE,
		DeviceId: &pb.DGResponse_DGDeviceID{DeviceId: string(id), DeviceChannel: req.GetDevice().GetDeviceChannel()},
		WaveName: &pb.DGResponse_DGWave{Wave: name},
	}
}

func (h *Hub) handleSetWave(req *pb.DGRequest, sessionToken string) *pb.DGResponse {
	id := device.DeviceID(req.GetDevice().GetDeviceId())
	if !h.Mgr.Has(id) {
		return cantDoThis(pb.DGError_DEVICEOFFLINE, string(id))
	}
	if !h.canControl(id, sessionToken) {
		return cantDoThis(pb.DGError_DEVICENOTLOCKBYYOU, string(id))
	}
	p, ok := waveform.PresetByName(req.GetWave().GetWaveName())
	if !ok {
		return cantDoThis(pb.DGError_UNKNOWN, string(id))
	}
	for _, ch := range channelsFor(req.GetDevice().GetDeviceChannel()) {
		_ = h.Mgr.SetWaveform(id, ch, p.Waveform)
	}
	return okResp(pb.DGEvent_SETWAVE)
}

func (h *Hub) handleCustomWave(req *pb.DGRequest, sessionToken string) *pb.DGResponse {
	id := device.DeviceID(req.GetDevice().GetDeviceId())
	if !h.Mgr.Has(id) {
		return cantDoThis(pb.DGError_DEVICEOFFLINE, string(id))
	}
	if !h.canControl(id, sessionToken) {
		return cantDoThis(pb.DGError_DEVICENOTLOCKBYYOU, string(id))
	}
	frames := make([][3]byte, 0, len(req.GetCustomWave()))
	for _, cw := range req.GetCustomWave() {
		b := cw.GetBytes()
		if len(b) != 3 {
			continue
		}
		frames = append(frames, [3]byte{b[0], b[1], b[2]})
	}
	w := waveform.V2FramesToWaveform("custom", frames)
	for _, ch := range channelsFor(req.GetDevice().GetDeviceChannel()) {
		_ = h.Mgr.SetWaveform(id, ch, w)
	}
	return okResp(pb.DGEvent_CUSTOMWAVE)
}

func (h *Hub) handleClearCustom(req *pb.DGRequest, sessionToken string) *pb.DGResponse {
	id := device.DeviceID(req.GetDevice().GetDeviceId())
	if !h.Mgr.Has(id) {
		return cantDoThis(pb.DGError_DEVICEOFFLINE, string(id))
	}
	if !h.canControl(id, sessionToken) {
		return cantDoThis(pb.DGError_DEVICENOTLOCKBYYOU, string(id))
	}
	for _, ch := range channelsFor(req.GetDevice().GetDeviceChannel()) {
		_ = h.Mgr.ClearWaveform(id, ch)
	}
	return okResp(pb.DGEvent_CLEARCUSTOM)
}

// --- helpers ---

func channelsFor(ch pb.DGDeviceChannel) []device.Channel {
	switch ch {
	case pb.DGDeviceChannel_CHANNEL_A:
		return []device.Channel{device.ChannelA}
	case pb.DGDeviceChannel_CHANNEL_B:
		return []device.Channel{device.ChannelB}
	default:
		return []device.Channel{device.ChannelA, device.ChannelB}
	}
}

func clampByte(v int32) uint8 {
	if v < 0 {
		return 0
	}
	if v > int32(device.MaxStrength) {
		return device.MaxStrength
	}
	return uint8(v)
}

func okResp(ev pb.DGEvent) *pb.DGResponse {
	return &pb.DGResponse{Version: 1, Event: ev}
}

func cantDoThis(e pb.DGError, deviceID string) *pb.DGResponse {
	r := &pb.DGResponse{Version: 1, Event: pb.DGEvent_CANTDOTHIS, Error: e}
	if deviceID != "" {
		r.DeviceId = &pb.DGResponse_DGDeviceID{DeviceId: deviceID}
	}
	return r
}

func lockedDeviceResp(id device.DeviceID, lockedByRemote, lockedByMe bool) *pb.DGResponse {
	return &pb.DGResponse{
		Version: 1,
		Event:   pb.DGEvent_LOCKDEVICE,
		Device: &pb.DGResponse_DGDevice{
			Id:               string(id),
			IsLockedByRemote: lockedByRemote,
			IsLockedByMe:     lockedByMe,
		},
	}
}

func newToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
