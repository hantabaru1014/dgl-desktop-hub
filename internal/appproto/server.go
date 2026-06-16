// Package appproto はアプリ操作側 (OpenDGLab プロトコル) の受け口を提供する。
// connectrpc.com/connect による Connect/gRPC/gRPC-Web (JSON/binary) を受ける。
package appproto

import (
	"context"
	"net/http"
	"sync"
	"time"

	"connectrpc.com/connect"

	"github.com/hantabaru1014/dgl-desktop-hub/internal/hubcore"
	pb "github.com/hantabaru1014/dgl-desktop-hub/internal/pb/com/github/opendglab"
	"github.com/hantabaru1014/dgl-desktop-hub/internal/pb/com/github/opendglab/opendglabconnect"
)

// keepAliveTick はストリーム keep-alive の送信間隔。
// アイドル TTL (90 秒) より十分短く設定する。
const keepAliveTick = 30 * time.Second

// tokenHeader は認証トークンを載せる HTTP ヘッダ名。
// HTTP ヘッダを付与できないアプリ向けに ?token= クエリでも補完できる
// (httpserver.go の tokenQueryMiddleware を参照)。
const tokenHeader = "X-DGLab-Token"

// Service は OpenDGLabServiceHandler の実装。
type Service struct {
	opendglabconnect.UnimplementedOpenDGLabServiceHandler
	hub *hubcore.Hub

	// streams はトークン毎のアクティブストリーム参照カウント。
	// 同一トークンで複数ストリームが張られても最後の 1 本が切れるまで
	// DisconnectApp を呼ばないために使う。
	streamMu sync.Mutex
	streams  map[string]int
}

// NewService は Service を生成する。
func NewService(hub *hubcore.Hub) *Service {
	return &Service{
		hub:     hub,
		streams: make(map[string]int),
	}
}

// Send は単発リクエストを処理する (Connect 経由)。
func (s *Service) Send(ctx context.Context, req *connect.Request[pb.DGRequest]) (*connect.Response[pb.DGResponse], error) {
	token := req.Header().Get(tokenHeader)
	resp, err := s.hub.HandleSend(ctx, req.Msg, token)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(resp), nil
}

// Subscribe は push 通知を server-streaming で配信する。
//
// 認可フロー:
//   - 最初のリクエストが CONNECT の場合は HandleSend でトークンを発行する。
//     レスポンスの connect token が空 (拒否/タイムアウト) なら購読させずに終了。
//   - CONNECT 以外の場合はヘッダの X-DGLab-Token が登録済みか確認し、
//     未認証なら CANTDOTHIS+UNAUTHED を返して終了。
//
// 認可通過後は購読ループへ入り、30 秒ごとに TouchApp でアイドルカウンタをリセットする。
func (s *Service) Subscribe(ctx context.Context, req *connect.Request[pb.DGRequest], stream *connect.ServerStream[pb.DGResponse]) error {
	token := req.Header().Get(tokenHeader)

	if req.Msg.GetEvent() == pb.DGEvent_CONNECT {
		// CONNECT イベント: トークンを発行してレスポンスを返す。
		resp, _ := s.hub.HandleSend(ctx, req.Msg, token)
		if c := resp.GetConnect(); c != nil {
			token = c.GetToken()
		}
		if err := stream.Send(resp); err != nil {
			return err
		}
		// トークンが空 = 接続拒否/タイムアウト → 購読させない。
		if token == "" {
			return nil
		}
	} else {
		// CONNECT 以外: 認証済みトークンが必要。
		if !s.hub.IsKnownApp(token) {
			if err := stream.Send(&pb.DGResponse{
				Version: 1,
				Event:   pb.DGEvent_CANTDOTHIS,
				Error:   pb.DGError_UNAUTHED,
			}); err != nil {
				return err
			}
			return nil
		}
	}

	// --- 認可通過 ---

	// 参照カウントをインクリメントし、ストリーム終了時に後始末する。
	s.streamMu.Lock()
	s.streams[token]++
	s.streamMu.Unlock()
	defer func() {
		s.streamMu.Lock()
		s.streams[token]--
		if s.streams[token] <= 0 {
			delete(s.streams, token)
			s.streamMu.Unlock()
			// 最後のストリームが切れたのでアプリを切断扱いにする。
			s.hub.DisconnectApp(token)
		} else {
			s.streamMu.Unlock()
		}
	}()

	ch, cancel := s.hub.Subscribe()
	defer cancel()

	// keep-alive ティッカー: アプリが push を受信するだけで Send を送らなくても
	// アイドル TTL で除去されないよう TouchApp を定期呼び出しする。
	ticker := time.NewTicker(keepAliveTick)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			s.hub.TouchApp(token)
		case msg, ok := <-ch:
			if !ok {
				return nil
			}
			if err := stream.Send(msg); err != nil {
				return err
			}
		}
	}
}

// Handler は Connect ハンドラを返す (パスとハンドラ)。
func (s *Service) Handler(opts ...connect.HandlerOption) (string, http.Handler) {
	return opendglabconnect.NewOpenDGLabServiceHandler(s, opts...)
}
