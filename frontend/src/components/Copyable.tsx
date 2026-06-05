import { useCopy } from "../useCopy";

// Copyable はクリックでテキストをクリップボードにコピーする <code> 風表示。
export default function Copyable({ text }: { text: string }) {
  const [copied, copy] = useCopy();

  return (
    <button
      type="button"
      onClick={() => copy(text)}
      title="クリックでコピー"
      className="rounded bg-slate-700/60 px-1.5 py-0.5 font-mono text-slate-200 hover:bg-slate-600"
    >
      {text}
      <span className="ml-1 text-[10px] text-emerald-400">{copied ? "✓ コピー済" : "⧉"}</span>
    </button>
  );
}
