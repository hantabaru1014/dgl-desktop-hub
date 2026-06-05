// thirdPartyLicenses は本ソフトウェアのバイナリに同梱・配布される
// サードパーティ依存とそのライセンス表記をまとめたもの。
// Go 依存は go.mod の require (direct/indirect 両方) を、
// frontend 依存は package.json の実行時依存 + 同梱される CSS (tailwindcss) を対象とする。
// MIT / BSD / ISC / Apache-2.0 はいずれも著作権表示の保持が必要なため記載している。

export type LicenseKind = "protocol" | "go" | "frontend";

export type ThirdPartyLicense = {
  name: string;
  kind: LicenseKind;
  license: string; // SPDX 識別子
  copyright: string;
  url: string;
};

export const THIRD_PARTY_LICENSES: ThirdPartyLicense[] = [
  // --- プロトコル定義 (派生元。AGPL-3.0) ---
  // proto/com/github/opendglab/app.proto は OpenDGLab-OpenProtocol の
  // メッセージ定義を基にしている。生成 Go コードはバイナリへ取り込まれ、
  // app.proto 自体も UI に同梱・ダウンロード提供されるため表記が必要。
  { name: "OpenDGLab-OpenProtocol", kind: "protocol", license: "AGPL-3.0", copyright: "Copyright (c) OpenDGLab", url: "https://github.com/OpenDGLab/OpenDGLab-OpenProtocol" },

  // --- Go (バックエンド: バイナリへコンパイルされ配布される) ---
  { name: "connectrpc.com/connect", kind: "go", license: "Apache-2.0", copyright: "Copyright 2021-2025 The Connect Authors", url: "https://github.com/connectrpc/connect-go" },
  { name: "github.com/coder/websocket", kind: "go", license: "ISC", copyright: "Copyright (c) 2025 Coder", url: "https://github.com/coder/websocket" },
  { name: "github.com/wailsapp/wails/v3", kind: "go", license: "MIT", copyright: "Copyright (c) 2018-Present Lea Anthony", url: "https://github.com/wailsapp/wails" },
  { name: "github.com/wailsapp/wails/webview2", kind: "go", license: "MIT", copyright: "Copyright (c) 2018-Present Lea Anthony", url: "https://github.com/wailsapp/wails" },
  { name: "golang.org/x/net", kind: "go", license: "BSD-3-Clause", copyright: "Copyright 2009 The Go Authors", url: "https://pkg.go.dev/golang.org/x/net" },
  { name: "golang.org/x/crypto", kind: "go", license: "BSD-3-Clause", copyright: "Copyright 2009 The Go Authors", url: "https://pkg.go.dev/golang.org/x/crypto" },
  { name: "golang.org/x/exp", kind: "go", license: "BSD-3-Clause", copyright: "Copyright 2009 The Go Authors", url: "https://pkg.go.dev/golang.org/x/exp" },
  { name: "golang.org/x/sys", kind: "go", license: "BSD-3-Clause", copyright: "Copyright 2009 The Go Authors", url: "https://pkg.go.dev/golang.org/x/sys" },
  { name: "golang.org/x/text", kind: "go", license: "BSD-3-Clause", copyright: "Copyright 2009 The Go Authors", url: "https://pkg.go.dev/golang.org/x/text" },
  { name: "google.golang.org/protobuf", kind: "go", license: "BSD-3-Clause", copyright: "Copyright (c) 2018 The Go Authors", url: "https://github.com/protocolbuffers/protobuf-go" },
  { name: "tinygo.org/x/bluetooth", kind: "go", license: "BSD-3-Clause", copyright: "Copyright (c) 2019-2025 TinyGo Authors", url: "https://github.com/tinygo-org/bluetooth" },
  { name: "dario.cat/mergo", kind: "go", license: "BSD-3-Clause", copyright: "Copyright (c) 2013 Dario Castañé; Copyright (c) 2012 The Go Authors", url: "https://github.com/darccio/mergo" },
  { name: "github.com/Microsoft/go-winio", kind: "go", license: "MIT", copyright: "Copyright (c) 2015 Microsoft", url: "https://github.com/microsoft/go-winio" },
  { name: "github.com/ProtonMail/go-crypto", kind: "go", license: "BSD-3-Clause", copyright: "Copyright (c) 2009 The Go Authors", url: "https://github.com/ProtonMail/go-crypto" },
  { name: "github.com/adrg/xdg", kind: "go", license: "MIT", copyright: "Copyright (c) 2014 Adrian-George Bostan", url: "https://github.com/adrg/xdg" },
  { name: "github.com/cloudflare/circl", kind: "go", license: "BSD-3-Clause", copyright: "Copyright (c) 2019 Cloudflare", url: "https://github.com/cloudflare/circl" },
  { name: "github.com/cyphar/filepath-securejoin", kind: "go", license: "BSD-3-Clause", copyright: "Copyright (C) 2014-2015 Docker Inc & Go Authors; Copyright (C) 2017-2024 SUSE LLC", url: "https://github.com/cyphar/filepath-securejoin" },
  { name: "github.com/ebitengine/purego", kind: "go", license: "Apache-2.0", copyright: "Copyright the Ebitengine Authors", url: "https://github.com/ebitengine/purego" },
  { name: "github.com/emirpasic/gods", kind: "go", license: "BSD-2-Clause", copyright: "Copyright (c) 2015, Emir Pasic", url: "https://github.com/emirpasic/gods" },
  { name: "github.com/go-git/gcfg", kind: "go", license: "BSD-3-Clause", copyright: "Copyright (c) 2012 Péter Surányi; Portions Copyright (c) 2009 The Go Authors", url: "https://github.com/go-git/gcfg" },
  { name: "github.com/go-git/go-billy/v5", kind: "go", license: "Apache-2.0", copyright: "Copyright 2017 Sourced Technologies S.L.", url: "https://github.com/go-git/go-billy" },
  { name: "github.com/go-git/go-git/v5", kind: "go", license: "Apache-2.0", copyright: "Copyright 2018 Sourced Technologies, S.L.", url: "https://github.com/go-git/go-git" },
  { name: "github.com/go-ole/go-ole", kind: "go", license: "MIT", copyright: "Copyright © 2013-2017 Yasuhiro Matsumoto", url: "https://github.com/go-ole/go-ole" },
  { name: "github.com/godbus/dbus/v5", kind: "go", license: "BSD-2-Clause", copyright: "Copyright (c) 2013, Georg Reinke and Google", url: "https://github.com/godbus/dbus" },
  { name: "github.com/golang/groupcache", kind: "go", license: "Apache-2.0", copyright: "Copyright 2012 Google Inc.", url: "https://github.com/golang/groupcache" },
  { name: "github.com/jbenet/go-context", kind: "go", license: "MIT", copyright: "Copyright (c) 2014 Juan Batiz-Benet", url: "https://github.com/jbenet/go-context" },
  { name: "github.com/jchv/go-winloader", kind: "go", license: "ISC", copyright: "Copyright © 2021, John Chadwick", url: "https://github.com/jchv/go-winloader" },
  { name: "github.com/kevinburke/ssh_config", kind: "go", license: "MIT", copyright: "Copyright (c) 2017 Kevin Burke", url: "https://github.com/kevinburke/ssh_config" },
  { name: "github.com/klauspost/cpuid/v2", kind: "go", license: "MIT", copyright: "Copyright (c) 2015 Klaus Post", url: "https://github.com/klauspost/cpuid" },
  { name: "github.com/mattn/go-colorable", kind: "go", license: "MIT", copyright: "Copyright (c) 2016 Yasuhiro Matsumoto", url: "https://github.com/mattn/go-colorable" },
  { name: "github.com/mattn/go-isatty", kind: "go", license: "MIT", copyright: "Copyright (c) Yasuhiro MATSUMOTO", url: "https://github.com/mattn/go-isatty" },
  { name: "github.com/pjbgf/sha1cd", kind: "go", license: "Apache-2.0", copyright: "Copyright 2023 pjbgf", url: "https://github.com/pjbgf/sha1cd" },
  { name: "github.com/saltosystems/winrt-go", kind: "go", license: "MIT", copyright: "Copyright (c) 2022 SALTO SYSTEMS, S.L", url: "https://github.com/saltosystems/winrt-go" },
  { name: "github.com/sergi/go-diff", kind: "go", license: "MIT", copyright: "Copyright (c) 2012-2016 The go-diff Authors", url: "https://github.com/sergi/go-diff" },
  { name: "github.com/sirupsen/logrus", kind: "go", license: "MIT", copyright: "Copyright (c) 2014 Simon Eskildsen", url: "https://github.com/sirupsen/logrus" },
  { name: "github.com/skeema/knownhosts", kind: "go", license: "Apache-2.0", copyright: "Copyright 2024 Skeema LLC", url: "https://github.com/skeema/knownhosts" },
  { name: "github.com/soypat/cyw43439", kind: "go", license: "MIT", copyright: "Copyright (c) 2022 Patricio Whittingslow", url: "https://github.com/soypat/cyw43439" },
  { name: "github.com/soypat/seqs", kind: "go", license: "BSD-3-Clause", copyright: "Copyright (c) 2023, Patricio Whittingslow", url: "https://github.com/soypat/seqs" },
  { name: "github.com/tinygo-org/cbgo", kind: "go", license: "Apache-2.0", copyright: "Copyright the cbgo Authors", url: "https://github.com/tinygo-org/cbgo" },
  { name: "github.com/tinygo-org/pio", kind: "go", license: "BSD-3-Clause", copyright: "Copyright (c) 2023-2025 The TinyGo Authors", url: "https://github.com/tinygo-org/pio" },
  { name: "github.com/xanzy/ssh-agent", kind: "go", license: "Apache-2.0", copyright: "Copyright 2015, Sander van Harmelen", url: "https://github.com/xanzy/ssh-agent" },
  { name: "gopkg.in/warnings.v0", kind: "go", license: "BSD-2-Clause", copyright: "Copyright (c) 2016 Péter Surányi", url: "https://gopkg.in/warnings.v0" },

  // --- Frontend (実行時依存 + 同梱 CSS) ---
  { name: "react", kind: "frontend", license: "MIT", copyright: "Copyright (c) Meta Platforms, Inc. and affiliates", url: "https://github.com/facebook/react" },
  { name: "react-dom", kind: "frontend", license: "MIT", copyright: "Copyright (c) Meta Platforms, Inc. and affiliates", url: "https://github.com/facebook/react" },
  { name: "i18next", kind: "frontend", license: "MIT", copyright: "Copyright (c) 2011-present i18next", url: "https://github.com/i18next/i18next" },
  { name: "i18next-browser-languagedetector", kind: "frontend", license: "MIT", copyright: "Copyright (c) 2025 i18next", url: "https://github.com/i18next/i18next-browser-languageDetector" },
  { name: "react-i18next", kind: "frontend", license: "MIT", copyright: "Copyright (c) 2015-present i18next", url: "https://github.com/i18next/react-i18next" },
  { name: "qrcode", kind: "frontend", license: "MIT", copyright: "Copyright (c) 2012 Ryan Day", url: "https://github.com/soldair/node-qrcode" },
  { name: "@wailsio/runtime", kind: "frontend", license: "MIT", copyright: "Copyright (c) 2018-Present The Wails Team", url: "https://github.com/wailsapp/wails" },
  { name: "tailwindcss", kind: "frontend", license: "MIT", copyright: "Copyright (c) Tailwind Labs, Inc.", url: "https://github.com/tailwindlabs/tailwindcss" },
];
