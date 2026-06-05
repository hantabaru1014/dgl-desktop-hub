import { useState } from "react";

// copyToClipboard はテキストをクリップボードへコピーする。
// Clipboard API が使えない古い環境では textarea + execCommand にフォールバックする。
export async function copyToClipboard(text: string) {
  try {
    await navigator.clipboard.writeText(text);
  } catch {
    const ta = document.createElement("textarea");
    ta.value = text;
    document.body.appendChild(ta);
    ta.select();
    document.execCommand("copy");
    document.body.removeChild(ta);
  }
}

// useCopy はコピー処理と「コピー済」表示状態 (一定時間で自動リセット) をまとめて返す。
export function useCopy(resetMs = 1200): [boolean, (text: string) => Promise<void>] {
  const [copied, setCopied] = useState(false);
  const copy = async (text: string) => {
    await copyToClipboard(text);
    setCopied(true);
    setTimeout(() => setCopied(false), resetMs);
  };
  return [copied, copy];
}
