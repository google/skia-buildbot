import { Page } from 'playwright-core';
import { PlaywrightTestBed } from '../../../puppeteer-tests/playwright_util';

export type MockPersona = 'chrome' | 'v8';

export interface RecordedMutation<T = any> {
  method: string;
  path: string;
  body: T;
}

/**
 * Client for controlling the stateful Perf mock server (POST /_/test/reset,
 * GET /_/test/recorded) from Playwright tests.
 */
export class MockServerClient {
  private readonly testBedOrUrl: PlaywrightTestBed | string;

  private clientRecorded: Map<string, RecordedMutation[]> = new Map();

  private attachedPages: WeakSet<Page> = new WeakSet();

  constructor(testBedOrUrl: PlaywrightTestBed | string) {
    this.testBedOrUrl = testBedOrUrl;
    this.ensurePageListener();
  }

  private ensurePageListener(): void {
    if (typeof this.testBedOrUrl === 'string' || !this.testBedOrUrl.page) {
      return;
    }
    const page = this.testBedOrUrl.page;
    if (this.attachedPages.has(page)) {
      return;
    }
    this.attachedPages.add(page);
    page.on('request', (req) => {
      if (req.method() !== 'POST') {
        return;
      }
      try {
        const parsedUrl = new URL(req.url());
        const path = parsedUrl.pathname;
        const body = req.postDataJSON();
        const list = this.clientRecorded.get(path) || [];
        list.push({ method: 'POST', path, body });
        this.clientRecorded.set(path, list);
      } catch {
        // Ignore non-JSON or malformed request URLs.
      }
    });
  }

  public get baseUrl(): string {
    return typeof this.testBedOrUrl === 'string' ? this.testBedOrUrl : this.testBedOrUrl.baseUrl;
  }

  /**
   * Resets all mutable mock server state (anomalies, shortcuts, frames, and
   * recorded mutations) and switches the active window.perf persona.
   */
  public async reset(persona: MockPersona = 'chrome'): Promise<void> {
    this.clientRecorded.clear();
    this.ensurePageListener();
    const resp = await fetch(`${this.baseUrl}/_/test/reset`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ persona }),
    });
    if (!resp.ok) {
      throw new Error(`POST /_/test/reset failed with status ${resp.status}`);
    }
  }

  /**
   * Returns the ordered list of decoded JSON request bodies recorded by the
   * mock server for the given endpoint path (e.g. "/_/triage/edit_anomalies").
   */
  public async recorded<T = any>(path: string): Promise<T[]> {
    const url = `${this.baseUrl}/_/test/recorded?path=${encodeURIComponent(path)}`;
    const resp = await fetch(url);
    if (!resp.ok) {
      throw new Error(`GET ${url} failed with status ${resp.status}`);
    }
    const entries = (await resp.json()) as RecordedMutation<T>[];
    if (entries.length > 0) {
      return entries.map((e) => e.body);
    }
    const fallback = (this.clientRecorded.get(path) || []) as RecordedMutation<T>[];
    return fallback.map((e) => e.body);
  }

  /**
   * Returns the full map of recorded mutations keyed by endpoint path.
   */
  public async allRecorded(): Promise<Record<string, RecordedMutation[]>> {
    const resp = await fetch(`${this.baseUrl}/_/test/recorded`);
    if (!resp.ok) {
      throw new Error(`GET /_/test/recorded failed with status ${resp.status}`);
    }
    const result = (await resp.json()) as Record<string, RecordedMutation[]>;
    this.clientRecorded.forEach((entries, path) => {
      if (!result[path] || result[path].length === 0) {
        result[path] = entries;
      }
    });
    return result;
  }

  /**
   * Pins localStorage.v2_ui and perf:use-explore-v2 on the Playwright page
   * before navigation.
   */
  public async pinExploreV2(pageOrEnabled: Page | boolean, maybeEnabled?: boolean): Promise<void> {
    const page =
      typeof pageOrEnabled === 'boolean'
        ? (this.testBedOrUrl as PlaywrightTestBed).page
        : pageOrEnabled;
    const enabled = typeof pageOrEnabled === 'boolean' ? pageOrEnabled : !!maybeEnabled;
    const val = enabled ? 'true' : 'false';
    await page.addInitScript((v: string) => {
      if (window.sessionStorage.getItem('__pinned_explore_v2') !== '1') {
        window.sessionStorage.setItem('__pinned_explore_v2', '1');
        window.localStorage.setItem('v2_ui', v);
        window.localStorage.setItem('perf:use-explore-v2', v);
      }
    }, val);
  }
}
