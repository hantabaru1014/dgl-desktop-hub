// Package hubcore はアプリ操作側 (OpenDGLab プロトコル) と出力デバイス側
// (DeviceManager) を仲介する中核。接続許可/lock モデル、push 通知、
// プリセット解決を担う。
package hubcore

import (
	"sync"
	"time"

	"github.com/hantabaru1014/dgl-desktop-hub/internal/device"
	pb "github.com/hantabaru1014/dgl-desktop-hub/internal/pb/com/github/opendglab"
)

// アプリのアイドル切断パラメータ。
const (
	appIdleTTL      = 90 * time.Second // 最終通信からこの時間で切断扱い
	appSweepEvery   = 30 * time.Second // sweeper の周期
	approvalTimeout = 60 * time.Second // 接続許可モーダルの待機上限
)

// EventSink は Hub から UI 層 (Wails) へ通知するためのシンク。いずれも nil 可。
type EventSink struct {
	OnGraph           func([]device.GraphSample)
	OnDevicesChanged  func([]device.Info)
	OnAppsChanged     func([]AppInfo)
	OnApprovalRequest func(ApprovalRequest)
}

// ApprovalRequest は接続許可をユーザーに尋ねる要求。
type ApprovalRequest struct {
	RequestID string `json:"requestId"`
	AppName   string `json:"appName"`
	UUID      string `json:"uuid"`
}

// AppInfo は接続中アプリの情報。
type AppInfo struct {
	Token   string `json:"token"`
	AppName string `json:"appName"`
	UUID    string `json:"uuid"`
}

// appEntry はアプリ情報 + 最終通信時刻。
type appEntry struct {
	info     AppInfo
	lastSeen time.Time
}

// Hub は中核オブジェクト。
type Hub struct {
	Mgr *device.DeviceManager

	mu          sync.RWMutex
	autoApprove bool
	exclusive   map[device.DeviceID]bool   // device -> 排他モード (true で単一アプリのみ)
	owners      map[device.DeviceID]string // device -> ownerToken (排他モード時)
	apps        map[string]*appEntry       // token -> app

	sinkMu sync.RWMutex
	sink   EventSink

	subMu  sync.Mutex
	subs   map[int]chan *pb.DGResponse
	subSeq int

	approvalMu sync.Mutex
	approvals  map[string]chan bool

	sweepQuit chan struct{}
}

// NewHub は Hub を生成する。デフォルトは自動許可。排他モードはデバイス毎 (既定 false)。
func NewHub() *Hub {
	h := &Hub{
		autoApprove: true,
		exclusive:   make(map[device.DeviceID]bool),
		owners:      make(map[device.DeviceID]string),
		apps:        make(map[string]*appEntry),
		subs:        make(map[int]chan *pb.DGResponse),
		approvals:   make(map[string]chan bool),
		sweepQuit:   make(chan struct{}),
	}
	h.Mgr = device.NewDeviceManager(device.Hooks{
		OnGraph:          h.handleGraph,
		OnDevicesChanged: h.handleDevicesChanged,
	})
	go h.sweepLoop()
	return h
}

// SetEventSink は UI 層のシンクを差し替える。
func (h *Hub) SetEventSink(s EventSink) {
	h.sinkMu.Lock()
	h.sink = s
	h.sinkMu.Unlock()
}

func (h *Hub) currentSink() EventSink {
	h.sinkMu.RLock()
	defer h.sinkMu.RUnlock()
	return h.sink
}

// Start/Stop は出力ループの開始/停止。
func (h *Hub) Start() { h.Mgr.Start() }
func (h *Hub) Stop() {
	h.Mgr.Stop()
	select {
	case <-h.sweepQuit:
	default:
		close(h.sweepQuit)
	}
}

func (h *Hub) handleGraph(s []device.GraphSample) {
	if sink := h.currentSink(); sink.OnGraph != nil {
		sink.OnGraph(s)
	}
}

func (h *Hub) handleDevicesChanged(infos []device.Info) {
	if sink := h.currentSink(); sink.OnDevicesChanged != nil {
		sink.OnDevicesChanged(infos)
	}
	// 接続中アプリへ DEVICERESET (= デバイス一覧更新の合図) を push。
	h.broadcast(&pb.DGResponse{Version: 1, Event: pb.DGEvent_DEVICERESET})
}

// --- アクセスモード ---

// AutoApprove は接続自動許可設定を返す。
func (h *Hub) AutoApprove() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.autoApprove
}

// SetAutoApprove は接続自動許可設定を更新する。
func (h *Hub) SetAutoApprove(v bool) {
	h.mu.Lock()
	h.autoApprove = v
	h.mu.Unlock()
}

// IsExclusive はデバイスが排他モードかを返す。
func (h *Hub) IsExclusive(id device.DeviceID) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.exclusive[id]
}

// OwnerName は排他モードのデバイスを専有しているアプリ名を返す。
// 非排他/未専有なら空文字。
func (h *Hub) OwnerName(id device.DeviceID) string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if !h.exclusive[id] {
		return ""
	}
	token := h.owners[id]
	if token == "" {
		return ""
	}
	if e, ok := h.apps[token]; ok {
		if e.info.AppName != "" {
			return e.info.AppName
		}
		return "(名称なし)"
	}
	return "(不明なアプリ)"
}

// notifyDevicesChanged はデバイス一覧の更新を UI へ通知する (owner 変化時など)。
func (h *Hub) notifyDevicesChanged() {
	if sink := h.currentSink(); sink.OnDevicesChanged != nil {
		sink.OnDevicesChanged(h.Mgr.List())
	}
}

// SetExclusive はデバイスの排他モードを設定する。off にすると owner も解放。
func (h *Hub) SetExclusive(id device.DeviceID, v bool) {
	h.mu.Lock()
	h.exclusive[id] = v
	if !v {
		delete(h.owners, id)
	}
	h.mu.Unlock()
	h.notifyDevicesChanged()
}

// --- 接続許可 (autoApprove=false 時) ---

// requestApproval は許可要求を発行し、ユーザーの応答 (またはタイムアウト) を待つ。
func (h *Hub) requestApproval(appName, uuid string) bool {
	id := newToken()
	ch := make(chan bool, 1)
	h.approvalMu.Lock()
	h.approvals[id] = ch
	h.approvalMu.Unlock()

	if sink := h.currentSink(); sink.OnApprovalRequest != nil {
		sink.OnApprovalRequest(ApprovalRequest{RequestID: id, AppName: appName, UUID: uuid})
	}

	defer func() {
		h.approvalMu.Lock()
		delete(h.approvals, id)
		h.approvalMu.Unlock()
	}()
	select {
	case ok := <-ch:
		return ok
	case <-time.After(approvalTimeout):
		return false
	}
}

// ResolveApproval はユーザーの承認/拒否を反映する。
func (h *Hub) ResolveApproval(requestID string, approve bool) {
	h.approvalMu.Lock()
	ch, ok := h.approvals[requestID]
	h.approvalMu.Unlock()
	if ok {
		select {
		case ch <- approve:
		default:
		}
	}
}

// Apps は接続中アプリ一覧を返す。
func (h *Hub) Apps() []AppInfo {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]AppInfo, 0, len(h.apps))
	for _, e := range h.apps {
		out = append(out, e.info)
	}
	return out
}

func (h *Hub) registerApp(a AppInfo) {
	h.mu.Lock()
	h.apps[a.Token] = &appEntry{info: a, lastSeen: time.Now()}
	h.mu.Unlock()
	h.notifyApps()
}

// DisconnectApp はアプリを明示的に切断扱いで除去する (ストリーム終了時など)。
func (h *Hub) DisconnectApp(token string) { h.removeApp(token) }

// isKnownApp はトークンが登録済みアプリのものかを返す (再接続の承認スキップ用)。
func (h *Hub) isKnownApp(token string) bool {
	if token == "" {
		return false
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.apps[token]
	return ok
}

// touchApp はトークンに対応するアプリの最終通信時刻を更新する。
func (h *Hub) touchApp(token string) {
	if token == "" {
		return
	}
	h.mu.Lock()
	if e, ok := h.apps[token]; ok {
		e.lastSeen = time.Now()
	}
	h.mu.Unlock()
}

// releaseOwnedBy は token がロックしていたデバイスを全て解放する。
// 呼び出し側で h.mu を保持していること。
func (h *Hub) releaseOwnedBy(token string) {
	for dev, owner := range h.owners {
		if owner == token {
			delete(h.owners, dev)
		}
	}
}

// removeApp はアプリを切断扱いで除去する。
func (h *Hub) removeApp(token string) {
	if token == "" {
		return
	}
	h.mu.Lock()
	_, ok := h.apps[token]
	delete(h.apps, token)
	h.releaseOwnedBy(token)
	h.mu.Unlock()
	if ok {
		h.notifyApps()
		h.notifyDevicesChanged() // 専有解放を反映
	}
}

func (h *Hub) notifyApps() {
	if sink := h.currentSink(); sink.OnAppsChanged != nil {
		sink.OnAppsChanged(h.Apps())
	}
}

// sweepLoop は一定時間通信のないアプリを切断扱いで除去する。
func (h *Hub) sweepLoop() {
	t := time.NewTicker(appSweepEvery)
	defer t.Stop()
	for {
		select {
		case <-h.sweepQuit:
			return
		case <-t.C:
			h.sweepIdleApps()
		}
	}
}

func (h *Hub) sweepIdleApps() {
	now := time.Now()
	var removed bool
	h.mu.Lock()
	for token, e := range h.apps {
		if now.Sub(e.lastSeen) > appIdleTTL {
			delete(h.apps, token)
			h.releaseOwnedBy(token)
			removed = true
		}
	}
	h.mu.Unlock()
	if removed {
		h.notifyApps()
		h.notifyDevicesChanged()
	}
}

// --- push 購読 ---

// Subscribe は push 通知用チャネルを登録し、解除関数を返す。
func (h *Hub) Subscribe() (<-chan *pb.DGResponse, func()) {
	ch := make(chan *pb.DGResponse, 16)
	h.subMu.Lock()
	id := h.subSeq
	h.subSeq++
	h.subs[id] = ch
	h.subMu.Unlock()
	return ch, func() {
		h.subMu.Lock()
		if c, ok := h.subs[id]; ok {
			delete(h.subs, id)
			close(c)
		}
		h.subMu.Unlock()
	}
}

func (h *Hub) broadcast(resp *pb.DGResponse) {
	h.subMu.Lock()
	defer h.subMu.Unlock()
	for _, ch := range h.subs {
		select {
		case ch <- resp:
		default:
			// 受信側が詰まっていれば取りこぼす (push はベストエフォート)。
		}
	}
}
