import { useEffect, useState } from "react";
import { Trans, useTranslation } from "react-i18next";
import { Events } from "@wailsio/runtime";
import { HubService, type ApprovalRequestDTO } from "../api";

// ApprovalModal は autoApprove=false 時の接続許可要求をモーダルで表示する。
export default function ApprovalModal() {
  const { t } = useTranslation();
  const [queue, setQueue] = useState<ApprovalRequestDTO[]>([]);

  useEffect(() => {
    const off = Events.On("app:approval", (e: { data: ApprovalRequestDTO }) => {
      if (e.data) setQueue((q) => [...q, e.data]);
    });
    return () => off?.();
  }, []);

  const current = queue[0];
  if (!current) return null;

  const respond = (approve: boolean) => {
    void HubService.ApproveConnection(current.requestId, approve);
    setQueue((q) => q.slice(1));
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
      <div className="w-80 rounded-xl border border-slate-600 bg-slate-800 p-5 shadow-2xl">
        <h2 className="text-base font-bold text-slate-100">{t("approval.title")}</h2>
        <p className="mt-2 text-sm text-slate-300">
          <Trans
            i18nKey="approval.message"
            values={{ name: current.appName || t("approval.noName") }}
            components={[<span className="font-semibold text-indigo-300" />]}
          />
        </p>
        {current.uuid && <p className="mt-1 text-[11px] text-slate-500">UUID: {current.uuid}</p>}
        <div className="mt-4 flex justify-end gap-2">
          <button
            className="rounded-lg bg-slate-600 px-3 py-1.5 text-sm text-white hover:bg-slate-500"
            onClick={() => respond(false)}
          >
            {t("approval.deny")}
          </button>
          <button
            className="rounded-lg bg-emerald-600 px-3 py-1.5 text-sm text-white hover:bg-emerald-500"
            onClick={() => respond(true)}
          >
            {t("approval.approve")}
          </button>
        </div>
      </div>
    </div>
  );
}
