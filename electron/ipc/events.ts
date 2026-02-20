export const IPC_EVENTS = {
  swapWatchAllUpdate: "swap.watchAll.update"
} as const;

export type IpcEventChannel = (typeof IPC_EVENTS)[keyof typeof IPC_EVENTS];
