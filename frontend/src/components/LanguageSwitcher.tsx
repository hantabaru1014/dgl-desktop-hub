import { useTranslation } from "react-i18next";
import { SUPPORTED_LANGUAGES } from "../i18n";

// LanguageSwitcher は UI 言語を切り替えるドロップダウン。
// 選択は i18next の languagedetector により localStorage に永続化される。
export default function LanguageSwitcher() {
  const { i18n, t } = useTranslation();
  // "ja-JP" など地域付きでも先頭2文字で現在値を判定する。
  const current = i18n.resolvedLanguage ?? i18n.language;
  const label = t("language.label");

  return (
    <select
      aria-label={label}
      title={label}
      value={current}
      onChange={(e) => void i18n.changeLanguage(e.target.value)}
      className="rounded bg-slate-700 px-2 py-1 text-xs text-slate-100"
    >
      {SUPPORTED_LANGUAGES.map((l) => (
        <option key={l.code} value={l.code}>
          {l.label}
        </option>
      ))}
    </select>
  );
}
