import { useState } from "react";
import appProto from "../../../proto/com/github/opendglab/app.proto?raw";

type Props = {
  baseUrl: string; // 例: http://localhost:7330
  onClose: () => void;
};

// CodeBlock は複数行のコマンド例をコピーボタン付きで表示する。
function CodeBlock({ code }: { code: string }) {
  const [copied, setCopied] = useState(false);
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(code);
    } catch {
      const ta = document.createElement("textarea");
      ta.value = code;
      document.body.appendChild(ta);
      ta.select();
      document.execCommand("copy");
      document.body.removeChild(ta);
    }
    setCopied(true);
    setTimeout(() => setCopied(false), 1200);
  };
  return (
    <div className="group relative">
      <pre className="overflow-x-auto rounded-lg bg-slate-950/70 p-3 text-[11px] leading-relaxed text-slate-200">
        <code>{code}</code>
      </pre>
      <button
        type="button"
        onClick={copy}
        className="absolute right-2 top-2 rounded bg-slate-700/80 px-2 py-0.5 text-[10px] text-slate-100 opacity-0 transition group-hover:opacity-100 hover:bg-slate-600"
      >
        {copied ? "✓ コピー済" : "⧉ コピー"}
      </button>
    </div>
  );
}

function Section({ title, desc, children }: { title: string; desc?: string; children: React.ReactNode }) {
  return (
    <div className="space-y-1.5">
      <h3 className="text-sm font-bold text-slate-100">{title}</h3>
      {desc && <p className="text-xs text-slate-400">{desc}</p>}
      {children}
    </div>
  );
}

// ProtocolHelpModal はアプリ操作側プロトコル (OpenDGLab / Connect) の概要と
// curl での接続・操作例を表示する。
export default function ProtocolHelpModal({ baseUrl, onClose }: Props) {
  const send = `${baseUrl}/com.github.opendglab.OpenDGLabService/Send`;

  const connect = `curl -X POST ${send} \\
  -H "Content-Type: application/json" \\
  -d '{"version":1,"event":"CONNECT","connect":{"appName":"my-app","uuid":"my-uuid"}}'
# => {"version":1,"event":"CONNECT","connect":{"token":"<発行されたトークン>"}}`;

  const getDevice = `curl -X POST ${send} \\
  -H "Content-Type: application/json" \\
  -H "X-DGLab-Token: <token>" \\
  -d '{"version":1,"event":"GETDEVICE"}'
# => deviceList にデバイス一覧 (id を以降の deviceId に使う)`;

  const setStrength = `curl -X POST ${send} \\
  -H "Content-Type: application/json" \\
  -H "X-DGLab-Token: <token>" \\
  -d '{"version":1,"event":"SETSTRENGTH","device":{"deviceId":"<id>"},"strength":{"strengthA":20,"strengthB":0}}'`;

  const waveList = `curl -X POST ${send} \\
  -H "Content-Type: application/json" \\
  -H "X-DGLab-Token: <token>" \\
  -d '{"version":1,"event":"GETWAVELIST"}'`;

  const setWave = `curl -X POST ${send} \\
  -H "Content-Type: application/json" \\
  -H "X-DGLab-Token: <token>" \\
  -d '{"version":1,"event":"SETWAVE","device":{"deviceId":"<id>","deviceChannel":"CHANNEL_A"},"wave":{"waveName":"Breathing"}}'`;

  // downloadProto は app.proto をファイルとして保存させる。
  const downloadProto = () => {
    const blob = new Blob([appProto], { type: "text/plain;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = "app.proto";
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"
      onClick={onClose}
    >
      <div
        className="max-h-[85vh] w-full max-w-2xl overflow-y-auto rounded-xl border border-slate-600 bg-slate-800 p-5 shadow-2xl"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="mb-3 flex items-center justify-between">
          <h2 className="text-base font-bold text-slate-100">アプリ操作プロトコル / curl 例</h2>
          <button
            className="rounded-lg bg-slate-600 px-3 py-1 text-sm text-white hover:bg-slate-500"
            onClick={onClose}
          >
            閉じる
          </button>
        </div>

        <div className="space-y-4">
          <div className="rounded-lg bg-slate-900/50 p-3 text-xs text-slate-300">
            <p>
              OpenDGLab-OpenProtocol を{" "}
              <a
                href="https://connectrpc.com/docs/protocol"
                className="text-indigo-300 underline"
                target="_blank"
                rel="noreferrer"
              >
                Connect プロトコル
              </a>{" "}
              (JSON/binary、gRPC・gRPC-Web 互換) で提供します。すべて{" "}
              <code className="rounded bg-slate-700/60 px-1">POST</code> の単発リクエストで、1 リクエスト =
              1 イベント (<code className="rounded bg-slate-700/60 px-1">DGRequest</code>) です。
            </p>
            <ul className="mt-2 list-disc space-y-0.5 pl-5">
              <li>
                エンドポイント:{" "}
                <code className="rounded bg-slate-700/60 px-1 font-mono">{send}</code>
              </li>
              <li>
                ヘッダ: <code className="rounded bg-slate-700/60 px-1">Content-Type: application/json</code>
              </li>
              <li>
                認証: CONNECT で得た token を{" "}
                <code className="rounded bg-slate-700/60 px-1">X-DGLab-Token</code> ヘッダに付与
              </li>
              <li>enum (event / deviceChannel) は名前の文字列で指定 (例: "SETSTRENGTH", "CHANNEL_A")</li>
            </ul>
          </div>

          <Section title="1. 接続 (CONNECT)" desc="トークンを発行します。以降のリクエストに付与してください。">
            <CodeBlock code={connect} />
          </Section>

          <Section title="2. デバイス一覧取得 (GETDEVICE)" desc="操作対象の deviceId を取得します。">
            <CodeBlock code={getDevice} />
          </Section>

          <Section title="3. 強度設定 (SETSTRENGTH)" desc="strengthA/B は絶対値。ハブ側のソフトリミットで頭打ちされます。">
            <CodeBlock code={setStrength} />
          </Section>

          <Section title="4. 波形リスト取得 (GETWAVELIST)">
            <CodeBlock code={waveList} />
          </Section>

          <Section title="5. 波形設定 (SETWAVE)" desc="deviceChannel は CHANNEL_A / CHANNEL_B。waveName はリストの名前。">
            <CodeBlock code={setWave} />
          </Section>

          <Section title="プロトコル定義 (app.proto)" desc="メッセージ / enum / サービスの完全な定義です。">
            <div className="mb-2">
              <button
                type="button"
                onClick={downloadProto}
                className="rounded-lg bg-indigo-700/70 px-3 py-1 text-xs text-indigo-100 hover:bg-indigo-600"
              >
                ⭳ app.proto をダウンロード
              </button>
            </div>
            <CodeBlock code={appProto} />
          </Section>
        </div>
      </div>
    </div>
  );
}
