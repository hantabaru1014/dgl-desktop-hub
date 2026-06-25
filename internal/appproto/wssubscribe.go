package appproto

import (
	"net/http"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/encoding/protojson"

	pb "github.com/hantabaru1014/dgl-desktop-hub/internal/pb/com/github/opendglab"
)

// ServeWS は /ws/subscribe を提供する。grpc / grpc-Web を使えない環境向けに
// Subscribe RPC と等価な動作を WebSocket で行う。
//
// 利用方法:
//   - 最初の text frame に DGRequest (protojson) を載せて送信する。
//     CONNECT イベントなら新規トークンを発行、それ以外なら認証済みトークンが必要。
//   - 認証は X-DGLab-Token ヘッダ、または ?token= クエリ (tokenQueryMiddleware が補完)。
//   - 以降サーバから DGResponse の push を text frame で受け取る (クライアントは送信不要)。
func (s *Service) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		// hub は同一ホスト前提のローカルツール用なので Origin 制限はかけない。
		InsecureSkipVerify: true,
	})
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusInternalError, "")

	// 最初のフレーム = DGRequest JSON。
	ctx := r.Context()
	typ, data, err := conn.Read(ctx)
	if err != nil {
		return
	}
	if typ != websocket.MessageText {
		conn.Close(websocket.StatusUnsupportedData, "expected text frame")
		return
	}
	req := &pb.DGRequest{}
	if err := protojson.Unmarshal(data, req); err != nil {
		conn.Close(websocket.StatusUnsupportedData, "invalid DGRequest json")
		return
	}

	// 以降は受信不要だが、Close を検知できるよう CloseRead で
	// バックグラウンド Read + 切断時の ctx キャンセルを任せる。
	ctx = conn.CloseRead(ctx)

	token := r.Header.Get(tokenHeader)
	send := func(resp *pb.DGResponse) error {
		b, err := protojson.Marshal(resp)
		if err != nil {
			return err
		}
		return conn.Write(ctx, websocket.MessageText, b)
	}

	if err := s.runSubscription(ctx, req, token, send); err == nil {
		conn.Close(websocket.StatusNormalClosure, "")
	}
}
