package device

import (
	"fmt"
	"sync"
	"time"

	"github.com/hantabaru1014/dgl-desktop-hub/internal/waveform"
)

// TickInterval は出力ループの周期。Coyote V3 の B0 は 100ms ごとに送る。
const TickInterval = 100 * time.Millisecond

// ChannelOutput はグラフ描画用の 1 チャンネル分の出力値。
type ChannelOutput struct {
	Strength uint8 `json:"strength"` // 現在強度 (ソフトリミット適用済) 0..limit = 白線
	Amp      uint8 `json:"amp"`      // 波形平均強度 0..100
	Output   uint8 `json:"output"`   // 実効出力 = clamp済強度*amp/100 = 棒
}

// GraphSample は 1 デバイス・1 周期分のグラフサンプル。
type GraphSample struct {
	DeviceID DeviceID      `json:"deviceId"`
	A        ChannelOutput `json:"a"`
	B        ChannelOutput `json:"b"`
}

// Hooks は DeviceManager が外部 (Wails イベント等) へ通知するためのコールバック群。
// いずれも nil 可。
type Hooks struct {
	OnGraph          func([]GraphSample) // 毎周期、全デバイスのサンプルをまとめて
	OnDevicesChanged func([]Info)        // デバイス追加/削除/状態変化時
}

// managed は DeviceManager 内部のデバイスラッパ。
type managed struct {
	mu    sync.Mutex
	dev   CoyoteDevice
	limit SoftLimit
	a, b  channelState
}

// DeviceManager は全デバイスを登録し、100ms 周期で出力を駆動する。
type DeviceManager struct {
	mu      sync.RWMutex
	devices map[DeviceID]*managed
	order   []DeviceID
	hooks   Hooks

	loopMu  sync.Mutex
	ticker  *time.Ticker
	quit    chan struct{}
	running bool
}

// NewDeviceManager は DeviceManager を生成する。
func NewDeviceManager(hooks Hooks) *DeviceManager {
	return &DeviceManager{
		devices: make(map[DeviceID]*managed),
		hooks:   hooks,
	}
}

// Add はデバイスを登録する。強度報告コールバックを配線し、初期ソフトリミットを
// 適用する。登録済みの ID なら既存を返す。
func (m *DeviceManager) Add(dev CoyoteDevice) DeviceID {
	id := dev.ID()
	m.mu.Lock()
	if _, ok := m.devices[id]; ok {
		m.mu.Unlock()
		return id
	}
	md := &managed{
		dev:   dev,
		limit: DefaultSoftLimit(),
	}
	m.devices[id] = md
	m.order = append(m.order, id)
	m.mu.Unlock()

	dev.OnStrengthReport(func(a, b uint8) {
		md.mu.Lock()
		md.a.reported = a
		md.b.reported = b
		md.mu.Unlock()
	})
	if sla, ok := dev.(SoftLimitAware); ok {
		_ = sla.ApplySoftLimit(md.limit.A, md.limit.B)
	}
	m.notifyDevices()
	return id
}

// Remove はデバイスを登録解除し Close する。
func (m *DeviceManager) Remove(id DeviceID) error {
	m.mu.Lock()
	md, ok := m.devices[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("device: not found: %s", id)
	}
	delete(m.devices, id)
	for i, v := range m.order {
		if v == id {
			m.order = append(m.order[:i], m.order[i+1:]...)
			break
		}
	}
	m.mu.Unlock()

	err := md.dev.Close()
	m.notifyDevices()
	return err
}

// List は登録順のデバイス情報を返す。
func (m *DeviceManager) List() []Info {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Info, 0, len(m.order))
	for _, id := range m.order {
		if md, ok := m.devices[id]; ok {
			out = append(out, md.dev.Info())
		}
	}
	return out
}

// Has は ID の存在を返す。
func (m *DeviceManager) Has(id DeviceID) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.devices[id]
	return ok
}

func (m *DeviceManager) get(id DeviceID) (*managed, error) {
	m.mu.RLock()
	md, ok := m.devices[id]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("device: not found: %s", id)
	}
	return md, nil
}

// SetSoftLimit はソフトリミットを設定する。SoftLimitAware なデバイスには
// デバイス側にも反映する。
func (m *DeviceManager) SetSoftLimit(id DeviceID, lim SoftLimit) error {
	md, err := m.get(id)
	if err != nil {
		return err
	}
	md.mu.Lock()
	md.limit = SoftLimit{A: ClampStrength(int(lim.A)), B: ClampStrength(int(lim.B))}
	// リミットを下げた場合、現在の強度も新リミットまで切り下げる。
	md.a.clampToLimit(md.limit.A)
	md.b.clampToLimit(md.limit.B)
	applied := md.limit
	md.mu.Unlock()
	if sla, ok := md.dev.(SoftLimitAware); ok {
		_ = sla.ApplySoftLimit(applied.A, applied.B)
	}
	m.notifyDevices()
	return nil
}

// SoftLimitOf は現在のソフトリミットを返す。
func (m *DeviceManager) SoftLimitOf(id DeviceID) (SoftLimit, error) {
	md, err := m.get(id)
	if err != nil {
		return SoftLimit{}, err
	}
	md.mu.Lock()
	defer md.mu.Unlock()
	return md.limit, nil
}

// SetStrength は指定チャンネルの強度を mode に従い更新する。
func (m *DeviceManager) SetStrength(id DeviceID, ch Channel, mode StrengthMode, val uint8) error {
	md, err := m.get(id)
	if err != nil {
		return err
	}
	md.mu.Lock()
	if ch == ChannelA {
		md.a.applyStrength(mode, val, md.limit.A)
	} else {
		md.b.applyStrength(mode, val, md.limit.B)
	}
	md.mu.Unlock()
	m.notifyDevices()
	return nil
}

// SetWaveform は指定チャンネルの波形を差し替える。
func (m *DeviceManager) SetWaveform(id DeviceID, ch Channel, w waveform.Waveform) error {
	md, err := m.get(id)
	if err != nil {
		return err
	}
	md.mu.Lock()
	if ch == ChannelA {
		md.a.setWaveform(w)
	} else {
		md.b.setWaveform(w)
	}
	md.mu.Unlock()
	m.notifyDevices()
	return nil
}

// WaveNames は各チャンネルの現在の波形名を返す。
func (m *DeviceManager) WaveNames(id DeviceID) (a, b string, err error) {
	md, e := m.get(id)
	if e != nil {
		return "", "", e
	}
	md.mu.Lock()
	defer md.mu.Unlock()
	return md.a.wave.Name, md.b.wave.Name, nil
}

// ClearWaveform は指定チャンネルの波形を空 (無出力) にする。
func (m *DeviceManager) ClearWaveform(id DeviceID, ch Channel) error {
	return m.SetWaveform(id, ch, waveform.Waveform{})
}

// Snapshot は指定デバイスの現在の目標/報告強度を返す (テスト・introspection 用)。
func (m *DeviceManager) Snapshot(id DeviceID) (targetA, targetB, reportedA, reportedB uint8, lim SoftLimit, err error) {
	md, e := m.get(id)
	if e != nil {
		return 0, 0, 0, 0, SoftLimit{}, e
	}
	md.mu.Lock()
	defer md.mu.Unlock()
	return md.a.target, md.b.target, md.a.reported, md.b.reported, md.limit, nil
}

// Start は出力ループを開始する。既に動作中なら何もしない。
func (m *DeviceManager) Start() {
	m.loopMu.Lock()
	defer m.loopMu.Unlock()
	if m.running {
		return
	}
	m.running = true
	m.ticker = time.NewTicker(TickInterval)
	m.quit = make(chan struct{})
	go m.loop(m.ticker, m.quit)
}

// Stop は出力ループを停止する。
func (m *DeviceManager) Stop() {
	m.loopMu.Lock()
	defer m.loopMu.Unlock()
	if !m.running {
		return
	}
	m.running = false
	m.ticker.Stop()
	close(m.quit)
}

func (m *DeviceManager) loop(ticker *time.Ticker, quit chan struct{}) {
	for {
		select {
		case <-quit:
			return
		case <-ticker.C:
			m.tickAll()
		}
	}
}

func (m *DeviceManager) snapshotManaged() []*managed {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*managed, 0, len(m.order))
	for _, id := range m.order {
		if md, ok := m.devices[id]; ok {
			out = append(out, md)
		}
	}
	return out
}

func (m *DeviceManager) tickAll() {
	mds := m.snapshotManaged()
	samples := make([]GraphSample, 0, len(mds))
	for _, md := range mds {
		md.mu.Lock()
		ta, tb := md.a.target, md.b.target // 現在強度 (ソフトリミット適用済)
		// target は設定時点で適用済だが、念のため出力直前にも頭打ちする。
		sa := clampStrength(ta, md.limit.A)
		sb := clampStrength(tb, md.limit.B)
		qa := md.a.nextQuad()
		qb := md.b.nextQuad()
		id := md.dev.ID()
		md.mu.Unlock()

		_ = md.dev.Output(OutputCommand{
			StrengthA: sa,
			StrengthB: sb,
			QuadA:     qa,
			QuadB:     qb,
		})
		samples = append(samples, GraphSample{
			DeviceID: id,
			A:        channelOutput(ta, sa, qa.MeanIntensity()),
			B:        channelOutput(tb, sb, qb.MeanIntensity()),
		})
	}
	if m.hooks.OnGraph != nil {
		m.hooks.OnGraph(samples)
	}
}

// channelOutput はグラフ用サンプルを作る。
// Strength=現在強度(ソフトリミット適用済, 白線)、Output=実効出力(強度 * 振幅, 棒)。
func channelOutput(target, clamped, amp uint8) ChannelOutput {
	return ChannelOutput{
		Strength: target,
		Amp:      amp,
		Output:   uint8(int(clamped) * int(amp) / 100),
	}
}

func (m *DeviceManager) notifyDevices() {
	if m.hooks.OnDevicesChanged != nil {
		m.hooks.OnDevicesChanged(m.List())
	}
}
