export interface IpcErrorBody {
  code: string;
  message: string;
  details?: unknown;
  traceId: string;
}

export interface IpcOkResponse<T> {
  ok: true;
  data: T;
  traceId: string;
}

export interface IpcErrResponse {
  ok: false;
  error: IpcErrorBody;
}

export type IpcResponse<T> = IpcOkResponse<T> | IpcErrResponse;

export class IpcAppError extends Error {
  code: string;
  details?: unknown;

  constructor(code: string, message: string, details?: unknown) {
    super(message);
    this.name = "IpcAppError";
    this.code = code;
    this.details = details;
  }
}

export function errInvalidArgument(message: string, details?: unknown): IpcAppError {
  return new IpcAppError("INVALID_ARGUMENT", message, details);
}

export function errUnauthenticated(message: string, details?: unknown): IpcAppError {
  return new IpcAppError("UNAUTHENTICATED", message, details);
}

export function errUnimplemented(message: string, details?: unknown): IpcAppError {
  return new IpcAppError("UNIMPLEMENTED", message, details);
}

export function errResourceExhausted(message: string, details?: unknown): IpcAppError {
  return new IpcAppError("RESOURCE_EXHAUSTED", message, details);
}

export function normalizeIpcError(error: unknown, traceId: string): IpcErrResponse {
  if (error instanceof IpcAppError) {
    return {
      ok: false,
      error: {
        code: error.code,
        message: error.message,
        details: error.details,
        traceId
      }
    };
  }

  const message = error instanceof Error ? error.message : "Unknown IPC failure";
  return {
    ok: false,
    error: {
      code: "INTERNAL",
      message,
      traceId
    }
  };
}
