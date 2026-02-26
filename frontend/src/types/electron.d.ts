type XsInvokeChannel =
  | "app.ping"
  | "wallet.init"
  | "wallet.getStatus"
  | "wallet.unlock"
  | "wallet.lock"
  | "wallet.getBalances"
  | "wallet.getNewAddress"
  | "wallet.listAddresses"
  | "wallet.listUtxos"
  | "wallet.listTransactions"
  | "wallet.sendOnchain"
  | "swap.create"
  | "swap.check"
  | "swap.list"
  | "swap.get"
  | "swap.getEvents"
  | "swap.watchAll"
  | "nodes.list"
  | "nodes.start"
  | "nodes.stop"
  | "nodes.restart"
  | "nodes.watchLogs"
  | "binaries.ensureInstalled";

type XsInvoke = (channel: XsInvokeChannel, payload?: unknown) => Promise<unknown>;

interface Window {
  xs?: {
    invoke: XsInvoke;
    onSwapWatchAll?: (listener: (payload: unknown) => void) => () => void;
  };
}
