type BrowserAgent = {
  noticeError(error: Error | string, customAttributes?: object): unknown;
  addPageAction(name: string, attributes?: object): unknown;
  setUserId(value: string | null, resetSession?: boolean): unknown;
};

declare global {
  interface Window {
    __synthifyNewRelicBrowserAgent?: BrowserAgent;
  }
}

const beacon = process.env.NEXT_PUBLIC_NEW_RELIC_BROWSER_BEACON || 'bam.nr-data.net';
const errorBeacon = process.env.NEXT_PUBLIC_NEW_RELIC_BROWSER_ERROR_BEACON || beacon;
const licenseKey = process.env.NEXT_PUBLIC_NEW_RELIC_BROWSER_LICENSE_KEY;
const applicationID = process.env.NEXT_PUBLIC_NEW_RELIC_BROWSER_APPLICATION_ID;
const accountID = process.env.NEXT_PUBLIC_NEW_RELIC_BROWSER_ACCOUNT_ID;
const trustKey = process.env.NEXT_PUBLIC_NEW_RELIC_BROWSER_TRUST_KEY;
const agentID = process.env.NEXT_PUBLIC_NEW_RELIC_BROWSER_AGENT_ID || applicationID;

let pendingInit: Promise<BrowserAgent | null> | null = null;

export function initNewRelicBrowser(): Promise<BrowserAgent | null> {
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
  ]).then(([
    { Agent },
    { Ajax },
    { JSErrors },
    { Metrics },
    { PageViewEvent },
    { PageViewTiming },
    { SoftNav },
    { GenericEvents },
  ]) => {
    const loaderConfig = {
      accountID,
      trustKey,
      agentID,
      licenseKey,
      applicationID,
    };

    const agent = new Agent({
      info: {
        beacon,
        errorBeacon,
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
        logging: {
          enabled: false,
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
      ],
    });

    window.__synthifyNewRelicBrowserAgent = agent;
    return agent;
  }).catch((error) => {
    pendingInit = null;
    console.error('New Relic Browser initialization failed:', error);
    return null;
  });

  return pendingInit;
}

export function getNewRelicBrowserAgent(): BrowserAgent | null {
  if (typeof window === 'undefined') return null;
  return window.__synthifyNewRelicBrowserAgent ?? null;
}

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

// recordBrowserPageAction emits a custom PageAction event. Prefer this over
// noticeBrowserError for things that are worth observing but are not user-facing
// JS errors (so they stay out of the error rate / alerting feeds).
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
