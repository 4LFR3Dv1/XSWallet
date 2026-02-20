export const IPC_CHANNELS = {
  ping: "app.ping",
  walletGetStatus: "wallet.getStatus",
  walletUnlock: "wallet.unlock",
  walletLock: "wallet.lock",
  walletGetBalances: "wallet.getBalances",
  swapCreate: "swap.create",
  swapCheck: "swap.check",
  swapList: "swap.list",
  swapGet: "swap.get",
  swapWatchAll: "swap.watchAll",
  nodesList: "nodes.list",
  nodesStart: "nodes.start",
  nodesStop: "nodes.stop",
  nodesRestart: "nodes.restart",
  nodesWatchLogs: "nodes.watchLogs",
  binariesEnsureInstalled: "binaries.ensureInstalled"
} as const;

export type IpcChannel = (typeof IPC_CHANNELS)[keyof typeof IPC_CHANNELS];
