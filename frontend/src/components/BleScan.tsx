import { useState } from "react";
import { HubService } from "../api";

type ScanRow = { address: string; name: string; rssi: number };

// BleScan は BLE スキャン → 接続を行うボタン + 結果リスト。
export default function BleScan() {
  const [scanning, setScanning] = useState(false);
  const [results, setResults] = useState<ScanRow[]>([]);
  const [open, setOpen] = useState(false);
  const [error, setError] = useState("");
  const [connecting, setConnecting] = useState<string | null>(null);

  const scan = async () => {
    setScanning(true);
    setError("");
    setOpen(true);
    try {
      const rows = await HubService.ScanBLE(6);
      setResults(rows as ScanRow[]);
    } catch (e: any) {
      setError(String(e?.message ?? e));
    } finally {
      setScanning(false);
    }
  };

  const connect = async (addr: string, name: string) => {
    setError("");
    setConnecting(addr);
    try {
      await HubService.ConnectBLE(addr, name);
      setOpen(false);
    } catch (e: any) {
      setError(String(e?.message ?? e));
    } finally {
      setConnecting(null);
    }
  };

  return (
    <div className="relative">
      <button
        className="rounded-lg bg-emerald-700 px-4 py-2 text-sm font-semibold text-white hover:bg-emerald-600 disabled:opacity-50"
        onClick={scan}
        disabled={scanning}
      >
        {scanning ? "スキャン中…" : "BLE デバイスをスキャン"}
      </button>

      {open && (
        <div className="absolute z-20 mt-2 w-80 rounded-lg border border-slate-700 bg-slate-800 p-3 shadow-xl">
          <div className="mb-2 flex items-center justify-between">
            <span className="text-sm font-semibold">スキャン結果</span>
            <button className="text-xs text-slate-400 hover:text-slate-200" onClick={() => setOpen(false)}>
              閉じる
            </button>
          </div>
          {error && <div className="mb-2 text-xs text-rose-400">{error}</div>}
          {results.length === 0 && !scanning ? (
            <div className="text-xs text-slate-500">Coyote が見つかりませんでした。</div>
          ) : (
            <ul className="space-y-1">
              {results.map((r) => (
                <li key={r.address} className="flex items-center justify-between rounded bg-slate-900/60 px-2 py-1.5">
                  <div className="min-w-0">
                    <div className="truncate text-sm">{r.name}</div>
                    <div className="truncate text-[10px] text-slate-500">
                      {r.address} · {r.rssi}dBm
                    </div>
                  </div>
                  <button
                    className="ml-2 shrink-0 rounded bg-emerald-600 px-2 py-1 text-xs text-white hover:bg-emerald-500 disabled:cursor-not-allowed disabled:opacity-50"
                    onClick={() => connect(r.address, r.name)}
                    disabled={connecting !== null}
                  >
                    {connecting === r.address ? "接続中…" : "接続"}
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
    </div>
  );
}
