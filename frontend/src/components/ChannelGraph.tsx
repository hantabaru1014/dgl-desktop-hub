import { useEffect, useRef } from "react";
import { getBuffer, MAX_POINTS, subscribe, type Point } from "../graphStore";

type Props = {
  deviceId: string;
  channel: "a" | "b";
  softLimit: number;
  color: string;
};

// ChannelGraph はチャンネルの強度を時間軸 (左へ流れる) の棒グラフで描画する。
// 棒の高さ = 実効出力(波形で変調された強度)、薄い線 = チャンネル強度、
// 破線 = ソフトリミット。
// 縦軸の最大は「ソフトリミット+1」とし、強度はリミットを超えないため常に収まる。
export default function ChannelGraph({ deviceId, channel, softLimit, color }: Props) {
  const canvasRef = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext("2d");
    if (!ctx) return;

    const draw = () => {
      const dpr = window.devicePixelRatio || 1;
      const w = canvas.clientWidth;
      const h = canvas.clientHeight;
      if (canvas.width !== w * dpr || canvas.height !== h * dpr) {
        canvas.width = w * dpr;
        canvas.height = h * dpr;
      }
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
      ctx.clearRect(0, 0, w, h);

      // 背景グリッド
      ctx.strokeStyle = "rgba(148,163,184,0.12)";
      ctx.lineWidth = 1;
      for (let i = 1; i < 4; i++) {
        const y = (h * i) / 4;
        ctx.beginPath();
        ctx.moveTo(0, y);
        ctx.lineTo(w, y);
        ctx.stroke();
      }

      // 縦軸最大 = ソフトリミット+1。
      const maxVal = softLimit + 1;

      const buf = getBuffer(deviceId);
      const points: Point[] = channel === "a" ? buf.a : buf.b;
      const n = Math.min(points.length, MAX_POINTS);
      const barW = w / MAX_POINTS;

      // 棒 (実効出力)。最新が右端に来るよう右詰めで描画。
      for (let i = 0; i < n; i++) {
        const p = points[points.length - n + i];
        const x = w - (n - i) * barW;
        const barH = Math.min(p.o / maxVal, 1) * h;
        ctx.fillStyle = color;
        ctx.fillRect(x, h - barH, Math.max(barW - 0.5, 1), barH);
      }

      // 強度ライン (リミットを超える分は上端でクリップ)
      ctx.strokeStyle = "rgba(226,232,240,0.5)";
      ctx.lineWidth = 1;
      ctx.beginPath();
      for (let i = 0; i < n; i++) {
        const p = points[points.length - n + i];
        const x = w - (n - i) * barW + barW / 2;
        const y = h - Math.min(p.s / maxVal, 1) * h;
        if (i === 0) ctx.moveTo(x, y);
        else ctx.lineTo(x, y);
      }
      ctx.stroke();

      // ソフトリミット破線
      const limY = h - Math.min(softLimit / maxVal, 1) * h;
      ctx.strokeStyle = "rgba(248,113,113,0.7)";
      ctx.setLineDash([4, 4]);
      ctx.beginPath();
      ctx.moveTo(0, limY);
      ctx.lineTo(w, limY);
      ctx.stroke();
      ctx.setLineDash([]);
    };
    // 初回描画 + フレーム到着 (100ms 周期) ごとに再描画。
    draw();
    return subscribe(draw);
  }, [deviceId, channel, softLimit, color]);

  return <canvas ref={canvasRef} className="w-full h-16 rounded bg-slate-900/60" />;
}
