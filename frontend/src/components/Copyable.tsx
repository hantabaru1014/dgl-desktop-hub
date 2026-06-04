import { useState } from "react";

// Copyable はクリックでテキストをクリップボードにコピーする <code> 風表示。
export default function Copyable({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(text);
    } catch {
      // フォールバック (古い環境)
      const ta = document.createElement("textarea");
      ta.value = text;
      document.body.appendChild(ta);
      ta.select();
      document.execCommand("copy");
      document.body.removeChild(ta);
    }
    setCopied(true);
    setTimeout(() => setCopied(false), 1200);
  };

  return (
    <button
      type="button"
      onClick={copy}
      title="クリックでコピー"
      className="rounded bg-slate-700/60 px-1.5 py-0.5 font-mono text-slate-200 hover:bg-slate-600"
    >
      {text}
      <span className="ml-1 text-[10px] text-emerald-400">{copied ? "✓ コピー済" : "⧉"}</span>
    </button>
  );
}
