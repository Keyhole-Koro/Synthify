import { env, type LogLevel } from '@/config/env';
import { recordBrowserLog } from '@/lib/newrelic/browser';

// Structured first-party logger. Every call is delivered to New Relic Logs
// (independent of console level) and optionally mirrored to the console.
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
const noop = () => {};
const rawConsole = {
  debug: typeof console !== 'undefined' ? console.debug.bind(console) : noop,
  info: typeof console !== 'undefined' ? console.info.bind(console) : noop,
  warn: typeof console !== 'undefined' ? console.warn.bind(console) : noop,
  error: typeof console !== 'undefined' ? console.error.bind(console) : noop,
} satisfies Record<LogLevel, (...args: unknown[]) => void>;

const RANK: Record<LogLevel, number> = { debug: 10, info: 20, warn: 30, error: 40 };
const consoleMin = RANK[env.observability.consoleLevel];

type Attributes = Record<string, unknown>;

function errorAttributes(error: unknown): Attributes {
  if (error === undefined || error === null) return {};
  if (error instanceof Error) {
    return { errorName: error.name, errorMessage: error.message, errorStack: error.stack };
  }
  return { errorValue: String(error) };
}

function emit(level: LogLevel, message: string, attributes?: Attributes, error?: unknown) {
  // NR delivery is unconditional (and a no-op when the agent is disabled).
  recordBrowserLog(level, message, { ...attributes, ...errorAttributes(error) });

  if (RANK[level] < consoleMin) return;
  const args: unknown[] = [message];
  if (attributes && Object.keys(attributes).length > 0) args.push(attributes);
  if (error !== undefined) args.push(error);
  rawConsole[level](...args);
}

export const log = {
  debug: (message: string, attributes?: Attributes) => emit('debug', message, attributes),
  info: (message: string, attributes?: Attributes) => emit('info', message, attributes),
  warn: (message: string, attributes?: Attributes) => emit('warn', message, attributes),
  // Ergonomic overloads: log.error('msg', err) or log.error('msg', { source }, err).
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
