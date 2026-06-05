import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Events } from "@wailsio/runtime";
import {
  HubService,
  type DeviceDTO,
  type PresetDTO,
  type AccessModeDTO,
  type ServerInfoDTO,
  type AppDTO,
} from "./api";
import DeviceCard from "./components/DeviceCard";
import BleScan from "./components/BleScan";
import SocketServerPanel from "./components/SocketServerPanel";
import Copyable from "./components/Copyable";
import ApprovalModal from "./components/ApprovalModal";
import ProtocolHelpModal from "./components/ProtocolHelpModal";
import SettingsModal from "./components/SettingsModal";
import { pushFrame } from "./graphStore";

export default function App() {
  const { t } = useTranslation();
  const [devices, setDevices] = useState<DeviceDTO[]>([]);
  const [presets, setPresets] = useState<PresetDTO[]>([]);
  const [access, setAccess] = useState<AccessModeDTO>({ autoApprove: true });
  const [server, setServer] = useState<ServerInfoDTO | null>(null);
  const [apps, setApps] = useState<AppDTO[]>([]);
  const [appPort, setAppPort] = useState<number>(7330);
  const [showHelp, setShowHelp] = useState(false);
  const [showSettings, setShowSettings] = useState(false);

  useEffect(() => {
    void HubService.ListDevices().then(setDevices);
    void HubService.GetPresets().then(setPresets);
    void HubService.GetAccessMode().then(setAccess);
    void HubService.GetServerInfo().then((s) => {
      setServer(s);
      setAppPort(s.port);
    });
    void HubService.ListApps().then(setApps);

    const offDevices = Events.On("devices:changed", (e: { data: DeviceDTO[] }) => {
      setDevices(e.data ?? []);
    });
    const offGraph = Events.On("graph:frame", (e: { data: any[] }) => {
      pushFrame(e.data ?? []);
    });
    const offApps = Events.On("apps:changed", (e: { data: AppDTO[] }) => {
      setApps(e.data ?? []);
    });
    return () => {
      offDevices?.();
      offGraph?.();
      offApps?.();
    };
  }, []);

  const toggleAuto = (v: boolean) => {
    setAccess({ autoApprove: v });
    void HubService.SetAutoApprove(v);
  };

  return (
    <div className="min-h-full">
      <header className="sticky top-0 z-10 border-b border-slate-700 bg-slate-900/95 px-6 py-4 backdrop-blur">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <h1 className="text-xl font-bold text-slate-100">DG-LAB Desktop Hub</h1>
          <div className="flex items-center gap-4 text-sm">
            <label className="flex items-center gap-1.5">
              <input
                type="checkbox"
                checked={access.autoApprove}
                onChange={(e) => toggleAuto(e.target.checked)}
              />
              {t("header.autoApprove")}
            </label>
            <span className="text-slate-400">{t("header.connectedApps", { count: apps.length })}</span>
            <button
              type="button"
              aria-label={t("settings.title")}
              title={t("settings.title")}
              onClick={() => setShowSettings(true)}
              className="rounded p-1.5 text-slate-300 hover:bg-slate-700 hover:text-slate-100"
            >
              <svg
                xmlns="http://www.w3.org/2000/svg"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2"
                strokeLinecap="round"
                strokeLinejoin="round"
                className="h-5 w-5"
              >
                <circle cx="12" cy="12" r="3" />
                <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" />
              </svg>
            </button>
          </div>
        </div>
        {server && (
          <div className="mt-2 flex flex-wrap items-center gap-x-6 gap-y-1 text-xs text-slate-400">
            <span className="flex items-center gap-1">
              {t("header.connectGrpc")} <Copyable text={server.connect} />
            </span>
            <span className="flex items-center gap-1">
              {t("common.port")}:
              <input
                type="number"
                min={1}
                max={65535}
                value={appPort}
                className="w-20 rounded bg-slate-700 px-1.5 py-0.5 text-right text-slate-100"
                onChange={(e) => setAppPort(Number(e.target.value))}
              />
              <button
                className="rounded bg-slate-600 px-2 py-0.5 text-slate-100 hover:bg-slate-500"
                onClick={() =>
                  void HubService.SetAppServerPort(appPort).then((s) => {
                    setServer(s);
                    setAppPort(s.port);
                  })
                }
              >
                {t("common.apply")}
              </button>
            </span>
            <button
              className="rounded bg-indigo-700/70 px-2 py-0.5 text-indigo-100 hover:bg-indigo-600"
              onClick={() => setShowHelp(true)}
            >
              {t("header.protocolExamples")}
            </button>
          </div>
        )}
      </header>

      <main className="mx-auto max-w-5xl space-y-4 px-6 py-6">
        <div className="flex items-center gap-3">
          <button
            className="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white hover:bg-indigo-500"
            onClick={() => void HubService.AddDemoDevice("")}
          >
            {t("devices.addDemo")}
          </button>
          <BleScan />
          <span className="text-sm text-slate-400">{t("devices.count", { count: devices.length })}</span>
        </div>

        <SocketServerPanel />

        {devices.length === 0 ? (
          <div className="rounded-xl border border-dashed border-slate-700 p-10 text-center text-slate-500">
            {t("devices.empty")}
          </div>
        ) : (
          <div className="space-y-4">
            {devices.map((d) => (
              <DeviceCard key={d.id} device={d} presets={presets} />
            ))}
          </div>
        )}
      </main>

      <ApprovalModal />
      {showHelp && server && (
        <ProtocolHelpModal baseUrl={server.connect} onClose={() => setShowHelp(false)} />
      )}
      {showSettings && <SettingsModal onClose={() => setShowSettings(false)} />}
    </div>
  );
}
