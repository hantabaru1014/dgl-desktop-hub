// graphStore は graph:frame イベントで届くサンプルをデバイス毎のリングバッファに
// 蓄積する。React の再レンダリングを避けるため state ではなくモジュール変数で保持し、
// 描画側は subscribe でフレーム到着時にのみ再描画する (100ms 周期)。

export type Point = { s: number; o: number }; // s=強度(0..200), o=実効出力(0..200)
export type ChannelBuf = { a: Point[]; b: Point[] };

export const MAX_POINTS = 300; // 100ms 周期 * 300 = 30 秒分

const buffers = new Map<string, ChannelBuf>();
const listeners = new Set<() => void>();

// subscribe はフレーム到着時に呼ばれるリスナを登録し、解除関数を返す。
export function subscribe(fn: () => void): () => void {
  listeners.add(fn);
  return () => listeners.delete(fn);
}

type RawSample = {
  deviceId: string;
  a: { strength: number; amp: number; output: number };
  b: { strength: number; amp: number; output: number };
};

export function pushFrame(samples: RawSample[]) {
  for (const s of samples) {
    let buf = buffers.get(s.deviceId);
    if (!buf) {
      buf = { a: [], b: [] };
      buffers.set(s.deviceId, buf);
    }
    buf.a.push({ s: s.a.strength, o: s.a.output });
    buf.b.push({ s: s.b.strength, o: s.b.output });
    if (buf.a.length > MAX_POINTS) buf.a.shift();
    if (buf.b.length > MAX_POINTS) buf.b.shift();
  }
  for (const fn of listeners) fn();
}

export function getBuffer(deviceId: string): ChannelBuf {
  return buffers.get(deviceId) ?? { a: [], b: [] };
}
