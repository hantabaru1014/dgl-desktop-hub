import { Trans, useTranslation } from "react-i18next";
import appProto from "../../../proto/com/github/opendglab/app.proto?raw";
import { useCopy } from "../useCopy";

type Props = {
  baseUrl: string; // 例: http://localhost:7330
  onClose: () => void;
};

// Trans の埋め込みコード片で共有するインライン <code> スタイル。
const codeTag = <code className="rounded bg-slate-700/60 px-1" />;
const codeMonoTag = <code className="rounded bg-slate-700/60 px-1 font-mono" />;

// CodeBlock は複数行のコマンド例をコピーボタン付きで表示する。
function CodeBlock({ code }: { code: string }) {
  const { t } = useTranslation();
  const [copied, copy] = useCopy();
  return (
    <div className="group relative">
      <pre className="overflow-x-auto rounded-lg bg-slate-950/70 p-3 text-[11px] leading-relaxed text-slate-200">
        <code>{code}</code>
      </pre>
      <button
        type="button"
        onClick={() => copy(code)}
        className="absolute right-2 top-2 rounded bg-slate-700/80 px-2 py-0.5 text-[10px] text-slate-100 opacity-0 transition group-hover:opacity-100 hover:bg-slate-600"
      >
        {copied ? t("common.copied") : t("common.copy")}
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
  const { t } = useTranslation();
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
          <h2 className="text-base font-bold text-slate-100">{t("protocol.title")}</h2>
          <button
            className="rounded-lg bg-slate-600 px-3 py-1 text-sm text-white hover:bg-slate-500"
            onClick={onClose}
          >
            {t("common.close")}
          </button>
        </div>

        <div className="space-y-4">
          <div className="rounded-lg bg-slate-900/50 p-3 text-xs text-slate-300">
            <p>
              <Trans
                i18nKey="protocol.intro"
                components={[
                  <a
                    href="https://connectrpc.com/docs/protocol"
                    className="text-indigo-300 underline"
                    target="_blank"
                    rel="noreferrer"
                  />,
                  codeTag,
                  codeTag,
                ]}
              />
            </p>
            <ul className="mt-2 list-disc space-y-0.5 pl-5">
              <li>
                <Trans
                  i18nKey="protocol.endpoint"
                  values={{ url: send }}
                  components={[codeMonoTag]}
                />
              </li>
              <li>
                <Trans
                  i18nKey="protocol.header"
                  components={[codeTag]}
                />
              </li>
              <li>
                <Trans
                  i18nKey="protocol.auth"
                  components={[codeTag]}
                />
              </li>
              <li>{t("protocol.enumNote")}</li>
            </ul>
          </div>

          <Section title={t("protocol.section1Title")} desc={t("protocol.section1Desc")}>
            <CodeBlock code={connect} />
          </Section>

          <Section title={t("protocol.section2Title")} desc={t("protocol.section2Desc")}>
            <CodeBlock code={getDevice} />
          </Section>

          <Section title={t("protocol.section3Title")} desc={t("protocol.section3Desc")}>
            <CodeBlock code={setStrength} />
          </Section>

          <Section title={t("protocol.section4Title")}>
            <CodeBlock code={waveList} />
          </Section>

          <Section title={t("protocol.section5Title")} desc={t("protocol.section5Desc")}>
            <CodeBlock code={setWave} />
          </Section>

          <Section title={t("protocol.protoTitle")} desc={t("protocol.protoDesc")}>
            <div className="mb-2">
              <button
                type="button"
                onClick={downloadProto}
                className="rounded-lg bg-indigo-700/70 px-3 py-1 text-xs text-indigo-100 hover:bg-indigo-600"
              >
                {t("protocol.downloadProto")}
              </button>
            </div>
            <CodeBlock code={appProto} />
          </Section>
        </div>
      </div>
    </div>
  );
}
