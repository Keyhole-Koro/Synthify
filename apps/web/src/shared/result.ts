/**
 * Lightweight Result type for representing expected domain operations and validation failures type-safely.
 *
 * - Domain errors (input validation, auth failure, quota exceeded): Return `Result<T, E>`.
 * - System errors (network disconnection, 500 fatal, unexpected bugs): Throw an Error exception.
 *
 * @typeParam T - Type of the successful value.
 * @typeParam E - Type of the error reason.
 */
export type Result<T, E> =
  | { ok: true; value: T }
  | { ok: false; error: E };

/**
 * Creates an `ok` Result containing the given value.
 *
 * @param value - The success value.
 * @returns A successful Result object.
 */
export function ok<T>(value: T): Result<T, never> {
  return { ok: true, value };
}

/**
 * Creates an `err` Result containing the given error payload.
 *
 * @param error - The error payload.
 * @returns A failed Result object.
 */
export function err<E>(error: E): Result<never, E> {
  return { ok: false, error };
}

/**
 * Type guard returning true if the Result is successful (`ok: true`).
 *
 * @param result - The Result to check.
 * @returns True if result is ok.
 */
export function isOk<T, E>(result: Result<T, E>): result is { ok: true; value: T } {
  return result.ok;
}

/**
 * Type guard returning true if the Result is a failure (`ok: false`).
 *
 * @param result - The Result to check.
 * @returns True if result is err.
 */
export function isErr<T, E>(result: Result<T, E>): result is { ok: false; error: E } {
  return !result.ok;
}
