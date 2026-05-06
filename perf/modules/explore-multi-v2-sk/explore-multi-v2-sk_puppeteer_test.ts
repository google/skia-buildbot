import { expect } from 'chai';
import { loadCachedTestBed, TestBed } from '../../../puppeteer-tests/util';
import { poll } from '../common/puppeteer-test-util';
import { ExploreMultiV2SkPO } from './explore-multi-v2-sk_po';
import { ElementHandle } from 'puppeteer';

describe('explore-multi-v2-sk', () => {
  let testBed: TestBed;
  let exploreMultiV2SkPO: ExploreMultiV2SkPO;

  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    const page = testBed.page;
    page.on('console', (msg) => console.log('PAGE LOG:', msg.text()));

    await page.evaluateOnNewDocument(() => {
      (window as any).WORKER_URL =
        'data:application/javascript,self.postMessage({ type: "LOADED" }); self.onmessage = (e) => { if (e.data.type === "INIT") { self.postMessage({ type: "READY" }); } else if (e.data.type === "SUGGEST") { self.postMessage({ type: "SUGGEST_RESULT", idx: e.data.idx, payload: [{ params: [{ key: "test", value: "Score" }], count: 42, score: 100 }] }); } };';

      // Trap fetchMock when it is assigned to window
      let fm: any;
      Object.defineProperty(window, 'fetchMock', {
        get() {
          return fm;
        },
        set(v) {
          fm = v;
          console.log('MOCK LOG: fetchMock trapped!');
          // Allow falling back to network or original fetch
          if (fm.config) {
            fm.config.fallbackToNetwork = true;
          }
          // Register wasm mocks immediately
          fm.get(
            'glob:*/_/wasm/meta.json*',
            { version: 'test-version' },
            { overwriteRoutes: true }
          );
          fm.get('glob:*/_/wasm/params.json*', {}, { overwriteRoutes: true });
          fm.get('glob:*/dist/explore-multi-v2-sk/filter.wasm*', new ArrayBuffer(0), {
            overwriteRoutes: true,
          });
          fm.get(
            'glob:*/_/initpage*',
            {
              dataframe: {
                paramset: { arch: ['arm', 'x86'], os: ['windows', 'linux'] },
              },
            },
            { overwriteRoutes: true }
          );
          fm.get(
            'glob:*/_/defaults*',
            {
              default_range: 12960000,
              include_params: [],
            },
            { overwriteRoutes: true }
          );
        },
        configurable: true,
      });
    });

    await page.goto(testBed.baseUrl);
    const exploreMultiV2Sk = (await page.waitForSelector(
      'explore-multi-v2-sk'
    )) as ElementHandle<HTMLElement>;
    exploreMultiV2SkPO = new ExploreMultiV2SkPO(exploreMultiV2Sk);
  });

  it('should display the correct static content', async () => {
    const staticContent = await exploreMultiV2SkPO.staticContent;

    expect(staticContent).to.not.be.null;
    expect(staticContent!.title).to.equal('Explore Multi V2');
    expect(staticContent!.subtitle).to.equal(
      'High-performance custom dimension analysis (Work in Progress)'
    );
    expect(staticContent!.facetedSearchBarTitle).to.equal('Faceted Search Bar');
    expect(staticContent!.visualizationsTitle).to.equal('Visualizations');
  });

  it('should trigger fetch when panning in Date Mode', async () => {
    const page = testBed.page;

    // Mock /_/trace_values
    await page.evaluate(() => {
      (window as any).fetchMock.post(
        '/_/trace_values',
        { results: { 'test-trace': [{ x: 1000, y: 10 }] } },
        { overwriteRoutes: true }
      );
    });

    // Toggle Date Mode
    await page.evaluate(() => {
      const explore = document.querySelector('explore-multi-v2-sk');
      const exploreEl = explore as any;
      exploreEl._matchingTraceIds = ['t1', 't2'];
      exploreEl._pageSize = 10;
      exploreEl._tracePage = 0;
      exploreEl._dateMode = true;
      exploreEl._globalBounds = {};
      exploreEl._loadedBounds = {};
      exploreEl._viewportMinX = null;
      exploreEl._viewportMaxX = null;
    });

    // Simulate panning by calling _handleViewportChanged directly
    await page.evaluate(() => {
      const explore = document.querySelector('explore-multi-v2-sk') as any;
      if (explore) {
        explore._handleViewportChanged({
          detail: { minCommit: 500, maxCommit: 1500 },
        });
      }
    });

    // Wait for fetch to be called
    await page.waitForFunction(() => {
      const calls = (window as any).fetchMock.calls('/_/trace_values');
      return calls.length > 0;
    });

    // Verify request body
    const calls = await page.evaluate(() => {
      return (window as any).fetchMock.calls('/_/trace_values').map((c: any) => c[1].body);
    });

    expect(calls.length).to.be.greaterThan(0);
    const body = JSON.parse(calls[0]);
    expect(body.begin).to.be.a('number');
    expect(body.end).to.be.a('number');
    expect(body.begin).to.equal(500);
    expect(body.end).to.equal(1500);
  });

  it('should update suggestion counts when typing', async () => {
    const page = testBed.page;

    // Set availableParams on query-bar-sk
    await page.evaluate(() => {
      const explore = document.querySelector('explore-multi-v2-sk') as any;
      const queryBar = explore.shadowRoot.querySelector('query-bar-sk') as any;
      queryBar.availableParams = [{ id: 1, key: 'test', value: 'Score', count: 0 }];
      queryBar.query = {};
    });

    // Type in the query bar and trigger input event
    await page.evaluate(async () => {
      const explore = document.querySelector('explore-multi-v2-sk') as any;
      const queryBar = explore.shadowRoot.querySelector('query-bar-sk') as any;
      const input = queryBar.shadowRoot.querySelector('md-outlined-text-field');
      input.value = 'Sc';
      input.dispatchEvent(new Event('input'));
      await queryBar.updateComplete;
    });

    // Wait for the count to be updated to (42)
    await poll(
      async () => (await exploreMultiV2SkPO.getSuggestionCountText()) === '(42)',
      'Suggestion count did not update to (42)'
    );

    const countText = await exploreMultiV2SkPO.getSuggestionCountText();

    expect(countText).to.equal('(42)');
  });

  it('should load worker and become ready', async () => {
    const workerReady = await exploreMultiV2SkPO.isWorkerReady();

    expect(workerReady).to.be.true;
  });

  it('should set diffBase when Diff button is clicked', async () => {
    const page = testBed.page;

    // Set availableParams and query to make options appear
    await page.evaluate(() => {
      const explore = document.querySelector('explore-multi-v2-sk') as any;
      const queryBar = explore.shadowRoot.querySelector('query-bar-sk') as any;
      queryBar.availableParams = [{ key: 'test', value: 'Score', count: 1 }];
      queryBar.optionsByKey = { test: [{ value: 'Score', count: 1 }] };
      queryBar.query = { test: ['Score'] }; // So it appears as a pill
    });

    await exploreMultiV2SkPO.clickDiffButtonOnFirstQueryBarPill();

    // Verify _diffBase is set
    const diffBase = await exploreMultiV2SkPO.getDiffBase();

    expect(diffBase).to.deep.equal({ key: 'test', value: 'Score' });
  });

  it('should display Diff Base chip when diffBase is set', async () => {
    const page = testBed.page;

    // Set diffBase directly
    await page.evaluate(() => {
      const explore = document.querySelector('explore-multi-v2-sk') as any;
      explore._diffBase = { key: 'test', value: 'Score' };
      explore.requestUpdate();
    });

    exploreMultiV2SkPO.waitForDiffBaseChip();

    const chipText = await page.evaluate(() => {
      const explore = document.querySelector('explore-multi-v2-sk');
      const chip = explore?.shadowRoot?.querySelector('.config-pill');
      return chip ? chip.textContent : '';
    });

    expect(chipText).to.include('Diff Base:');
    expect(chipText).to.include('Score');
  });

  it('should add a new query bar when the add button is clicked', async () => {
    const page = testBed.page;

    // Initially, there should be one query bar.
    let queryBarCount = await exploreMultiV2SkPO.getQueryBarCount();
    expect(queryBarCount).to.equal(1);

    // Find and click the add button.
    await page.evaluate(() => {
      const explore = document.querySelector('explore-multi-v2-sk');
      const addButton = explore?.shadowRoot?.querySelector('.add-query-circle-btn') as HTMLElement;
      addButton?.click();
    });

    // Wait for the new query bar to appear.
    await poll(
      async () => (await exploreMultiV2SkPO.getQueryBarCount()) === 2,
      'Query bar count did not become 2'
    );

    queryBarCount = await exploreMultiV2SkPO.getQueryBarCount();
    expect(queryBarCount).to.equal(2);
  });

  it('should load queries from a shortcut in the URL', async () => {
    const page = testBed.page;

    // Mock /_/shortcut/get to return our graph configs
    await page.evaluate(() => {
      (window as any).fetchMock.post(
        '/_/shortcut/get',
        {
          graphs: [{ queries: ['test=A&stat=value'], formulas: [], keys: '' }],
        },
        { overwriteRoutes: true }
      );
    });

    // Load the shortcut directly instead of full page navigation
    await page.evaluate(async () => {
      const explore = document.querySelector('explore-multi-v2-sk') as any;
      await explore._loadShortcut('test-shortcut-id');
    });

    // Wait for the query-bar-sk element to be updated with the loaded queries
    await page.waitForFunction(() => {
      const explore = document.querySelector('explore-multi-v2-sk') as any;
      const queryBar = explore?.shadowRoot?.querySelector('query-bar-sk') as any;
      return queryBar && queryBar.query && Object.keys(queryBar.query).length > 0;
    });

    const query = await page.evaluate(() => {
      const explore = document.querySelector('explore-multi-v2-sk') as any;
      const queryBar = explore.shadowRoot.querySelector('query-bar-sk') as any;
      return queryBar.query;
    });

    expect(query).to.deep.equal({ test: ['A'], stat: ['value'] });
  });

  it('should update URL with default begin and end on load if not present', async () => {
    const page = testBed.page;

    // Poll until begin parameter populates on page load
    await poll(async () => {
      const urlStr = await page.evaluate(() => window.location.href);
      const url = new URL(urlStr);
      return url.searchParams.get('begin') !== null;
    }, 'Deterministic URL begin parameter did not populate on page load');

    const urlStr = await page.evaluate(() => window.location.href);
    const url = new URL(urlStr);
    expect(url.searchParams.get('begin')).to.not.be.null;
    expect(url.searchParams.get('end')).to.not.be.null;
    expect(Number(url.searchParams.get('begin'))).to.be.greaterThan(0);
    expect(Number(url.searchParams.get('end'))).to.be.greaterThan(0);
  });

  it('resolves partial bounds deterministically in URL on load', async () => {
    const page = testBed.page;

    // Navigate with only begin (relative past to demo date anchor 1585699200)
    await page.goto(`${testBed.baseUrl}?begin=1570000000`);
    await page.waitForSelector('explore-multi-v2-sk');

    // Poll until end parameter resolves deterministically
    await poll(async () => {
      const urlStr = await page.evaluate(() => window.location.href);
      const url = new URL(urlStr);
      return url.searchParams.get('end') !== null;
    }, 'Deterministic URL end parameter did not resolve on page load');

    const urlStr = await page.evaluate(() => window.location.href);
    const url = new URL(urlStr);
    expect(url.searchParams.get('begin')).to.equal('1570000000');

    const beginVal = Number(url.searchParams.get('begin'));
    const endVal = Number(url.searchParams.get('end'));
    // Expect end bound to match resolved defaults (150 days or backend defaults)
    expect(endVal - beginVal).to.be.within(7000000, 13000000);
  });

  it('should update URL begin and end when viewport is changed in Date Mode', async () => {
    const page = testBed.page;

    // Mock /_/trace_values and toggle Date Mode with viewport change
    await page.evaluate(() => {
      (window as any).fetchMock.post(
        '/_/trace_values',
        { results: { 'test-trace': [{ x: 1700000000, y: 10 }] } },
        { overwriteRoutes: true }
      );
      const explore = document.querySelector('explore-multi-v2-sk') as any;
      explore._matchingTraceIds = ['test-trace'];
      explore._pageSize = 10;
      explore._tracePage = 0;
      explore._dateMode = true;
      explore._handleViewportChanged({
        detail: { minCommit: 1700000000, maxCommit: 1700086400 },
      });
    });

    // Wait a bit for state reflection
    await new Promise((resolve) => setTimeout(resolve, 100));

    const urlStr = await page.evaluate(() => window.location.href);
    const url = new URL(urlStr);
    expect(url.searchParams.get('begin')).to.equal('1700000000');
    expect(url.searchParams.get('end')).to.equal('1700086400');

    // Poll to verify panned database fetch was successfully triggered
    await poll(async () => {
      const calls = await page.evaluate(() => (window as any).fetchMock.calls('/_/trace_values'));
      return calls.length > 0;
    }, 'Data-fetching query to /_/trace_values did not fire');
  });

  it('should translate commit numbers to timestamps in Commit Mode panning', async () => {
    const page = testBed.page;

    // Mock /_/trace_values, set mock series data, disable Date Mode, and trigger viewport change
    await page.evaluate(() => {
      (window as any).fetchMock.post(
        '/_/trace_values',
        { results: { 'test-trace': [{ x: 100, y: 10 }] } },
        { overwriteRoutes: true }
      );
      const explore = document.querySelector('explore-multi-v2-sk') as any;
      explore._matchingTraceIds = ['test-trace'];
      explore._pageSize = 10;
      explore._tracePage = 0;
      explore._dateMode = false;
      explore._seriesData = [
        {
          id: 'test-trace',
          rows: [
            { commit_number: 100, createdat: 1710000000 },
            { commit_number: 200, createdat: 1720000000 },
          ],
        },
      ];
      explore._handleViewportChanged({
        detail: { minCommit: 100, maxCommit: 200 },
      });
    });

    // Wait for state reflection
    await new Promise((resolve) => setTimeout(resolve, 100));

    const urlStr = await page.evaluate(() => window.location.href);
    const url = new URL(urlStr);
    expect(url.searchParams.get('begin')).to.equal('1710000000');
    expect(url.searchParams.get('end')).to.equal('1720000000');

    // Poll to verify panned database fetch was successfully triggered
    await poll(async () => {
      const calls = await page.evaluate(() => (window as any).fetchMock.calls('/_/trace_values'));
      return calls.length > 0;
    }, 'Data-fetching query to /_/trace_values did not fire');
  });

  it('should reset begin and end URL params when zoom is reset', async () => {
    const page = testBed.page;

    // Set explicit begin/end, then trigger reset zoom
    await page.evaluate(() => {
      const explore = document.querySelector('explore-multi-v2-sk') as any;
      explore._begin = 1680000000;
      explore._end = 1680100000;
      explore._onResetZoom();
    });

    // Wait for state reflection
    await new Promise((resolve) => setTimeout(resolve, 100));

    const urlStr = await page.evaluate(() => window.location.href);
    const url = new URL(urlStr);
    expect(url.searchParams.get('begin')).to.satisfy(
      (val: string | null) => val === null || val === '-1'
    );
    expect(url.searchParams.get('end')).to.satisfy(
      (val: string | null) => val === null || val === '-1'
    );
  });
});
