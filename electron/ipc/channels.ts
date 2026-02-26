export const IPC_CHANNELS = {
  ping: "app.ping",
  walletInit: "wallet.init",
  walletGetStatus: "wallet.getStatus",
  walletUnlock: "wallet.unlock",
  walletLock: "wallet.lock",
  walletGetBalances: "wallet.getBalances",
  walletGetNewAddress: "wallet.getNewAddress",
  walletListAddresses: "wallet.listAddresses",
  walletListUtxos: "wallet.listUtxos",
  walletListTransactions: "wallet.listTransactions",
  walletSendOnchain: "wallet.sendOnchain",
  swapCreate: "swap.create",
  swapCheck: "swap.check",
  swapList: "swap.list",
  swapGet: "swap.get",
  swapGetEvents: "swap.getEvents",
  swapWatchAll: "swap.watchAll",
  nodesList: "nodes.list",
  nodesStart: "nodes.start",
  nodesStop: "nodes.stop",
  nodesRestart: "nodes.restart",
  nodesWatchLogs: "nodes.watchLogs",
  binariesEnsureInstalled: "binaries.ensureInstalled"
} as const;

export type IpcChannel = (typeof IPC_CHANNELS)[keyof typeof IPC_CHANNELS];
