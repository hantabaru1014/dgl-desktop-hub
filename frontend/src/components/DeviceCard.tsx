import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { HubService, type DeviceDTO, type PresetDTO } from "../api";
import ChannelGraph from "./ChannelGraph";

type Props = {
  device: DeviceDTO;
  presets: PresetDTO[];
};

const kindLabel: Record<string, string> = {
  demo: "Demo",
  ble: "BLE",
  socket: "Socket",
};

const kindColor: Record<string, string> = {
  demo: "bg-slate-600",
  ble: "bg-emerald-600",
  socket: "bg-sky-600",
};

export default function DeviceCard({ device, presets }: Props) {
  const { t } = useTranslation();
  // ソフトリミットは A/B を同時に送る必要があるため、片方の変更時にもう一方は
  // 現在のバックエンド値を使う。
  const setLimit = (channel: "a" | "b", v: number) => {
    const a = channel === "a" ? v : device.softLimitA;
    const b = channel === "b" ? v : device.softLimitB;
    void HubService.SetSoftLimit(device.id, a, b);
  };

  return (
    <div className="rounded-xl border border-slate-700 bg-slate-800/60 p-4 shadow-lg">
      <div className="mb-3 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <span
            className={`rounded px-2 py-0.5 text-xs font-bold text-white ${kindColor[device.kind] ?? "bg-slate-600"}`}
          >
            {kindLabel[device.kind] ?? device.kind}
          </span>
          <span className="font-semibold">{device.name}</span>
          <span className="text-xs text-slate-400">({device.status})</span>
          {device.battery >= 0 && <span className="text-xs text-slate-400">🔋 {device.battery}%</span>}
        </div>
        <div className="flex items-center gap-3">
          {device.exclusive && (
            <span className="rounded bg-amber-900/50 px-2 py-0.5 text-xs text-amber-300">
              {device.owner ? t("deviceCard.occupiedBy", { owner: device.owner }) : t("deviceCard.noExclusiveApp")}
            </span>
          )}
          <label className="flex items-center gap-1 text-xs text-slate-300" title={t("deviceCard.exclusiveTooltip")}>
            <input
              type="checkbox"
              checked={device.exclusive}
              onChange={(e) => void HubService.SetDeviceExclusive(device.id, e.target.checked)}
            />
            {t("deviceCard.exclusiveMode")}
          </label>
          <button
            className="rounded bg-rose-700/80 px-2 py-1 text-xs text-white hover:bg-rose-600"
            onClick={() => void HubService.RemoveDevice(device.id)}
          >
            {t("common.delete")}
          </button>
        </div>
      </div>

      <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
        <ChannelPanel
          label="A"
          color="#34d399"
          deviceId={device.id}
          channel="a"
          channelIndex={0}
          currentStrength={device.strengthA}
          currentLimit={device.softLimitA}
          currentWave={device.waveA}
          presets={presets}
          onLimit={(v) => setLimit("a", v)}
        />
        <ChannelPanel
          label="B"
          color="#38bdf8"
          deviceId={device.id}
          channel="b"
          channelIndex={1}
          currentStrength={device.strengthB}
          currentLimit={device.softLimitB}
          currentWave={device.waveB}
          presets={presets}
          onLimit={(v) => setLimit("b", v)}
        />
      </div>
    </div>
  );
}

type PanelProps = {
  label: string;
  color: string;
  deviceId: string;
  channel: "a" | "b";
  channelIndex: number;
  currentStrength: number;
  currentLimit: number;
  currentWave: string;
  presets: PresetDTO[];
  onLimit: (v: number) => void;
};

function ChannelPanel({
  label,
  color,
  deviceId,
  channel,
  channelIndex,
  currentStrength,
  currentLimit,
  currentWave,
  presets,
  onLimit,
}: PanelProps) {
  const { t } = useTranslation();
  // バックエンドの現在値に追従しつつ、操作中はローカルで即時反映する。
  const [strength, setStrength] = useState(currentStrength);
  const [limit, setLimit] = useState(currentLimit);

  useEffect(() => setStrength(currentStrength), [currentStrength]);
  useEffect(() => setLimit(currentLimit), [currentLimit]);

  const applyStrength = (v: number) => {
    const c = clamp(v, limit); // ソフトリミットを超えて設定不可
    setStrength(c);
    void HubService.SetStrength(deviceId, channelIndex, 0, c); // mode 0 = 絶対
  };
  const applyLimit = (v: number) => {
    const c = clamp(v);
    setLimit(c);
    // リミットを下げたら現在強度も追従して切り下げる (バックエンドと同挙動)。
    if (strength > c) setStrength(c);
    onLimit(c);
  };

  const setPreset = (name: string) => {
    if (name === "__clear__") void HubService.ClearWaveform(deviceId, channelIndex);
    else void HubService.SetWaveformPreset(deviceId, channelIndex, name);
  };

  return (
    <div className="rounded-lg bg-slate-900/40 p-3">
      <div className="mb-2 flex items-center justify-between">
        <span className="text-sm font-bold" style={{ color }}>
          {t("deviceCard.channel", { label })}
        </span>
        <select
          className="rounded bg-slate-700 px-2 py-1 text-xs text-slate-100"
          value={currentWave || "__clear__"}
          onChange={(e) => setPreset(e.target.value)}
        >
          <option value="__clear__">{t("deviceCard.noWaveform")}</option>
          {presets.map((p) => (
            <option key={p.name} value={p.name}>
              {p.name}
            </option>
          ))}
        </select>
      </div>

      <ChannelGraph deviceId={deviceId} channel={channel} softLimit={limit} color={color} />

      <div className="mt-3 space-y-2 text-xs">
        <SliderRow
          label={t("deviceCard.strength")}
          labelColor="text-slate-300"
          value={strength}
          max={limit}
          accent={color}
          onChange={applyStrength}
        />
        <SliderRow
          label={t("deviceCard.softLimit")}
          labelColor="text-rose-300"
          value={limit}
          accent="#f43f5e"
          onChange={applyLimit}
        />
      </div>
    </div>
  );
}

type SliderRowProps = {
  label: string;
  labelColor: string;
  value: number;
  max?: number;
  accent: string;
  onChange: (v: number) => void;
};

function SliderRow({ label, labelColor, value, max = 200, accent, onChange }: SliderRowProps) {
  return (
    <div>
      <div className={`flex items-center justify-between ${labelColor}`}>
        <span>{label}</span>
        <input
          type="number"
          min={0}
          max={max}
          value={value}
          className="w-16 rounded bg-slate-700 px-1.5 py-0.5 text-right text-slate-100"
          onChange={(e) => onChange(Number(e.target.value))}
        />
      </div>
      <input
        type="range"
        min={0}
        max={max}
        value={value}
        className="w-full"
        style={{ accentColor: accent }}
        onChange={(e) => onChange(Number(e.target.value))}
      />
    </div>
  );
}

function clamp(v: number, max = 200): number {
  if (Number.isNaN(v)) return 0;
  if (v < 0) return 0;
  if (v > max) return max;
  return Math.round(v);
}
