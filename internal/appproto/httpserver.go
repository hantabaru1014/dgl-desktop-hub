package appproto

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/hantabaru1014/dgl-desktop-hub/internal/hubcore"
)

// Server はアプリ操作側 (OpenDGLab) を受ける HTTP サーバ。
// Connect/gRPC/gRPC-Web を h2c で http2 ストリーミング対応として提供する。
type Server struct {
	hub *hubcore.Hub

	mu      sync.Mutex
	httpSrv *http.Server
	port    int
	running bool
}

// NewServer は Server を生成する。
func NewServer(hub *hubcore.Hub) *Server {
	return &Server{hub: hub}
}

// Handler は mux を構築して返す (テストや埋め込み用)。
func (s *Server) Handler() http.Handler {
	svc := NewService(s.hub)
	mux := http.NewServeMux()
	path, h := svc.Handler()
	mux.Handle(path, h)
	// ?token= クエリをヘッダへ補完してから h2c で平文 http2 を許可。
	return h2c.NewHandler(tokenQueryMiddleware(mux), &http2.Server{})
}

// tokenQueryMiddleware は HTTP ヘッダを付与できないアプリ向けに、
// X-DGLab-Token ヘッダが未設定で ?token= クエリがある場合に
// クエリ値をヘッダへ補完する。ヘッダが既にある場合は何もしない。
func tokenQueryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(tokenHeader) == "" {
			if tok := r.URL.Query().Get("token"); tok != "" {
				r.Header.Set(tokenHeader, tok)
			}
		}
		next.ServeHTTP(w, r)
	})
}

// Start は port で待ち受けを開始する (非ブロッキング)。bind 失敗時はエラーを返す。
func (s *Server) Start(port int) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil
	}
	ln, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		s.mu.Unlock()
		return err
	}
	srv := &http.Server{Handler: s.Handler()}
	s.httpSrv = srv
	s.port = ln.Addr().(*net.TCPAddr).Port
	s.running = true
	s.mu.Unlock()

	go func() { _ = srv.Serve(ln) }()
	return nil
}

// Running はサーバ稼働状態を返す。
func (s *Server) Running() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// Port は待ち受けポートを返す。停止中は 0。
func (s *Server) Port() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return 0
	}
	return s.port
}

// Stop はサーバを停止する。
func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	srv := s.httpSrv
	s.running = false
	s.port = 0
	s.mu.Unlock()
	if srv == nil {
		return nil
	}
	return srv.Shutdown(ctx)
}
