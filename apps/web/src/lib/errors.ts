export type AppErrorKind =
  | 'auth'         // Authentication/Authorization (401, 403, Firebase Auth)
  | 'not_found'    // Resource not found (404)
  | 'validation'   // Invalid input (400)
  | 'network'      // Network failure, timeout
  | 'unavailable'  // Server overloaded or maintenance (503)
  | 'server'       // Internal server error (500)
  | 'unknown';

export interface AppError extends Error {
  kind: AppErrorKind;
  retryable: boolean;
  code?: string;      // Machine-readable code (e.g., 'auth/user-not-found' or 'ALREADY_EXISTS')
  status?: number;    // HTTP Status code if applicable
}

export function isAppError(err: unknown): err is AppError {
  return (
    err instanceof Error &&
    'kind' in err &&
    typeof (err as { kind?: unknown }).kind === 'string' &&
    typeof (err as { retryable?: unknown }).retryable === 'boolean'
  );
}

export function createAppError(params: {
  kind: AppErrorKind;
  message: string;
  retryable?: boolean;
  code?: string;
  status?: number;
  cause?: unknown;
}): AppError {
  const err = new Error(params.message) as AppError;
  err.name = 'AppError';
  err.kind = params.kind;
  err.retryable = params.retryable ?? false;
  err.code = params.code;
  err.status = params.status;
  if (params.cause) {
    err.cause = params.cause;
  }
  return err;
}
