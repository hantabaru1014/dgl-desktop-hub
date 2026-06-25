package appproto

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/encoding/protojson"

	pb "github.com/hantabaru1014/dgl-desktop-hub/internal/pb/com/github/opendglab"
)

// wsURL は httptest.NewServer の URL を ws:// に書き換えて /ws/subscribe を付ける。
func wsURL(httpURL string) string {
	return strings.Replace(httpURL, "http://", "ws://", 1) + "/ws/subscribe"
}

// dialWS は ws:// 接続を開き、最初の DGRequest を JSON 送信する。
func dialWS(t *testing.T, ctx context.Context, url string, req *pb.DGRequest, header http.Header) *websocket.Conn {
	t.Helper()
	body, err := protojson.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	conn, _, err := websocket.Dial(ctx, url, &websocket.DialOptions{HTTPHeader: header})
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	if err := conn.Write(ctx, websocket.MessageText, body); err != nil {
		conn.Close(websocket.StatusInternalError, "")
		t.Fatalf("Write: %v", err)
	}
	return conn
}

// recvResp は 1 メッセージ受信して DGResponse として返す。
func recvResp(t *testing.T, ctx context.Context, conn *websocket.Conn) *pb.DGResponse {
	t.Helper()
	typ, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if typ != websocket.MessageText {
		t.Fatalf("frame type = %v, want Text", typ)
	}
	resp := &pb.DGResponse{}
	if err := protojson.Unmarshal(data, resp); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	return resp
}

// TestWSSubscribeUnauthed: 認証ヘッダなしで非 CONNECT を送ると
// 最初の受信が CANTDOTHIS+UNAUTHED でストリームが閉じる。
func TestWSSubscribeUnauthed(t *testing.T) {
	_, _, _, _, srv := setup(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn := dialWS(t, ctx, wsURL(srv.URL), &pb.DGRequest{
		Version: 1,
		Event:   pb.DGEvent_GETDEVICE,
	}, nil)
	defer conn.Close(websocket.StatusNormalClosure, "")

	resp := recvResp(t, ctx, conn)
	if resp.GetEvent() != pb.DGEvent_CANTDOTHIS || resp.GetError() != pb.DGError_UNAUTHED {
		t.Errorf("want CANTDOTHIS+UNAUTHED, got event=%v error=%v", resp.GetEvent(), resp.GetError())
	}

	if _, _, err := conn.Read(ctx); err == nil {
		t.Error("expected stream to end after CANTDOTHIS+UNAUTHED")
	}
}

// TestWSSubscribeConnect: 最初のリクエストを CONNECT にすると最初の受信が
// CONNECT レスポンスでトークンが空でない。
func TestWSSubscribeConnect(t *testing.T) {
	_, _, _, _, srv := setup(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn := dialWS(t, ctx, wsURL(srv.URL), &pb.DGRequest{
		Version: 1,
		Event:   pb.DGEvent_CONNECT,
		Connect: &pb.DGRequest_DGConnect{AppName: "ws-tester", Uuid: "uuid-ws"},
	}, nil)
	defer conn.Close(websocket.StatusNormalClosure, "")

	resp := recvResp(t, ctx, conn)
	if resp.GetEvent() != pb.DGEvent_CONNECT {
		t.Errorf("want CONNECT, got %v", resp.GetEvent())
	}
	if tok := resp.GetConnect().GetToken(); tok == "" {
		t.Error("CONNECT response should have non-empty token")
	}
}

// TestWSSubscribeQueryToken: ?token= クエリで認証経路 (tokenQueryMiddleware) を通すと
// UNAUTHED が返らず購読ループに入る。
func TestWSSubscribeQueryToken(t *testing.T) {
	_, _, _, c, srv := setup(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tok := connectApp(t, c, "ws-q", "uuid-ws-q")
	url := wsURL(srv.URL) + "?token=" + tok

	conn := dialWS(t, ctx, url, &pb.DGRequest{
		Version: 1,
		Event:   pb.DGEvent_GETDEVICE,
	}, nil)
	defer conn.Close(websocket.StatusNormalClosure, "")

	// 認証通過なら subscribe ループに入るだけで何も push されない。
	// 短いデッドラインで Read → タイムアウトで購読中とみなす。
	// UNAUTHED が返ったらテスト失敗。
	rctx, rcancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer rcancel()
	_, data, err := conn.Read(rctx)
	if err == nil {
		resp := &pb.DGResponse{}
		_ = protojson.Unmarshal(data, resp)
		if resp.GetEvent() == pb.DGEvent_CANTDOTHIS && resp.GetError() == pb.DGError_UNAUTHED {
			t.Errorf("query-token auth should succeed, got UNAUTHED")
		}
		return
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("unexpected Read error: %v", err)
	}
}
