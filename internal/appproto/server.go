// Package appproto はアプリ操作側 (OpenDGLab プロトコル) の受け口を提供する。
// connectrpc.com/connect による Connect/gRPC/gRPC-Web (JSON/binary) を受ける。
package appproto

import (
	"context"
	"net/http"

	"connectrpc.com/connect"

	"github.com/hantabaru1014/dgl-desktop-hub/internal/hubcore"
	pb "github.com/hantabaru1014/dgl-desktop-hub/internal/pb/com/github/opendglab"
	"github.com/hantabaru1014/dgl-desktop-hub/internal/pb/com/github/opendglab/opendglabconnect"
)

// Service は OpenDGLabServiceHandler の実装。
type Service struct {
	opendglabconnect.UnimplementedOpenDGLabServiceHandler
	hub *hubcore.Hub
}

// NewService は Service を生成する。
func NewService(hub *hubcore.Hub) *Service {
	return &Service{hub: hub}
}

// Send は単発リクエストを処理する (Connect 経由)。
func (s *Service) Send(ctx context.Context, req *connect.Request[pb.DGRequest]) (*connect.Response[pb.DGResponse], error) {
	token := req.Header().Get("X-DGLab-Token")
	resp, err := s.hub.HandleSend(ctx, req.Msg, token)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(resp), nil
}

// Subscribe は push 通知を server-streaming で配信する。
func (s *Service) Subscribe(ctx context.Context, req *connect.Request[pb.DGRequest], stream *connect.ServerStream[pb.DGResponse]) error {
	// 最初のリクエストが CONNECT ならトークンを発行して接続確立を返す。
	token := req.Header().Get("X-DGLab-Token")
	if req.Msg.GetEvent() == pb.DGEvent_CONNECT {
		resp, _ := s.hub.HandleSend(ctx, req.Msg, token)
		if c := resp.GetConnect(); c != nil {
			token = c.GetToken()
		}
		if err := stream.Send(resp); err != nil {
			return err
		}
	}
	// ストリーム終了時 (アプリ切断) はアプリを除去。
	if token != "" {
		defer s.hub.DisconnectApp(token)
	}
	ch, cancel := s.hub.Subscribe()
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			return nil
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
