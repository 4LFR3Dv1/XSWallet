import { contextBridge, ipcRenderer } from "electron";
import { IPC_CHANNELS, type IpcChannel } from "./ipc/channels";
import type { IpcErrResponse, IpcOkResponse } from "./ipc/errors";
import { IPC_EVENTS } from "./ipc/events";

const allowedChannels = new Set<IpcChannel>(Object.values(IPC_CHANNELS));

class XsIpcError extends Error {
  code: string;
  traceId: string;
  details?: unknown;

  constructor(code: string, message: string, traceId: string, details?: unknown) {
    super(message);
    this.name = "XsIpcError";
    this.code = code;
    this.traceId = traceId;
    this.details = details;
  }
}

contextBridge.exposeInMainWorld("xs", {
  invoke: async (channel: IpcChannel, payload?: unknown) => {
    if (!allowedChannels.has(channel)) {
      throw new Error(`IPC channel not allowed: ${channel}`);
    }
    const response = (await ipcRenderer.invoke(channel, payload)) as IpcOkResponse<unknown> | IpcErrResponse;
    if (response && typeof response === "object" && "ok" in response) {
      if (response.ok) {
        return response.data;
      }
      throw new XsIpcError(
        response.error.code,
        response.error.message,
        response.error.traceId,
        response.error.details
      );
    }
    return response;
  },
  onSwapWatchAll: (listener: (payload: unknown) => void) => {
    const wrapped = (_event: unknown, payload: unknown) => {
      listener(payload);
    };
    ipcRenderer.on(IPC_EVENTS.swapWatchAllUpdate, wrapped);
    return () => {
      ipcRenderer.removeListener(IPC_EVENTS.swapWatchAllUpdate, wrapped);
    };
  }
});
