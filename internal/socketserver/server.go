// Package socketserver は DG-Lab socket mode の WebSocket サーバを実装する。
// LAN 内のスマホ (DG-Lab アプリ) が QR をスキャンして接続し、ハブが
// コントローラとして Coyote を操作する。各接続スマホ = 1 つの SocketCoyote。
package socketserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/hantabaru1014/dgl-desktop-hub/internal/device"
)

// Callbacks は接続/切断時にハブへ通知するためのコールバック。
type Callbacks struct {
	OnConnect    func(device.CoyoteDevice)
	OnDisconnect func(device.DeviceID)
}

// Server は単一の WebSocket サーバ。
type Server struct {
	cb Callbacks

	mu           sync.Mutex
	controllerID string
	host         string
	port         int
	httpSrv      *http.Server
	conns        map[string]*wsConn // phoneID -> conn
	running      bool
}

// NewServer は Server を生成する。
func NewServer(cb Callbacks) *Server {
	return &Server{cb: cb, conns: make(map[string]*wsConn)}
}

// Start はサーバを起動する (非ブロッキング)。host が空なら LAN IP を自動検出。
func (s *Server) Start(host string, port int) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil
	}
	if host == "" {
		host = LANIP()
	}
	ln, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		s.mu.Unlock()
		return err
	}
	s.host = host
	s.port = ln.Addr().(*net.TCPAddr).Port
	s.controllerID = newID()
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleWS)
	srv := &http.Server{Handler: mux}
	s.httpSrv = srv
	s.running = true
	s.mu.Unlock()

	go func() { _ = srv.Serve(ln) }()
	return nil
}

// Stop はサーバを停止し、全接続を閉じる。
func (s *Server) Stop() error {
	s.mu.Lock()
	srv := s.httpSrv
	s.running = false
	conns := make([]*wsConn, 0, len(s.conns))
	for _, c := range s.conns {
		conns = append(conns, c)
	}
	s.conns = make(map[string]*wsConn)
	s.mu.Unlock()

	for _, c := range conns {
		_ = c.ws.Close(websocket.StatusNormalClosure, "server stop")
	}
	if srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		return srv.Shutdown(ctx)
	}
	return nil
}

// Running はサーバ稼働状態を返す。
func (s *Server) Running() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// QR はスマホがスキャンする QR 文字列を返す。
func (s *Server) QR() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return ""
	}
	return QRString(s.host, s.port, s.controllerID)
}

// ControllerID は QR に埋め込むコントローラ ID を返す。
func (s *Server) ControllerID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.controllerID
}

// Port は待ち受けポートを返す。
func (s *Server) Port() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.port
}

// Addr は host:port を返す。
func (s *Server) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return ""
	}
	return fmt.Sprintf("%s:%d", s.host, s.port)
}

type wsConn struct {
	ws      *websocket.Conn
	phoneID string
	sendMu  sync.Mutex
	coy     *SocketCoyote
}

func (c *wsConn) sendEnvelope(e Envelope) error {
	data, err := e.encode()
	if err != nil {
		return err
	}
	c.sendMu.Lock()
	defer c.sendMu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return c.ws.Write(ctx, websocket.MessageText, data)
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}
	phoneID := newID()
	c := &wsConn{ws: ws, phoneID: phoneID}

	s.mu.Lock()
	controllerID := s.controllerID
	s.conns[phoneID] = c
	s.mu.Unlock()

	// 新規接続へ自身の ID を通知。
	_ = c.sendEnvelope(Envelope{Type: "bind", ClientID: phoneID, TargetID: "", Message: "targetId"})

	ctx := r.Context()
	go s.heartbeat(ctx, c, controllerID)
	s.readLoop(ctx, c, controllerID)

	// 後始末。
	s.mu.Lock()
	delete(s.conns, phoneID)
	coy := c.coy
	s.mu.Unlock()
	_ = ws.Close(websocket.StatusNormalClosure, "bye")
	if coy != nil && s.cb.OnDisconnect != nil {
		s.cb.OnDisconnect(coy.ID())
	}
}

func (s *Server) readLoop(ctx context.Context, c *wsConn, controllerID string) {
	for {
		_, data, err := c.ws.Read(ctx)
		if err != nil {
			return
		}
		if len(data) > 1<<16 {
			continue // プロトコル上限を大きく超えるメッセージは無視。
		}
		var e Envelope
		if err := json.Unmarshal(data, &e); err != nil {
			continue
		}
		switch e.Type {
		case "bind":
			s.handleBind(c, controllerID, e)
		case "msg":
			if fb, ok := ParseStrengthFeedback(e.Message); ok && c.coy != nil {
				c.coy.onFeedback(fb)
			}
		case "heartbeat":
			// 何もしない。
		}
	}
}

func (s *Server) handleBind(c *wsConn, controllerID string, e Envelope) {
	// アプリは {clientId: controllerID, targetId: 自身ID} で bind 要求を送る。
	if e.ClientID != controllerID {
		_ = c.sendEnvelope(Envelope{Type: "error", ClientID: controllerID, TargetID: c.phoneID, Message: "401"})
		return
	}
	if c.coy != nil {
		return // 既に bind 済み
	}
	coy := newSocketCoyote(controllerID, c.phoneID, c.sendEnvelope, func() error {
		return c.ws.Close(websocket.StatusNormalClosure, "removed")
	})
	c.coy = coy

	// 両者へ成功を通知。
	_ = c.sendEnvelope(Envelope{Type: "bind", ClientID: controllerID, TargetID: c.phoneID, Message: "200"})
	if s.cb.OnConnect != nil {
		s.cb.OnConnect(coy)
	}
}

func (s *Server) heartbeat(ctx context.Context, c *wsConn, controllerID string) {
	t := time.NewTicker(60 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := c.sendEnvelope(Envelope{Type: "heartbeat", ClientID: controllerID, TargetID: c.phoneID, Message: "200"}); err != nil {
				return
			}
		}
	}
}

func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// LANIP は LAN 内 IPv4 アドレスを推定して返す。失敗時は 127.0.0.1。
func LANIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err == nil {
		defer conn.Close()
		if addr, ok := conn.LocalAddr().(*net.UDPAddr); ok {
			return addr.IP.String()
		}
	}
	// フォールバック: 最初の非ループバック IPv4。
	addrs, _ := net.InterfaceAddrs()
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ip4 := ipnet.IP.To4(); ip4 != nil {
				return ip4.String()
			}
		}
	}
	return "127.0.0.1"
}
