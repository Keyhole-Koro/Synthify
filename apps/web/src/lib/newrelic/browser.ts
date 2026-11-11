import { env, type LogLevel } from '@/config/env';

type NrLogLevel = 'ERROR' | 'TRACE' | 'DEBUG' | 'INFO' | 'WARN';
type NrLogOptions = { customAttributes?: object; level?: NrLogLevel };

type BrowserAgent = {
  noticeError(error: Error | string, customAttributes?: object): unknown;
  addPageAction(name: string, attributes?: object): unknown;
  log(message: string, options?: NrLogOptions): unknown;
  wrapLogger(parent: object, functionName: string, options?: NrLogOptions): unknown;
  setUserId(value: string | null, resetSession?: boolean): unknown;
};

declare global {
  interface Window {
    __synthifyNewRelicBrowserAgent?: BrowserAgent;
  }
}

const nr = env.observability.newRelic;
const NR_LEVEL: Record<LogLevel, NrLogLevel> = {
  debug: 'DEBUG',
  info: 'INFO',
  warn: 'WARN',
  error: 'ERROR',
};

let pendingInit: Promise<BrowserAgent | null> | null = null;

/**
 * Dynamically loads and initializes the New Relic Browser agent if configured with a license key and app ID.
 *
 * Concurrent callers share the same in-flight initialization promise. When
 * browser execution or required credentials are unavailable, the promise
 * resolves to `null` and application behavior is left unchanged.
 *
 * @returns Promise resolving to the initialized BrowserAgent instance or null.
 */
export function initNewRelicBrowser(): Promise<BrowserAgent | null> {
  const { licenseKey, applicationID } = nr;
  // Narrow to string for the agent config below; also the real enablement gate.
  if (typeof window === 'undefined' || !licenseKey || !applicationID) return Promise.resolve(null);
  if (window.__synthifyNewRelicBrowserAgent) return Promise.resolve(window.__synthifyNewRelicBrowserAgent);
  if (pendingInit) return pendingInit;

  pendingInit = Promise.all([
    import('@newrelic/browser-agent/loaders/agent'),
    import('@newrelic/browser-agent/features/ajax'),
    import('@newrelic/browser-agent/features/jserrors'),
    import('@newrelic/browser-agent/features/metrics'),
    import('@newrelic/browser-agent/features/page_view_event'),
    import('@newrelic/browser-agent/features/page_view_timing'),
    import('@newrelic/browser-agent/features/soft_navigations'),
    import('@newrelic/browser-agent/features/generic_events'),
    import('@newrelic/browser-agent/features/logging'),
  ]).then(([
    { Agent },
    { Ajax },
    { JSErrors },
    { Metrics },
    { PageViewEvent },
    { PageViewTiming },
    { SoftNav },
    { GenericEvents },
    { Logging },
  ]) => {
    const loaderConfig = {
      accountID: nr.accountID,
      trustKey: nr.trustKey,
      agentID: nr.agentID,
      licenseKey,
      applicationID,
    };

    const agent = new Agent({
      info: {
        beacon: nr.beacon,
        errorBeacon: nr.errorBeacon,
        licenseKey,
        applicationID,
      },
      loader_config: Object.fromEntries(
        Object.entries(loaderConfig).filter(([, value]) => Boolean(value))
      ),
      init: {
        ajax: {
          enabled: true,
          block_internal: true,
        },
        distributed_tracing: {
          enabled: false,
        },
        // Powers addPageAction (recordBrowserPageAction). Used for non-error
        // operational signals we still want to observe in prod — e.g. swallowed
        // Firestore snapshot errors — without inflating the JS error rate.
        generic_events: {
          enabled: true,
        },
        jserrors: {
          enabled: true,
        },
        // Backs log()/wrapLogger so handled errors + warnings reach NR Logs.
        logging: {
          enabled: true,
        },
        metrics: {
          enabled: true,
        },
        page_action: {
          enabled: false,
        },
        page_view_timing: {
          enabled: true,
        },
        performance: {
          capture_marks: false,
          capture_measures: false,
          resources: {
            enabled: false,
          },
        },
        session_replay: {
          enabled: false,
        },
        session_trace: {
          enabled: false,
        },
        soft_navigations: {
          enabled: true,
        },
        user_actions: {
          enabled: false,
        },
        obfuscate: [
          {
            regex: /([?&](?:token|id_token|access_token|refresh_token|password|secret|key|code)=)[^&]*/gi,
            replacement: '$1[REDACTED]',
          },
        ],
      },
      features: [
        Ajax,
        JSErrors,
        Metrics,
        PageViewEvent,
        PageViewTiming,
        SoftNav,
        GenericEvents,
        Logging,
      ],
    });

    window.__synthifyNewRelicBrowserAgent = agent;

    // Catch-all backstop: mirror every console.error/warn — including
    // third-party output and anything not yet routed through log.ts — into NR
    // Logs. log.ts writes via the original console refs it captured before this
    // ran, so first-party structured logs are not double-counted here.
    if (env.observability.wrapConsole && typeof console !== 'undefined') {
      try {
        agent.wrapLogger(console, 'error', { level: 'ERROR' });
        agent.wrapLogger(console, 'warn', { level: 'WARN' });
      } catch {
        // wrapLogger is best-effort; never let it break agent init.
      }
    }
    return agent;
  }).catch((error) => {
    pendingInit = null;
    // Raw console: the agent failed to load, so log.ts/NR can't capture this.
    console.error('New Relic Browser initialization failed:', error);
    return null;
  });

  return pendingInit;
}

/**
 * Retrieves the globally cached New Relic browser agent instance if present.
 *
 * @returns The BrowserAgent instance or null if uninitialized.
 */
export function getNewRelicBrowserAgent(): BrowserAgent | null {
  if (typeof window === 'undefined') return null;
  return window.__synthifyNewRelicBrowserAgent ?? null;
}

/**
 * Records an error to New Relic Browser with optional custom attributes.
 *
 * @param error - The Error or message to notice.
 * @param customAttributes - Optional contextual metadata.
 * @returns Nothing; if the agent is not ready, delivery is queued behind
 *   asynchronous initialization on a best-effort basis.
 */
export function noticeBrowserError(error: Error | string, customAttributes?: Record<string, unknown>) {
  const agent = getNewRelicBrowserAgent();
  if (agent) {
    agent.noticeError(error, customAttributes);
    return;
  }
  void initNewRelicBrowser().then((initializedAgent) => {
    initializedAgent?.noticeError(error, customAttributes);
  });
}

/**
 * Emits a custom PageAction event to New Relic for non-error operational events.
 *
 * @param name - Action name.
 * @param attributes - Custom event attributes.
 * @returns Nothing; events are discarded when the agent cannot be initialized.
 */
export function recordBrowserPageAction(name: string, attributes?: Record<string, unknown>) {
  const agent = getNewRelicBrowserAgent();
  if (agent) {
    agent.addPageAction(name, attributes);
    return;
  }
  void initNewRelicBrowser().then((initializedAgent) => {
    initializedAgent?.addPageAction(name, attributes);
  });
}

/**
 * Sends a structured log record to New Relic Logs at the specified severity level.
 *
 * @param level - Log severity level.
 * @param message - Main message string.
 * @param attributes - Queryable key-value metadata.
 * @returns Nothing; delivery is best effort and does not block the caller.
 */
export function recordBrowserLog(
  level: LogLevel,
  message: string,
  attributes?: Record<string, unknown>,
) {
  const options: NrLogOptions = { level: NR_LEVEL[level], customAttributes: attributes };
  const agent = getNewRelicBrowserAgent();
  if (agent) {
    agent.log(message, options);
    return;
  }
  void initNewRelicBrowser().then((initializedAgent) => {
    initializedAgent?.log(message, options);
  });
}

/**
 * Sets the authenticated user ID on the New Relic Browser session for tracking user journeys.
 *
 * @param userId - Unique user identifier, or null on logout.
 * @returns Nothing; the update is sent immediately when the agent is ready or
 *   after asynchronous initialization otherwise.
 */
export function setBrowserUserId(userId: string | null) {
  const agent = getNewRelicBrowserAgent();
  if (agent) {
    agent.setUserId(userId);
    return;
  }
  void initNewRelicBrowser().then((initializedAgent) => {
    initializedAgent?.setUserId(userId);
  });
}
