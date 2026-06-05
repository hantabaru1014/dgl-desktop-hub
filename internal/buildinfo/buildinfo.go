// Package buildinfo はビルド時に注入されるアプリのメタ情報を保持する。
// CommitSHA はリリースビルド時に ldflags の -X で上書きされる
// (例: -X github.com/hantabaru1014/dgl-desktop-hub/internal/buildinfo.CommitSHA=abc1234)。
// 注入されない開発ビルドでは既定値の "dev" のまま。
package buildinfo

// RepoURL はこのソフトウェアのソースリポジトリ URL。
const RepoURL = "https://github.com/hantabaru1014/dgl-desktop-hub"

// CommitSHA はビルド元のコミット SHA。ldflags で注入される。
var CommitSHA = "dev"
