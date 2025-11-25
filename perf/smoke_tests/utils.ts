import { Page, Browser } from 'puppeteer';
import { launchBrowser, launchBrowserForCloudtopDebug } from '../../puppeteer-tests/util';

// If no URL is provided, execute tests against a local instance with
// auth proxy enabled.
// TODO(mordeckimarcin) Look into integrating it with run_perfserver script.
export const PERF_BASE_URL = process.env.PERF_BASE_URL || 'http://localhost:8003';

export function GetSubscriptionBaseUrl() {
  return PERF_BASE_URL + '/a?selectedSubscription=';
}

export type PageTest = (page: Page) => void;

// Change to true if you want to run live browser.
// TODO(mordeckimarcin) make configurable with bazel.
const DEBUG_VIA_CRD = process.env.DEBUG_VIA_CRD || false;
// An arbitrary delay that postpones test execution
// after starting a non-headless browser instance.
// Can be set to one's liking.
const CRD_BROWSER_STARTUP_DELAY = 10000;

export async function browserForSmokeTest(): Promise<Browser> {
  if (DEBUG_VIA_CRD) {
    const browser = launchBrowserForCloudtopDebug();
    await new Promise((r) => setTimeout(r, CRD_BROWSER_STARTUP_DELAY));
    return browser;
  }
  return launchBrowser();
}

export async function applyPageDefaults(page: Page, perfBaseUrl: string): Promise<Page> {
  page.on('console', (msg) => console.log(`PAGE LOG[${msg.type()}]: ${msg.text()}`));
  page.on('pageerror', (message) => console.log('PAGE ERROR: ', message));
  page.on('response', (response) =>
    console.log(`RESPONSE LOG[${response.status()}]: ${response.url()}`)
  );
  page.on('requestfailed', (request) =>
    console.log(`REQUEST FAILED: [${request.url()}] ${request.failure()?.errorText}`)
  );

  // Tell demo pages this is a Puppeteer test. Demo pages should not fake RPC
  // latency, render animations or exhibit any other non-deterministic
  // behavior that could result in differences in the screenshots uploaded to
  // Gold.
  await page.setCookie({
    url: perfBaseUrl,
    name: 'puppeteer',
    value: 'true',
  });

  return page;
}
