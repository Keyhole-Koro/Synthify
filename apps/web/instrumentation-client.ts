import { initNewRelicBrowser } from './src/lib/newrelic/browser';

try {
  initNewRelicBrowser();
} catch (error) {
  console.error('New Relic Browser initialization failed:', error);
}
