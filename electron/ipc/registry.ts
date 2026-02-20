import { IPC_CHANNELS, type IpcChannel } from "./channels";
import { IpcAppError, errInvalidArgument, errUnimplemented } from "./errors";

type ValidationResult = { ok: true } | { ok: false; reason: string };
type Validator = (payload: unknown) => ValidationResult;

export interface IpcContext {
  traceId: string;
  isVaultUnlocked: () => boolean;
  setVaultUnlocked: (value: boolean) => void;
  getSessionToken: () => string | null;
  setSessionToken: (value: string | null) => void;
  callApiBridge: <T>(path: string, init?: RequestInit) => Promise<T>;
  setSwapWatchSubscription: (senderId: number, subscribed: boolean) => void;
  senderId: number;
}

export interface IpcChannelDefinition {
  name: IpcChannel;
  access: "renderer_safe" | "privileged";
  mode: "read" | "mutate";
  requiresUnlocked: boolean;
  validate: Validator;
  handler: (payload: unknown, ctx: IpcContext) => Promise<unknown> | unknown;
}

const validateVoid: Validator = (payload) => {
  if (payload === undefined || payload === null) {
    return { ok: true };
  }
  return { ok: false, reason: "payload must be empty" };
};

const validateWalletUnlock: Validator = (payload) => {
  if (!payload || typeof payload !== "object") {
    return { ok: false, reason: "payload must be an object" };
  }
  const pin = (payload as { pin?: unknown }).pin;
  if (typeof pin !== "string" || pin.trim().length < 4) {
    return { ok: false, reason: "pin must be a string with at least 4 chars" };
  }
  return { ok: true };
};

const validateNodeAction: Validator = (payload) => {
  if (!payload || typeof payload !== "object") {
    return { ok: false, reason: "payload must be an object" };
  }
  const nodeType = (payload as { nodeType?: unknown }).nodeType;
  if (!["bitcoind", "elementsd", "lnd"].includes(String(nodeType))) {
    return { ok: false, reason: "nodeType must be bitcoind|elementsd|lnd" };
  }
  return { ok: true };
};

const validateBinaryEnsure: Validator = (payload) => {
  if (!payload || typeof payload !== "object") {
    return { ok: false, reason: "payload must be an object" };
  }
  const nodeType = (payload as { nodeType?: unknown }).nodeType;
  if (!["bitcoind", "elementsd", "lnd"].includes(String(nodeType))) {
    return { ok: false, reason: "nodeType must be bitcoind|elementsd|lnd" };
  }
  return { ok: true };
};

const validateSwapCreate: Validator = (payload) => {
  if (!payload || typeof payload !== "object") {
    return { ok: false, reason: "payload must be an object" };
  }
  const { fromChain, toChain, amountSats, from_chain, to_chain, amount_sats } = payload as {
    fromChain?: unknown;
    toChain?: unknown;
    amountSats?: unknown;
    from_chain?: unknown;
    to_chain?: unknown;
    amount_sats?: unknown;
  };
  const resolvedFrom = fromChain ?? from_chain;
  const resolvedTo = toChain ?? to_chain;
  const resolvedAmount = amountSats ?? amount_sats;
  if (typeof resolvedFrom !== "string" || typeof resolvedTo !== "string") {
    return { ok: false, reason: "fromChain and toChain must be strings" };
  }
  if (typeof resolvedAmount !== "number" || resolvedAmount <= 0) {
    return { ok: false, reason: "amountSats must be a positive number" };
  }
  return { ok: true };
};

const validateSwapGet: Validator = (payload) => {
  if (!payload || typeof payload !== "object") {
    return { ok: false, reason: "payload must be an object" };
  }
  const id = (payload as { id?: unknown }).id;
  if (typeof id !== "string" || id.trim().length < 1) {
    return { ok: false, reason: "id must be a non-empty string" };
  }
  return { ok: true };
};

const validateSwapCheck: Validator = (payload) => {
  if (!payload || typeof payload !== "object") {
    return { ok: false, reason: "payload must be an object" };
  }
  const id = (payload as { id?: unknown }).id;
  if (typeof id !== "string" || id.trim().length < 1) {
    return { ok: false, reason: "id must be a non-empty string" };
  }
  return { ok: true };
};

const validateSwapWatchAll: Validator = (payload) => {
  if (payload === undefined || payload === null) {
    return { ok: true };
  }
  if (typeof payload !== "object") {
    return { ok: false, reason: "payload must be an object when provided" };
  }
  const action = (payload as { action?: unknown }).action;
  if (action !== undefined && action !== "subscribe" && action !== "unsubscribe") {
    return { ok: false, reason: "action must be subscribe|unsubscribe when provided" };
  }
  return { ok: true };
};

function notImplemented(channel: IpcChannel): never {
  throw errUnimplemented(`Channel ${channel} not implemented yet`, { channel });
}

export const IPC_CHANNEL_REGISTRY: readonly IpcChannelDefinition[] = [
  {
    name: IPC_CHANNELS.ping,
    access: "renderer_safe",
    mode: "read",
    requiresUnlocked: false,
    validate: validateVoid,
    handler: (_payload, ctx) => ({ pong: true, traceId: ctx.traceId, source: "electron-main" })
  },
  {
    name: IPC_CHANNELS.walletGetStatus,
    access: "renderer_safe",
    mode: "read",
    requiresUnlocked: false,
    validate: validateVoid,
    handler: async (_payload, ctx) => {
      try {
        const status = await ctx.callApiBridge<{ state: "not_initialized" | "locked" | "unlocked" | "locked_out"; failed_attempts?: number }>(
          "/api/v1/wallet/status"
        );
        if (status.state !== "unlocked") {
          ctx.setVaultUnlocked(false);
          ctx.setSessionToken(null);
        }
        return status;
      } catch {
        return {
          state: ctx.isVaultUnlocked() ? "unlocked" : "locked"
        };
      }
    }
  },
  {
    name: IPC_CHANNELS.walletUnlock,
    access: "renderer_safe",
    mode: "mutate",
    requiresUnlocked: false,
    validate: validateWalletUnlock,
    handler: async (payload, ctx) => {
      const pin = (payload as { pin: string }).pin.trim();
      if (!pin) {
        throw errInvalidArgument("pin is required");
      }
      try {
        const response = await ctx.callApiBridge<{
          success: boolean;
          session_id?: string;
          error_message?: string;
          remaining_attempts?: number;
        }>("/api/v1/wallet/unlock", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ pin })
        });

        if (!response.success) {
          throw errInvalidArgument(response.error_message || "invalid pin", {
            remainingAttempts: response.remaining_attempts
          });
        }
        ctx.setVaultUnlocked(true);
        ctx.setSessionToken(response.session_id ?? null);
        return response;
      } catch (error) {
        if (error instanceof IpcAppError) {
          throw error;
        }
        throw errInvalidArgument("api-bridge unavailable for unlock");
      }
    }
  },
  {
    name: IPC_CHANNELS.walletLock,
    access: "privileged",
    mode: "mutate",
    requiresUnlocked: false,
    validate: validateVoid,
    handler: async (_payload, ctx) => {
      try {
        const response = await ctx.callApiBridge<{ success: boolean }>("/api/v1/wallet/lock", {
          method: "POST"
        });
        ctx.setVaultUnlocked(false);
        ctx.setSessionToken(null);
        return response;
      } catch {
        throw errInvalidArgument("api-bridge unavailable for lock");
      }
    }
  },
  {
    name: IPC_CHANNELS.walletGetBalances,
    access: "privileged",
    mode: "read",
    requiresUnlocked: true,
    validate: validateVoid,
    handler: async (_payload, ctx) => {
      try {
        return await ctx.callApiBridge<{
          btc: { confirmed: number; unconfirmed: number; pending_swap: number };
          liquid: { confirmed: number; unconfirmed: number; pending_swap: number };
          ln: { balance: number; pending_open: number; pending_close: number };
        }>("/api/v1/wallet/balances");
      } catch {
        return {
          btc: { confirmed: 0, unconfirmed: 0, pending_swap: 0 },
          liquid: { confirmed: 0, unconfirmed: 0, pending_swap: 0 },
          ln: { balance: 0, pending_open: 0, pending_close: 0 }
        };
      }
    }
  },
  {
    name: IPC_CHANNELS.swapCreate,
    access: "privileged",
    mode: "mutate",
    requiresUnlocked: true,
    validate: validateSwapCreate,
    handler: async (payload, ctx) => {
      const request = payload as {
        fromChain?: string;
        toChain?: string;
        amountSats?: number;
        invoice?: string;
        from_chain?: string;
        to_chain?: string;
        amount_sats?: number;
      };

      const body = {
        from_chain: request.from_chain ?? request.fromChain,
        to_chain: request.to_chain ?? request.toChain,
        amount_sats: request.amount_sats ?? request.amountSats,
        invoice: request.invoice
      };

      return ctx.callApiBridge<unknown>("/api/v1/swaps", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body)
      });
    }
  },
  {
    name: IPC_CHANNELS.swapList,
    access: "privileged",
    mode: "read",
    requiresUnlocked: true,
    validate: validateVoid,
    handler: async (_payload, ctx) => {
      return ctx.callApiBridge<unknown>("/api/v1/swaps");
    }
  },
  {
    name: IPC_CHANNELS.swapCheck,
    access: "privileged",
    mode: "mutate",
    requiresUnlocked: true,
    validate: validateSwapCheck,
    handler: async (payload, ctx) => {
      const id = encodeURIComponent((payload as { id: string }).id);
      return ctx.callApiBridge<unknown>(`/api/v1/swaps/${id}/check`, {
        method: "POST"
      });
    }
  },
  {
    name: IPC_CHANNELS.swapGet,
    access: "privileged",
    mode: "read",
    requiresUnlocked: true,
    validate: validateSwapGet,
    handler: async (payload, ctx) => {
      const id = encodeURIComponent((payload as { id: string }).id);
      return ctx.callApiBridge<unknown>(`/api/v1/swaps/${id}`);
    }
  },
  {
    name: IPC_CHANNELS.swapWatchAll,
    access: "privileged",
    mode: "read",
    requiresUnlocked: true,
    validate: validateSwapWatchAll,
    handler: (payload, ctx) => {
      const action = (payload as { action?: "subscribe" | "unsubscribe" } | undefined)?.action ?? "subscribe";
      const subscribed = action !== "unsubscribe";
      ctx.setSwapWatchSubscription(ctx.senderId, subscribed);
      return { subscribed };
    }
  },
  {
    name: IPC_CHANNELS.nodesList,
    access: "privileged",
    mode: "read",
    requiresUnlocked: true,
    validate: validateVoid,
    handler: async (_payload, ctx) => {
      try {
        const status = await ctx.callApiBridge<{
          services?: {
            xscore?: { status?: string };
            bitcoind?: { status?: string; blocks?: number };
            elementsd?: { status?: string; blocks?: number };
            lnd?: { status?: string };
          };
          bitcoin_block_height?: number;
        }>("/api/v1/system/status");

        const toNodeState = (raw: string | undefined): "running" | "stopped" | "unknown" => {
          if (raw === "running") return "running";
          if (raw === "stopped") return "stopped";
          return "unknown";
        };

        return {
          nodes: [
            {
              nodeType: "bitcoind",
              state: toNodeState(status.services?.bitcoind?.status),
              blocks: status.services?.bitcoind?.blocks ?? status.bitcoin_block_height ?? 0
            },
            {
              nodeType: "elementsd",
              state: toNodeState(status.services?.elementsd?.status)
            },
            {
              nodeType: "lnd",
              state: toNodeState(status.services?.lnd?.status)
            }
          ]
        };
      } catch {
        return {
          nodes: [
            { nodeType: "bitcoind", state: "stopped", blocks: 0 },
            { nodeType: "elementsd", state: "unknown" },
            { nodeType: "lnd", state: "unknown" }
          ]
        };
      }
    }
  },
  {
    name: IPC_CHANNELS.nodesStart,
    access: "privileged",
    mode: "mutate",
    requiresUnlocked: true,
    validate: validateNodeAction,
    handler: async (payload, ctx) => {
      const nodeType = (payload as { nodeType: string }).nodeType;
      return ctx.callApiBridge<unknown>(`/api/v1/nodes/${encodeURIComponent(nodeType)}/start`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({})
      });
    }
  },
  {
    name: IPC_CHANNELS.nodesStop,
    access: "privileged",
    mode: "mutate",
    requiresUnlocked: true,
    validate: validateNodeAction,
    handler: async (payload, ctx) => {
      const { nodeType, graceful } = payload as { nodeType: string; graceful?: boolean };
      return ctx.callApiBridge<unknown>(`/api/v1/nodes/${encodeURIComponent(nodeType)}/stop`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ graceful: graceful ?? true })
      });
    }
  },
  {
    name: IPC_CHANNELS.nodesRestart,
    access: "privileged",
    mode: "mutate",
    requiresUnlocked: true,
    validate: validateNodeAction,
    handler: async (payload, ctx) => {
      const nodeType = (payload as { nodeType: string }).nodeType;
      return ctx.callApiBridge<unknown>(`/api/v1/nodes/${encodeURIComponent(nodeType)}/restart`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({})
      });
    }
  },
  {
    name: IPC_CHANNELS.nodesWatchLogs,
    access: "privileged",
    mode: "read",
    requiresUnlocked: true,
    validate: validateNodeAction,
    handler: (_payload) => notImplemented(IPC_CHANNELS.nodesWatchLogs)
  },
  {
    name: IPC_CHANNELS.binariesEnsureInstalled,
    access: "privileged",
    mode: "mutate",
    requiresUnlocked: true,
    validate: validateBinaryEnsure,
    handler: (_payload) => notImplemented(IPC_CHANNELS.binariesEnsureInstalled)
  }
] as const;
