import { useEffect, useRef, useState } from "react";
import QRCode from "qrcode";
import { HubService, type SocketServerDTO } from "../api";

function buildQR(host: string, port: number, controllerId: string): string {
  return `https://www.dungeon-lab.com/app-download.php#DGLAB-SOCKET#ws://${host}:${port}/${controllerId}`;
}

// SocketServerPanel は socket mode サーバの起動/停止、QR 表示 (アドレス編集可)、
// QR の画像保存を行う。
export default function SocketServerPanel() {
  const [info, setInfo] = useState<SocketServerDTO | null>(null);
  const [port, setPort] = useState<number>(9999);
  const [host, setHost] = useState<string>("");
  const [error, setError] = useState("");
  const canvasRef = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    void HubService.GetSocketServerInfo().then((i) => {
      setInfo(i);
      setPort(i.port);
      setHost(i.host);
    });
  }, []);

  // host / port / controllerId が変わるたびに QR を再生成 (リアルタイム)。
  const qrString =
    info?.running && info.controllerId ? buildQR(host, info.port, info.controllerId) : "";

  useEffect(() => {
    if (qrString && canvasRef.current) {
      void QRCode.toCanvas(canvasRef.current, qrString, { width: 200, margin: 1 });
    }
  }, [qrString]);

  const start = async () => {
    setError("");
    try {
      const i = await HubService.StartSocketServer(port);
      setInfo(i);
      setPort(i.port);
      if (!host) setHost(i.host);
    } catch (e: any) {
      setError(String(e?.message ?? e));
    }
  };
  const stop = async () => {
    setError("");
    try {
      await HubService.StopSocketServer();
      setInfo(await HubService.GetSocketServerInfo());
    } catch (e: any) {
      setError(String(e?.message ?? e));
    }
  };

  const changeHost = (v: string) => {
    setHost(v);
    void HubService.SetSocketHost(v);
  };

  const saveQR = () => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const a = document.createElement("a");
    a.href = canvas.toDataURL("image/png");
    a.download = "dglab-socket-qr.png";
    a.click();
  };

  return (
    <div className="rounded-xl border border-slate-700 bg-slate-800/40 p-4">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-sm font-semibold text-slate-200">Socket Mode (スマホ接続)</h2>
          <p className="text-xs text-slate-500">
            DG-Lab アプリで QR をスキャンして Coyote を接続します。
          </p>
        </div>
        {info?.running ? (
          <button
            className="rounded-lg bg-rose-700 px-3 py-1.5 text-sm text-white hover:bg-rose-600"
            onClick={stop}
          >
            停止
          </button>
        ) : (
          <div className="flex items-center gap-2">
            <label className="flex items-center gap-1 text-xs text-slate-400">
              ポート
              <input
                type="number"
                min={1}
                max={65535}
                value={port}
                className="w-20 rounded bg-slate-700 px-1.5 py-0.5 text-right text-slate-100"
                onChange={(e) => setPort(Number(e.target.value))}
              />
            </label>
            <button
              className="rounded-lg bg-sky-700 px-3 py-1.5 text-sm text-white hover:bg-sky-600"
              onClick={start}
            >
              サーバを起動
            </button>
          </div>
        )}
      </div>

      {error && <div className="mt-2 text-xs text-rose-400">{error}</div>}

      {info?.running && (
        <div className="mt-3 flex items-start gap-4">
          <canvas ref={canvasRef} className="rounded bg-white p-1" />
          <div className="space-y-2 text-xs text-slate-400">
            <label className="block">
              <span className="text-slate-300">QR アドレス (ホスト)</span>
              <input
                type="text"
                value={host}
                placeholder="192.168.x.x"
                className="mt-1 w-48 rounded bg-slate-700 px-2 py-1 text-slate-100"
                onChange={(e) => changeHost(e.target.value)}
              />
              <span className="ml-2 text-slate-500">:{info.port}</span>
            </label>
            <button
              className="rounded bg-slate-600 px-3 py-1 text-slate-100 hover:bg-slate-500"
              onClick={saveQR}
            >
              QR を画像で保存
            </button>
            <div className="break-all text-[10px] text-slate-500">{qrString}</div>
          </div>
        </div>
      )}
    </div>
  );
}
