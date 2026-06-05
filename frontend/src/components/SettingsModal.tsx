import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { HubService, type AppInfoDTO } from "../api";
import { SUPPORTED_LANGUAGES } from "../i18n";
import { useCopy } from "../useCopy";
import { THIRD_PARTY_LICENSES } from "../data/thirdPartyLicenses";

type Props = {
  onClose: () => void;
};

type Tab = "general" | "about" | "licenses";

// SettingsModal は歯車ボタンから開く設定モーダル。
// 言語設定・ソフトウェア概要・サードパーティライセンスをタブで切り替えて表示する。
export default function SettingsModal({ onClose }: Props) {
  const { t, i18n } = useTranslation();
  const [tab, setTab] = useState<Tab>("general");

  const tabs: { key: Tab; label: string }[] = [
    { key: "general", label: t("settings.tabGeneral") },
    { key: "about", label: t("settings.tabAbout") },
    { key: "licenses", label: t("settings.tabLicenses") },
  ];

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"
      onClick={onClose}
    >
      <div
        className="flex h-[85vh] w-full max-w-2xl flex-col overflow-hidden rounded-xl border border-slate-600 bg-slate-800 shadow-2xl"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center justify-between border-b border-slate-700 px-5 py-3">
          <h2 className="text-base font-bold text-slate-100">{t("settings.title")}</h2>
          <button
            className="rounded-lg bg-slate-600 px-3 py-1 text-sm text-white hover:bg-slate-500"
            onClick={onClose}
          >
            {t("common.close")}
          </button>
        </div>

        <div className="flex border-b border-slate-700 px-5">
          {tabs.map((tb) => (
            <button
              key={tb.key}
              onClick={() => setTab(tb.key)}
              className={`-mb-px border-b-2 px-3 py-2 text-sm transition ${
                tab === tb.key
                  ? "border-indigo-400 text-slate-100"
                  : "border-transparent text-slate-400 hover:text-slate-200"
              }`}
            >
              {tb.label}
            </button>
          ))}
        </div>

        <div className="min-h-0 flex-1 overflow-y-auto p-5">
          {tab === "general" && (
            <div className="space-y-2">
              <label className="block text-sm font-medium text-slate-200" htmlFor="settings-language">
                {t("language.label")}
              </label>
              <select
                id="settings-language"
                value={i18n.resolvedLanguage ?? i18n.language}
                onChange={(e) => void i18n.changeLanguage(e.target.value)}
                className="rounded bg-slate-700 px-2 py-1.5 text-sm text-slate-100"
              >
                {SUPPORTED_LANGUAGES.map((l) => (
                  <option key={l.code} value={l.code}>
                    {l.label}
                  </option>
                ))}
              </select>
            </div>
          )}

          {tab === "about" && <AboutTab />}
          {tab === "licenses" && <LicensesTab />}
        </div>
      </div>
    </div>
  );
}

// AboutTab はアプリ名・リポジトリ URL・ビルド元コミット SHA を表示する。
function AboutTab() {
  const { t } = useTranslation();
  const [info, setInfo] = useState<AppInfoDTO | null>(null);
  const [copied, copy] = useCopy();

  useEffect(() => {
    void HubService.GetAppInfo().then(setInfo);
  }, []);

  if (!info) {
    return <p className="text-sm text-slate-400">…</p>;
  }

  return (
    <div className="space-y-3 text-sm">
      <div>
        <div className="text-xl font-bold text-slate-100">{info.name}</div>
      </div>
      <dl className="space-y-2">
        <div className="flex items-center gap-2">
          <dt className="w-28 shrink-0 text-slate-400">{t("settings.repository")}</dt>
          <dd>
            <a
              href={info.repoUrl}
              target="_blank"
              rel="noreferrer"
              className="break-all text-indigo-300 underline hover:text-indigo-200"
            >
              {info.repoUrl}
            </a>
          </dd>
        </div>
        <div className="flex items-center gap-2">
          <dt className="w-28 shrink-0 text-slate-400">{t("settings.commit")}</dt>
          <dd className="flex items-center gap-2">
            <code className="rounded bg-slate-700/60 px-1.5 py-0.5 font-mono text-slate-100">
              {info.commitSha}
            </code>
            <button
              type="button"
              onClick={() => void copy(info.commitSha)}
              className="rounded bg-slate-700/80 px-2 py-0.5 text-[11px] text-slate-100 hover:bg-slate-600"
            >
              {copied ? t("common.copied") : t("common.copy")}
            </button>
          </dd>
        </div>
      </dl>
    </div>
  );
}

// LicensesTab は同梱されるサードパーティ依存とそのライセンス表記を一覧表示する。
function LicensesTab() {
  const { t } = useTranslation();
  const groups = useMemo(() => {
    return [
      { kind: "protocol" as const, label: t("settings.licenseGroupProtocol") },
      { kind: "go" as const, label: t("settings.licenseGroupGo") },
      { kind: "frontend" as const, label: t("settings.licenseGroupFrontend") },
    ];
  }, [t]);

  return (
    <div className="space-y-4">
      <p className="text-xs text-slate-400">{t("settings.licenseIntro")}</p>
      {groups.map((g) => {
        const items = THIRD_PARTY_LICENSES.filter((l) => l.kind === g.kind);
        return (
          <div key={g.kind} className="space-y-1.5">
            <h3 className="text-sm font-bold text-slate-100">
              {g.label} ({items.length})
            </h3>
            <ul className="divide-y divide-slate-700/60 rounded-lg bg-slate-900/40">
              {items.map((l) => (
                <li key={l.name} className="px-3 py-2">
                  <div className="flex flex-wrap items-baseline justify-between gap-x-2">
                    <a
                      href={l.url}
                      target="_blank"
                      rel="noreferrer"
                      className="break-all font-mono text-xs text-indigo-300 underline hover:text-indigo-200"
                    >
                      {l.name}
                    </a>
                    <span className="shrink-0 rounded bg-slate-700/70 px-1.5 py-0.5 text-[10px] text-slate-200">
                      {l.license}
                    </span>
                  </div>
                  <div className="mt-0.5 text-[11px] text-slate-400">{l.copyright}</div>
                </li>
              ))}
            </ul>
          </div>
        );
      })}
    </div>
  );
}
