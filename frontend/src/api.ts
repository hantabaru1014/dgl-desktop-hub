// api.ts は Wails 生成バインディングへの薄い再エクスポート。
import { HubService } from "../bindings/github.com/hantabaru1014/dgl-desktop-hub/services";

export { HubService };
export type {
  DeviceDTO,
  PresetDTO,
  AccessModeDTO,
  ServerInfoDTO,
  AppDTO,
  ScanResultDTO,
  SocketServerDTO,
  ApprovalRequestDTO,
  AppInfoDTO,
} from "../bindings/github.com/hantabaru1014/dgl-desktop-hub/services";
