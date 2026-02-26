import { randomUUID } from "node:crypto";
import path from "node:path";
import { app, BrowserWindow, ipcMain, session, shell, webContents } from "electron";
import type { IpcMainInvokeEvent } from "electron";
import { type IpcChannel } from "./ipc/channels";
import { errInvalidArgument, errResourceExhausted, errUnauthenticated, type IpcResponse, normalizeIpcError } from "./ipc/errors";
import { IPC_EVENTS } from "./ipc/events";
import { IPC_CHANNEL_REGISTRY, type IpcChannelDefinition } from "./ipc/registry";

const DEV_SERVER_URL = process.env.ELECTRON_RENDERER_URL ?? "http://127.0.0.1:5173";
const DEV_API_URL = process.env.ELECTRON_API_URL ?? "http://localhost:3000";
const IS_DEV = !app.isPackaged;
const DEV_API_AUTH_BEARER = process.env.ELECTRON_API_AUTH_BEARER ?? (IS_DEV ? "dev" : "");
const MUTATION_RATE_WINDOW_MS = 2000;
const MUTATION_RATE_MAX_CALLS = 5;
const SWAP_WATCH_POLL_INTERVAL_MS = 10000;

let vaultUnlocked = false;
let sessionToken: string | null = null;
const mutationCallsByChannel = new Map<IpcChannel, number[]>();
const swapWatchSubscribers = new Set<number>();
let swapWatchTimer: ReturnType<typeof setInterval> | null = null;
let lastSwapWatchPayload = "";

function buildCsp(): string {
  if (IS_DEV) {
    return [
      "default-src 'self' http://127.0.0.1:5173",
      "script-src 'self' 'unsafe-inline' 'unsafe-eval' http://127.0.0.1:5173",
      "style-src 'self' 'unsafe-inline' http://127.0.0.1:5173",
      "img-src 'self' data: blob: http://127.0.0.1:5173",
      `connect-src 'self' http://127.0.0.1:5173 ws://127.0.0.1:5173 ${DEV_API_URL}`,
      "object-src 'none'",
      "base-uri 'self'",
      "frame-ancestors 'none'"
    ].join("; ");
  }

  return [
    "default-src 'self'",
    "script-src 'self'",
    "style-src 'self' 'unsafe-inline'",
    "img-src 'self' data: blob:",
    "connect-src 'self'",
    "object-src 'none'",
    "base-uri 'self'",
    "frame-ancestors 'none'"
  ].join("; ");
}

function installCspHeader(): void {
  const csp = buildCsp();
  session.defaultSession.webRequest.onHeadersReceived((details, callback) => {
    const headers = details.responseHeaders ?? {};
    headers["Content-Security-Policy"] = [csp];
    callback({ responseHeaders: headers });
  });
}

function isAppNavigationUrl(url: string): boolean {
  try {
    const expected = new URL(DEV_SERVER_URL);
    const target = new URL(url);
    return target.origin === expected.origin;
  } catch {
    return false;
  }
}

function configureWindowSecurity(win: BrowserWindow): void {
  win.webContents.setWindowOpenHandler(({ url }) => {
    if (url.startsWith("http://") || url.startsWith("https://")) {
      void shell.openExternal(url);
    }
    return { action: "deny" };
  });

  win.webContents.on("will-navigate", (event, url) => {
    if (!isAppNavigationUrl(url)) {
      event.preventDefault();
      if (url.startsWith("http://") || url.startsWith("https://")) {
        void shell.openExternal(url);
      }
    }
  });
}

function createWindow(): BrowserWindow {
  const win = new BrowserWindow({
    width: 1360,
    height: 860,
    minWidth: 1024,
    minHeight: 680,
    autoHideMenuBar: true,
    webPreferences: {
      preload: path.join(__dirname, "preload.js"),
      contextIsolation: true,
      sandbox: true,
      nodeIntegration: false
    }
  });

  configureWindowSecurity(win);
  void win.loadURL(DEV_SERVER_URL);
  return win;
}

function logIpcResult(channel: IpcChannel, traceId: string, startedAtMs: number, result: "ok" | "err", code?: string): void {
  const durationMs = Date.now() - startedAtMs;
  const codeSuffix = code ? ` code=${code}` : "";
  console.log(`[ipc] channel=${channel} traceId=${traceId} durationMs=${durationMs} result=${result}${codeSuffix}`);
}

function checkMutationRateLimit(channel: IpcChannel): boolean {
  const now = Date.now();
  const calls = mutationCallsByChannel.get(channel) ?? [];
  const windowCalls = calls.filter((timestamp) => now - timestamp <= MUTATION_RATE_WINDOW_MS);
  windowCalls.push(now);
  mutationCallsByChannel.set(channel, windowCalls);
  return windowCalls.length > MUTATION_RATE_MAX_CALLS;
}

function normalizeSwapList(data: unknown): unknown[] {
  if (Array.isArray(data)) {
    return data;
  }
  if (data && typeof data === "object" && Array.isArray((data as { swaps?: unknown[] }).swaps)) {
    return (data as { swaps: unknown[] }).swaps;
  }
  return [];
}

function clearDeadSwapSubscribers(): void {
  for (const senderId of swapWatchSubscribers) {
    const target = webContents.fromId(senderId);
    if (!target || target.isDestroyed()) {
      swapWatchSubscribers.delete(senderId);
    }
  }
}

async function publishSwapWatchSnapshot(): Promise<void> {
  clearDeadSwapSubscribers();
  if (swapWatchSubscribers.size === 0) {
    return;
  }

  const traceId = randomUUID();
  try {
    const raw = await callApiBridge<unknown>("/api/v1/swaps", traceId);
    const swaps = normalizeSwapList(raw);
    const serialized = JSON.stringify(swaps);
    if (serialized === lastSwapWatchPayload) {
      return;
    }
    lastSwapWatchPayload = serialized;
    for (const senderId of swapWatchSubscribers) {
      const target = webContents.fromId(senderId);
      if (!target || target.isDestroyed()) {
        continue;
      }
      target.send(IPC_EVENTS.swapWatchAllUpdate, {
        traceId,
        swaps
      });
    }
  } catch {
    // Keep watcher alive even if backend is temporarily unavailable.
  }
}

function ensureSwapWatchTimer(): void {
  if (swapWatchTimer) {
    return;
  }
  void publishSwapWatchSnapshot();
  swapWatchTimer = setInterval(() => {
    void publishSwapWatchSnapshot();
  }, SWAP_WATCH_POLL_INTERVAL_MS);
}

function stopSwapWatchTimerIfIdle(): void {
  clearDeadSwapSubscribers();
  if (swapWatchSubscribers.size > 0) {
    return;
  }
  if (swapWatchTimer) {
    clearInterval(swapWatchTimer);
    swapWatchTimer = null;
  }
  lastSwapWatchPayload = "";
}

function setSwapWatchSubscription(senderId: number, subscribed: boolean): void {
  if (subscribed) {
    swapWatchSubscribers.add(senderId);
    ensureSwapWatchTimer();
    return;
  }
  swapWatchSubscribers.delete(senderId);
  stopSwapWatchTimerIfIdle();
}

async function callApiBridge<T>(path: string, traceId: string, init?: RequestInit): Promise<T> {
  const url = `${DEV_API_URL}${path}`;
  const headers = new Headers(init?.headers ?? {});
  headers.set("x-trace-id", traceId);
  const authToken = sessionToken ?? DEV_API_AUTH_BEARER;
  if (authToken) {
    headers.set("authorization", `Bearer ${authToken}`);
  }

  const response = await fetch(url, {
    ...init,
    headers,
    signal: init?.signal ?? AbortSignal.timeout(120_000)
  });

  if (!response.ok) {
    const raw = await response.text();
    if (response.status === 401) {
      vaultUnlocked = false;
      sessionToken = null;
      throw errUnauthenticated("session expired", {
        path,
        status: response.status,
        body: raw.slice(0, 400)
      });
    }
    throw errInvalidArgument(`api-bridge request failed: ${response.status}`, {
      path,
      status: response.status,
      body: raw.slice(0, 400)
    });
  }

  return response.json() as Promise<T>;
}

async function invokeChannel(definition: IpcChannelDefinition, _event: IpcMainInvokeEvent, payload: unknown): Promise<IpcResponse<unknown>> {
  const traceId = randomUUID();
  const startedAtMs = Date.now();

  try {
    const valid = definition.validate(payload);
    if (!valid.ok) {
      throw errInvalidArgument(valid.reason, { channel: definition.name });
    }

    if (definition.mode === "mutate" && checkMutationRateLimit(definition.name)) {
      throw errResourceExhausted("rate limit exceeded", {
        channel: definition.name,
        windowMs: MUTATION_RATE_WINDOW_MS,
        maxCalls: MUTATION_RATE_MAX_CALLS
      });
    }

    if (definition.requiresUnlocked && !vaultUnlocked) {
      throw errUnauthenticated("vault is locked", { channel: definition.name });
    }

    const data = await definition.handler(payload, {
      traceId,
      isVaultUnlocked: () => vaultUnlocked,
      setVaultUnlocked: (value: boolean) => {
        vaultUnlocked = value;
      },
      getSessionToken: () => sessionToken,
      setSessionToken: (value: string | null) => {
        sessionToken = value;
      },
      callApiBridge: <T>(path: string, init?: RequestInit) => callApiBridge<T>(path, traceId, init),
      setSwapWatchSubscription,
      senderId: _event.sender.id
    });

    logIpcResult(definition.name, traceId, startedAtMs, "ok");
    return { ok: true, data, traceId };
  } catch (error) {
    const normalized = normalizeIpcError(error, traceId);
    logIpcResult(definition.name, traceId, startedAtMs, "err", normalized.error.code);
    return normalized;
  }
}

function registerIpc(): void {
  for (const definition of IPC_CHANNEL_REGISTRY) {
    ipcMain.handle(definition.name, async (event, payload) => {
      return invokeChannel(definition, event, payload);
    });
  }
}

app.whenReady().then(() => {
  installCspHeader();
  registerIpc();
  createWindow();

  app.on("activate", () => {
    if (BrowserWindow.getAllWindows().length === 0) {
      createWindow();
    }
  });
});

app.on("window-all-closed", () => {
  swapWatchSubscribers.clear();
  stopSwapWatchTimerIfIdle();
  if (process.platform !== "darwin") {
    app.quit();
  }
});
