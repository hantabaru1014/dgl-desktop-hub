package services

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// デフォルトポート。
const (
	defaultAppPort    = 7330
	defaultSocketPort = 9999
)

// config は永続化される設定。
type config struct {
	AppPort    int               `json:"appPort"`
	SocketPort int               `json:"socketPort"`
	SoftLimits map[string][2]int `json:"softLimits"`
	Exclusive  map[string]bool   `json:"exclusive"`
	SocketHost string            `json:"socketHost"`
}

// settingsStore は設定を JSON ファイルに永続化する。
type settingsStore struct {
	mu   sync.Mutex
	path string
	cfg  config
}

func newSettingsStore() *settingsStore {
	s := &settingsStore{cfg: config{
		AppPort:    defaultAppPort,
		SocketPort: defaultSocketPort,
		SoftLimits: map[string][2]int{},
		Exclusive:  map[string]bool{},
	}}
	if dir, err := os.UserConfigDir(); err == nil {
		s.path = filepath.Join(dir, "dgl-desktop-hub", "config.json")
		s.load()
	}
	return s
}

func (s *settingsStore) load() {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	var c config
	if json.Unmarshal(data, &c) != nil {
		return
	}
	if c.AppPort > 0 {
		s.cfg.AppPort = c.AppPort
	}
	if c.SocketPort > 0 {
		s.cfg.SocketPort = c.SocketPort
	}
	if c.SoftLimits != nil {
		s.cfg.SoftLimits = c.SoftLimits
	}
	if c.Exclusive != nil {
		s.cfg.Exclusive = c.Exclusive
	}
	s.cfg.SocketHost = c.SocketHost
}

func (s *settingsStore) save() {
	if s.path == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(s.path), 0o755)
	if data, err := json.MarshalIndent(s.cfg, "", "  "); err == nil {
		_ = os.WriteFile(s.path, data, 0o644)
	}
}

func (s *settingsStore) AppPort() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg.AppPort
}

func (s *settingsStore) SetAppPort(p int) {
	s.mu.Lock()
	s.cfg.AppPort = p
	s.mu.Unlock()
	s.save()
}

func (s *settingsStore) SocketPort() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg.SocketPort
}

func (s *settingsStore) SetSocketPort(p int) {
	s.mu.Lock()
	s.cfg.SocketPort = p
	s.mu.Unlock()
	s.save()
}

// GetSoftLimit は保存済みソフトリミットを返す。無ければ ok=false。
func (s *settingsStore) GetSoftLimit(id string) (a, b int, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.cfg.SoftLimits[id]
	return v[0], v[1], ok
}

// SetSoftLimit はソフトリミットを保存する。
func (s *settingsStore) SetSoftLimit(id string, a, b int) {
	s.mu.Lock()
	if s.cfg.SoftLimits == nil {
		s.cfg.SoftLimits = map[string][2]int{}
	}
	s.cfg.SoftLimits[id] = [2]int{a, b}
	s.mu.Unlock()
	s.save()
}

// GetExclusive は保存済み排他モードを返す。無ければ ok=false。
func (s *settingsStore) GetExclusive(id string) (v, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok = s.cfg.Exclusive[id]
	return v, ok
}

// SetExclusive は排他モードを保存する。
func (s *settingsStore) SetExclusive(id string, v bool) {
	s.mu.Lock()
	if s.cfg.Exclusive == nil {
		s.cfg.Exclusive = map[string]bool{}
	}
	s.cfg.Exclusive[id] = v
	s.mu.Unlock()
	s.save()
}

// SocketHost は保存済みの socket QR ホスト (空なら未設定)。
func (s *settingsStore) SocketHost() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg.SocketHost
}

// SetSocketHost は socket QR ホストを保存する。
func (s *settingsStore) SetSocketHost(host string) {
	s.mu.Lock()
	s.cfg.SocketHost = host
	s.mu.Unlock()
	s.save()
}
