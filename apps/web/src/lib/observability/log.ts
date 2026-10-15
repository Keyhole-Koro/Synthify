import type { LogLevel } from '@/config/env';
import { recordBrowserLog } from '@/lib/newrelic/browser';

// Structured first-party logger. Every call is delivered to New Relic Logs
// and warn/error calls are mirrored to the console.
//
// Two-path design with the wrapLogger backstop in newrelic/browser.ts:
//   - log.* (here): NR delivery via recordBrowserLog with queryable
//     customAttributes, plus console output through the ORIGINAL console refs
//     captured below.
//   - wrapLogger: wraps the live console.error/warn to catch everything else
//     (third-party output, raw console.* not yet migrated).
// Because we write to the captured originals — not the wrapped console — a
// log.error() is sent to NR exactly once (via recordBrowserLog), never also
// re-captured by wrapLogger.
const rawConsole = {
  debug: console.debug.bind(console),
  info: console.info.bind(console),
  warn: console.warn.bind(console),
  error: console.error.bind(console),
} satisfies Record<LogLevel, (...args: unknown[]) => void>;

type Attributes = Record<string, unknown>;

/**
 * Extracts standard error fields (name, message, stack) into an attributes dictionary.
 *
 * @param error - Caught error or exception.
 * @returns Flattened error attribute dictionary.
 */
function errorAttributes(error: unknown): Attributes {
  if (error === undefined || error === null) return {};
  if (error instanceof Error) {
    return { errorName: error.name, errorMessage: error.message, errorStack: error.stack };
  }
  return { errorValue: String(error) };
}

/**
 * Dispatches a log entry to New Relic Logs unconditionally and mirrors warnings and errors
 * to the browser console.
 *
 * @param level - Log severity level.
 * @param message - Main message string.
 * @param attributes - Optional key-value metadata.
 * @param error - Optional error object.
 */
function emit(level: LogLevel, message: string, attributes?: Attributes, error?: unknown) {
  // NR delivery is unconditional (and a no-op when the agent is disabled).
  recordBrowserLog(level, message, { ...attributes, ...errorAttributes(error) });

  if (level === 'debug' || level === 'info') return;
  const args: unknown[] = [message];
  if (attributes && Object.keys(attributes).length > 0) args.push(attributes);
  if (error !== undefined) args.push(error);
  rawConsole[level](...args);
}

/**
 * First-party structured client logger delivering telemetry to New Relic Logs
 * while echoing warnings and errors to the browser console.
 */
export const log = {
  /** Logs a debug message with optional metadata. */
  debug: (message: string, attributes?: Attributes) => emit('debug', message, attributes),
  /** Logs an informational message with optional metadata. */
  info: (message: string, attributes?: Attributes) => emit('info', message, attributes),
  /** Logs a warning message with optional metadata. */
  warn: (message: string, attributes?: Attributes) => emit('warn', message, attributes),
  /**
   * Logs an error message with error details and optional contextual metadata.
   *
   * @param message - Descriptive error message.
   * @param attributesOrError - Error instance or metadata dictionary.
   * @param maybeError - Error instance when attributes dictionary is passed as second argument.
   */
  error: (message: string, attributesOrError?: Attributes | unknown, maybeError?: unknown) => {
    if (attributesOrError instanceof Error) {
      emit('error', message, undefined, attributesOrError);
    } else if (attributesOrError && typeof attributesOrError === 'object') {
      emit('error', message, attributesOrError as Attributes, maybeError);
    } else {
      emit('error', message, undefined, attributesOrError ?? maybeError);
    }
  },
};
