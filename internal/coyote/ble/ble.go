package ble

import (
	"context"
	"fmt"
	"sync"
	"time"

	"tinygo.org/x/bluetooth"

	"github.com/hantabaru1014/dgl-desktop-hub/internal/device"
)

// BLE UUID 群 (基底 0000xxxx-0000-1000-8000-00805f9b34fb)。
var (
	serviceCtrlUUID = bluetooth.New16BitUUID(0x180C)
	charWriteUUID   = bluetooth.New16BitUUID(0x150A)
	charNotifyUUID  = bluetooth.New16BitUUID(0x150B)
	serviceBattUUID = bluetooth.New16BitUUID(0x180A)
	charBattUUID    = bluetooth.New16BitUUID(0x1500)
)

var (
	adapter     = bluetooth.DefaultAdapter
	enableOnce  sync.Once
	enableErr   error
	scanMu      sync.Mutex
	lastScan    = map[string]bluetooth.Address{} // addr string -> Address
)

func enableAdapter() error {
	enableOnce.Do(func() { enableErr = adapter.Enable() })
	return enableErr
}

// ScanResult はスキャンで見つかった Coyote デバイス。
type ScanResult struct {
	Address string `json:"address"`
	Name    string `json:"name"`
	RSSI    int    `json:"rssi"`
}

// Scan は timeout の間 BLE をスキャンし、Coyote v3 (名前 prefix 一致) を返す。
func Scan(timeout time.Duration) ([]ScanResult, error) {
	if err := enableAdapter(); err != nil {
		return nil, fmt.Errorf("ble: enable adapter: %w", err)
	}
	found := map[string]ScanResult{}
	var mu sync.Mutex

	errCh := make(chan error, 1)
	go func() {
		errCh <- adapter.Scan(func(_ *bluetooth.Adapter, r bluetooth.ScanResult) {
			name := r.LocalName()
			if !isCoyoteName(name) {
				return
			}
			addr := r.Address.String()
			mu.Lock()
			found[addr] = ScanResult{Address: addr, Name: name, RSSI: int(r.RSSI)}
			mu.Unlock()
			scanMu.Lock()
			lastScan[addr] = r.Address
			scanMu.Unlock()
		})
	}()

	time.Sleep(timeout)
	_ = adapter.StopScan()
	select {
	case err := <-errCh:
		if err != nil {
			return nil, fmt.Errorf("ble: scan: %w", err)
		}
	case <-time.After(time.Second):
	}

	mu.Lock()
	defer mu.Unlock()
	out := make([]ScanResult, 0, len(found))
	for _, v := range found {
		out = append(out, v)
	}
	return out, nil
}

func isCoyoteName(name string) bool {
	return len(name) >= len(NamePrefix) && name[:len(NamePrefix)] == NamePrefix
}

// BLECoyote は BLE 直接接続の Coyote v3 デバイス。device.CoyoteDevice を実装。
type BLECoyote struct {
	id      device.DeviceID
	name    string
	address bluetooth.Address

	mu        sync.Mutex
	status    device.Status
	battery   uint8
	dev       bluetooth.Device
	writeCh   bluetooth.DeviceCharacteristic
	connected bool

	// 出力差分用
	lastSentA, lastSentB uint8
	seq                  uint8

	limA, limB uint8

	report func(a, b uint8)

	cmdCh chan device.OutputCommand
	quit  chan struct{}
}

// NewBLECoyote はスキャン結果のアドレス文字列から BLE デバイスを構築する。
func NewBLECoyote(addr, name string) (*BLECoyote, error) {
	scanMu.Lock()
	a, ok := lastScan[addr]
	scanMu.Unlock()
	if !ok {
		return nil, fmt.Errorf("ble: address %q not found in last scan; scan first", addr)
	}
	if name == "" {
		name = "Coyote " + addr
	}
	return &BLECoyote{
		id:      device.DeviceID("ble-" + addr),
		name:    name,
		address: a,
		battery: device.BatteryUnknown,
		limA:    20,
		limB:    20,
		cmdCh:   make(chan device.OutputCommand, 1),
	}, nil
}

func (c *BLECoyote) ID() device.DeviceID { return c.id }

func (c *BLECoyote) Info() device.Info {
	c.mu.Lock()
	defer c.mu.Unlock()
	return device.Info{
		ID:      c.id,
		Kind:    device.KindBLE,
		Name:    c.name,
		Status:  c.status,
		Battery: c.battery,
	}
}

// Connect はデバイスに接続し、特性を取得して notify・出力ループを開始する。
func (c *BLECoyote) Connect(ctx context.Context) error {
	c.mu.Lock()
	if c.connected {
		c.mu.Unlock()
		return nil
	}
	c.status = device.StatusConnecting
	c.mu.Unlock()

	if err := enableAdapter(); err != nil {
		return fmt.Errorf("ble: enable: %w", err)
	}
	dev, err := adapter.Connect(c.address, bluetooth.ConnectionParams{})
	if err != nil {
		c.setStatus(device.StatusDisconnected)
		return fmt.Errorf("ble: connect: %w", err)
	}

	// 制御サービスの特性を取得。
	svcs, err := dev.DiscoverServices([]bluetooth.UUID{serviceCtrlUUID})
	if err != nil || len(svcs) == 0 {
		_ = dev.Disconnect()
		c.setStatus(device.StatusDisconnected)
		return fmt.Errorf("ble: discover ctrl service: %w", err)
	}
	chars, err := svcs[0].DiscoverCharacteristics([]bluetooth.UUID{charWriteUUID, charNotifyUUID})
	if err != nil {
		_ = dev.Disconnect()
		c.setStatus(device.StatusDisconnected)
		return fmt.Errorf("ble: discover ctrl chars: %w", err)
	}
	var writeCh, notifyCh bluetooth.DeviceCharacteristic
	var haveWrite, haveNotify bool
	for _, ch := range chars {
		switch ch.UUID() {
		case charWriteUUID:
			writeCh, haveWrite = ch, true
		case charNotifyUUID:
			notifyCh, haveNotify = ch, true
		}
	}
	if !haveWrite || !haveNotify {
		_ = dev.Disconnect()
		c.setStatus(device.StatusDisconnected)
		return fmt.Errorf("ble: missing write/notify characteristic")
	}

	// notify 登録 (B1 強度フィードバック)。
	_ = notifyCh.EnableNotifications(func(buf []byte) {
		if msg, ok := DecodeB1(buf); ok {
			c.mu.Lock()
			report := c.report
			c.mu.Unlock()
			if report != nil {
				report(msg.StrengthA, msg.StrengthB)
			}
		}
	})

	// バッテリ特性 (任意)。
	if bsvcs, err := dev.DiscoverServices([]bluetooth.UUID{serviceBattUUID}); err == nil && len(bsvcs) > 0 {
		if bchars, err := bsvcs[0].DiscoverCharacteristics([]bluetooth.UUID{charBattUUID}); err == nil && len(bchars) > 0 {
			c.readBattery(bchars[0])
			_ = bchars[0].EnableNotifications(func(buf []byte) {
				if len(buf) >= 1 {
					c.setBattery(buf[0])
				}
			})
		}
	}

	c.mu.Lock()
	c.dev = dev
	c.writeCh = writeCh
	c.connected = true
	c.status = device.StatusConnected
	c.quit = make(chan struct{})
	c.lastSentA, c.lastSentB = 0, 0
	c.mu.Unlock()

	// 再接続毎に BF (ソフト上限) を必ず再送。
	_ = c.ApplySoftLimit(c.limA, c.limB)

	go c.writeLoop()
	return nil
}

// writeLoop は cmdCh から最新の指令を受け取り B0 を書き込む。
func (c *BLECoyote) writeLoop() {
	for {
		select {
		case <-c.quit:
			return
		case cmd := <-c.cmdCh:
			c.writeB0(cmd)
		}
	}
}

func (c *BLECoyote) writeB0(cmd device.OutputCommand) {
	c.mu.Lock()
	if !c.connected {
		c.mu.Unlock()
		return
	}
	methodA, setA := c.diffStrength(cmd.StrengthA, &c.lastSentA)
	methodB, setB := c.diffStrength(cmd.StrengthB, &c.lastSentB)
	wc := c.writeCh
	c.mu.Unlock()

	b := EncodeB0(0, methodA, methodB, setA, setB, cmd.QuadA, cmd.QuadB)
	_, _ = wc.WriteWithoutResponse(b[:])
}

// diffStrength は目標強度と前回送信値を比較し、解読方式と設定値を決める。
func (c *BLECoyote) diffStrength(target uint8, last *uint8) (ParseMethod, uint8) {
	if target == *last {
		return ParseNoChange, 0
	}
	*last = target
	return ParseAbsolute, target
}

// Output は ticker からの指令を最新 1 件だけ保持 (drop-oldest)。
func (c *BLECoyote) Output(cmd device.OutputCommand) error {
	select {
	case c.cmdCh <- cmd:
	default:
		// 既に保留中の指令があれば捨てて最新で置き換える。
		select {
		case <-c.cmdCh:
		default:
		}
		select {
		case c.cmdCh <- cmd:
		default:
		}
	}
	return nil
}

func (c *BLECoyote) OnStrengthReport(fn func(a, b uint8)) {
	c.mu.Lock()
	c.report = fn
	c.mu.Unlock()
}

// ApplySoftLimit は BF 指令でデバイス側ソフト上限を設定する。
func (c *BLECoyote) ApplySoftLimit(a, b uint8) error {
	c.mu.Lock()
	c.limA, c.limB = a, b
	wc := c.writeCh
	connected := c.connected
	c.mu.Unlock()
	if !connected {
		return nil
	}
	bf := EncodeBF(a, b, 0, 0, 0, 0)
	_, err := wc.WriteWithoutResponse(bf[:])
	return err
}

func (c *BLECoyote) Disconnect() error {
	c.mu.Lock()
	if !c.connected {
		c.mu.Unlock()
		return nil
	}
	c.connected = false
	c.status = device.StatusDisconnected
	close(c.quit)
	dev := c.dev
	c.mu.Unlock()
	return dev.Disconnect()
}

func (c *BLECoyote) Close() error { return c.Disconnect() }

func (c *BLECoyote) setStatus(s device.Status) {
	c.mu.Lock()
	c.status = s
	c.mu.Unlock()
}

func (c *BLECoyote) setBattery(b uint8) {
	c.mu.Lock()
	c.battery = b
	c.mu.Unlock()
}

func (c *BLECoyote) readBattery(ch bluetooth.DeviceCharacteristic) {
	buf := make([]byte, 1)
	if n, err := ch.Read(buf); err == nil && n >= 1 {
		c.setBattery(buf[0])
	}
}
