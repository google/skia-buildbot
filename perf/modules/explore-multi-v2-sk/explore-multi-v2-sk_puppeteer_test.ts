import { expect } from 'chai';
import { loadCachedTestBed, takeScreenshot, TestBed } from '../../../puppeteer-tests/util';
import { poll } from '../common/puppeteer-test-util';
import { ExploreMultiV2SkPO } from './explore-multi-v2-sk_po';
import { ElementHandle } from 'puppeteer';
import { ScreencastRecorder } from '../../../puppeteer-tests/screencast';

describe('explore-multi-v2-sk', () => {
  let testBed: TestBed;
  let exploreMultiV2SkPO: ExploreMultiV2SkPO;

  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    const page = testBed.page;
    page.on('console', (msg) => console.log('PAGE LOG:', msg.text()));
    await page.goto(testBed.baseUrl);
    const exploreMultiV2Sk = (await page.waitForSelector(
      'explore-multi-v2-sk'
    )) as ElementHandle<HTMLElement>;
    await page.evaluate((el) => {
      (el as any)._debounceDelay = 0;
    }, exploreMultiV2Sk);
    exploreMultiV2SkPO = new ExploreMultiV2SkPO(exploreMultiV2Sk);
  });

  afterEach(async () => {
    const page = testBed.page;
    page.removeAllListeners('console');
  });

  it('should display the correct static content', async () => {
    const staticContent = await exploreMultiV2SkPO.staticContent;

    expect(staticContent).to.not.be.null;
    expect(staticContent!.title).to.equal('Explore Multi V2');
    expect(staticContent!.subtitle!.trim()).to.equal(
      'High-performance custom dimension analysis (Work in Progress)'
    );
    expect(staticContent!.facetedSearchBarTitle).to.equal('Faceted Search Bar');
    expect(staticContent!.visualizationsTitle).to.equal('Visualizations');
  });

  it('should trigger fetch when panning in Date Mode', async () => {
    const page = testBed.page;

    // Wait for worker to be ready first to avoid asynchronous initialization races
    await page.waitForFunction(() => {
      const explore = document.querySelector('explore-multi-v2-sk') as any;
      const queryBar = explore?.shadowRoot?.querySelector('query-bar-sk') as any;
      return queryBar && queryBar.availableParams.length > 0;
    });

    // Clear cache to force network fetch
    await page.evaluate(async () => {
      return await new Promise((resolve) => {
        const req = indexedDB.open('TraceCache', 1);
        req.onsuccess = () => {
          const db = req.result;
          if (db.objectStoreNames.contains('frames')) {
            const tx = db.transaction(['frames'], 'readwrite');
            const store = tx.objectStore('frames');
            store.clear();
            tx.oncomplete = () => resolve(true);
          } else {
            resolve(true);
          }
        };
        req.onerror = () => resolve(false);
      });
    });

    // Wait for worker to be ready and initial filter to finish
    await page.waitForFunction(() => {
      const explore = document.querySelector('explore-multi-v2-sk') as any;
      return (
        explore &&
        explore._workerController &&
        explore._workerController.isReady() &&
        Object.keys(explore._optionsByKey).length > 0
      );
    });

    // Toggle Date Mode
    await page.evaluate(() => {
      const explore = document.querySelector('explore-multi-v2-sk');
      if (!explore) {
        throw new Error('explore-multi-v2-sk element not found in DOM!');
      }
      const exploreEl = explore as any;
      exploreEl._matchingTraceIds = ['t1', 't2'];
      exploreEl._seriesData = [];
      exploreEl._pageSize = 10;
      exploreEl._tracePage = 0;
      exploreEl.dateMode = true;
      exploreEl._globalBounds = {};
      exploreEl._loadedBounds = {};
      exploreEl.viewportMinX = null;
      exploreEl.viewportMaxX = null;
    });

    // Mock fetch for /_/trace_values
    await page.evaluate(() => {
      (window as any).fetchMock.post('/_/trace_values', { results: {} }, { overwriteRoutes: true });
    });

    // Simulate panning by calling _handleViewportChanged directly
    await page.evaluate(async () => {
      const explore = document.querySelector('explore-multi-v2-sk') as any;
      if (!explore) {
        throw new Error(
          'explore-multi-v2-sk element not found in DOM when calling _doHandleViewportChanged!'
        );
      }
      await explore._doHandleViewportChanged({
        detail: { minCommit: 500, maxCommit: 1500 },
      });
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

    // Wait for availableParams to be populated from worker
    await page.waitForFunction(() => {
      const explore = document.querySelector('explore-multi-v2-sk');
      const queryBar = explore?.shadowRoot?.querySelector('query-bar-sk') as any;
      return queryBar && queryBar.availableParams.length > 0;
    });

    // Type in the query bar and trigger input event
    await page.evaluate(async () => {
      const explore = document.querySelector('explore-multi-v2-sk') as any;
      const queryBar = explore.shadowRoot.querySelector('query-bar-sk') as any;
      const input = queryBar.shadowRoot.querySelector('md-outlined-text-field');
      input.value = 'An';
      input.dispatchEvent(new Event('input'));
      await queryBar.updateComplete;
    });

    // Wait for the count to be updated to (5) for Android
    await page.waitForFunction(
      () => {
        const explore = document.querySelector('explore-multi-v2-sk') as any;
        const queryBar = explore.shadowRoot.querySelector('query-bar-sk') as any;
        const countEl = queryBar.shadowRoot.querySelector('.s-count.right');
        const text = countEl ? countEl.textContent.trim() : 'null';
        return text === '(5)';
      },
      { timeout: 10000 }
    );

    const countText = await exploreMultiV2SkPO.getSuggestionCountText();

    expect(countText.trim()).to.equal('(5)');
  });

  it('should load worker and become ready', async () => {
    const workerReady = await exploreMultiV2SkPO.isWorkerReady();

    expect(workerReady).to.be.true;
  });

  it('should set diffBase when Diff button is clicked', async () => {
    const page = testBed.page;

    // Set availableParams and query on the parent to make options appear
    await page.evaluate(async () => {
      const explore = document.querySelector('explore-multi-v2-sk') as any;
      if (!explore) throw new Error('explore-multi-v2-sk not found');

      // Mock worker filter to prevent it from sending empty results
      if (explore._workerController) {
        explore._workerController.filter = (_queries: any, _num: number, requestId?: number) => {
          return requestId !== undefined ? requestId : 0;
        };
      }

      explore.queries = [{ test: ['Score'] }];
      explore._optionsByKeyPerQuery = [{ test: [{ value: 'Score', count: 1 }] }];
      await explore.updateComplete;

      await customElements.whenDefined('query-bar-sk');

      const queryBar = explore.shadowRoot.querySelector('query-bar-sk') as any;
      if (!queryBar) throw new Error('query-bar-sk not found');

      // Ensure shadowRoot is available
      if (!queryBar.shadowRoot) throw new Error('query-bar-sk shadowRoot is null');

      queryBar.availableParams = [{ key: 'test', value: 'Score', count: 1 }];
      await queryBar.updateComplete;
    });

    await exploreMultiV2SkPO.clickDiffButtonOnFirstQueryBarPill();

    // Verify _diffBase is set
    const diffBase = await exploreMultiV2SkPO.getDiffBase();

    expect(diffBase).to.deep.equal({ key: 'test', value: 'Score' });
  });

  it('should display Diff Base chip when diffBase is set', async () => {
    const page = testBed.page;

    // Set diffBase directly
    await page.evaluate(async () => {
      const explore = document.querySelector('explore-multi-v2-sk') as any;
      explore._diffBase = { key: 'test', value: 'Score' };
      explore.requestUpdate();
      await explore.updateComplete;
    });

    await page.waitForFunction(
      () => {
        const explore = document.querySelector('explore-multi-v2-sk');
        const chip = explore?.shadowRoot?.querySelector('.config-pill.diff-base');
        const text = chip?.textContent || '';
        return text.includes('Diff Base:');
      },
      { timeout: 10000 }
    );

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

  it('should update state when formula pipeline step is added', async () => {
    const page = testBed.page;

    await page.evaluate(() => {
      const explore = document.querySelector('explore-multi-v2-sk') as any;
      const queryBar = explore?.shadowRoot?.querySelector('query-bar-sk') as any;
      const addBtn = queryBar?.shadowRoot?.querySelector(
        '.qb-add-formula-btn'
      ) as HTMLButtonElement;
      if (addBtn) {
        addBtn.click();
      }
    });

    await page.waitForFunction(() => {
      const explore = document.querySelector('explore-multi-v2-sk') as any;
      const queryBar = explore?.shadowRoot?.querySelector('query-bar-sk') as any;
      return queryBar?.shadowRoot?.querySelector('.qb-formula-item') !== null;
    });

    await page.evaluate(() => {
      const explore = document.querySelector('explore-multi-v2-sk') as any;
      const queryBar = explore?.shadowRoot?.querySelector('query-bar-sk') as any;
      const item = queryBar?.shadowRoot?.querySelector('.qb-formula-item') as HTMLButtonElement;
      if (item) {
        item.click();
      }
    });

    await page.waitForFunction(() => {
      const explore = document.querySelector('explore-multi-v2-sk') as any;
      return (
        explore &&
        explore._formulasPerQuery &&
        explore._formulasPerQuery[0] &&
        explore._formulasPerQuery[0].includes('fill')
      );
    });

    const formulaState = await page.evaluate(() => {
      const explore = document.querySelector('explore-multi-v2-sk') as any;
      return explore._formulasPerQuery[0];
    });

    expect(formulaState).to.deep.equal(['fill']);
  });

  it('should take screenshots of formula popover and formula pipeline steps', async () => {
    const page = testBed.page;

    // 1. Open formula popover
    await page.evaluate(() => {
      const explore = document.querySelector('explore-multi-v2-sk') as any;
      const queryBar = explore?.shadowRoot?.querySelector('query-bar-sk') as any;
      const addBtn = queryBar?.shadowRoot?.querySelector(
        '.qb-add-formula-btn'
      ) as HTMLButtonElement;
      if (addBtn) addBtn.click();
    });

    await page.waitForFunction(() => {
      const explore = document.querySelector('explore-multi-v2-sk') as any;
      const queryBar = explore?.shadowRoot?.querySelector('query-bar-sk') as any;
      return queryBar?.shadowRoot?.querySelector('.qb-formula-popover') !== null;
    });

    await takeScreenshot(testBed.page, 'perf', 'explore-multi-v2-sk_formula_popover');

    // 2. Click formula item to add fill() step 1
    await page.evaluate(() => {
      const explore = document.querySelector('explore-multi-v2-sk') as any;
      const queryBar = explore?.shadowRoot?.querySelector('query-bar-sk') as any;
      const items = queryBar?.shadowRoot?.querySelectorAll(
        '.qb-formula-item'
      ) as NodeListOf<HTMLButtonElement>;
      items[0]?.click(); // fill()
    });

    // 3. Open popover again and click ave() to add step 2
    await page.evaluate(() => {
      const explore = document.querySelector('explore-multi-v2-sk') as any;
      const queryBar = explore?.shadowRoot?.querySelector('query-bar-sk') as any;
      const addBtn = queryBar?.shadowRoot?.querySelector(
        '.qb-add-formula-btn'
      ) as HTMLButtonElement;
      if (addBtn) addBtn.click();
    });

    await page.waitForFunction(() => {
      const explore = document.querySelector('explore-multi-v2-sk') as any;
      const queryBar = explore?.shadowRoot?.querySelector('query-bar-sk') as any;
      return queryBar?.shadowRoot?.querySelector('.qb-formula-popover') !== null;
    });

    await page.evaluate(() => {
      const explore = document.querySelector('explore-multi-v2-sk') as any;
      const queryBar = explore?.shadowRoot?.querySelector('query-bar-sk') as any;
      const items = Array.from(
        queryBar?.shadowRoot?.querySelectorAll('.qb-formula-item') || []
      ) as HTMLButtonElement[];
      const aveBtn = items.find((btn) => btn.textContent?.includes('ave'));
      if (aveBtn) aveBtn.click();
    });

    // Verify chained formula pipeline in state
    await page.waitForFunction(() => {
      const explore = document.querySelector('explore-multi-v2-sk') as any;
      return (
        explore &&
        explore._formulasPerQuery &&
        explore._formulasPerQuery[0] &&
        explore._formulasPerQuery[0].length === 2
      );
    });

    const pipelineState = await page.evaluate(() => {
      const explore = document.querySelector('explore-multi-v2-sk') as any;
      return explore._formulasPerQuery[0];
    });

    expect(pipelineState).to.deep.equal(['fill', 'ave']);

    await takeScreenshot(testBed.page, 'perf', 'explore-multi-v2-sk_formula_pipeline');
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

    // Wait for worker to be ready and initial filter to finish
    await page.waitForFunction(() => {
      const explore = document.querySelector('explore-multi-v2-sk') as any;
      return (
        explore &&
        explore._workerController &&
        explore._workerController.isReady() &&
        Object.keys(explore._optionsByKey).length > 0
      );
    });

    // Mock /_/trace_values and toggle Date Mode with viewport change
    await page.evaluate(() => {
      (window as any).fetchMock.post(
        '/_/trace_values',
        {
          results: { ',arch=arm,config=8888,os=Android,project=Skia,': [{ x: 1700000000, y: 10 }] },
        },
        { overwriteRoutes: true }
      );
      const explore = document.querySelector('explore-multi-v2-sk') as any;
      explore._matchingTraceIds = [',arch=arm,config=8888,os=Android,project=Skia,'];
      explore._pageSize = 10;
      explore._tracePage = 0;
      explore.dateMode = true;
      explore._loadedBounds = {};
      explore._globalBounds = {};
      explore._viewportMinX = null;
      explore._viewportMaxX = null;
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

    // Wait for worker to be ready and initial filter to finish
    await page.waitForFunction(() => {
      const explore = document.querySelector('explore-multi-v2-sk') as any;
      return (
        explore &&
        explore._workerController &&
        explore._workerController.isReady() &&
        Object.keys(explore._optionsByKey).length > 0
      );
    });

    // Mock /_/trace_values, set mock series data, disable Date Mode, and trigger viewport change
    await page.evaluate(() => {
      (window as any).fetchMock.post(
        '/_/trace_values',
        { results: { ',arch=arm,config=8888,os=Android,project=Skia,': [{ x: 100, y: 10 }] } },
        { overwriteRoutes: true }
      );
      const explore = document.querySelector('explore-multi-v2-sk') as any;
      explore._matchingTraceIds = [',arch=arm,config=8888,os=Android,project=Skia,'];
      explore._pageSize = 10;
      explore._tracePage = 0;
      explore._dateMode = false;
      explore._loadedBounds = {};
      explore._globalBounds = {};
      explore._viewportMinX = null;
      explore._viewportMaxX = null;
      explore._seriesData = [
        {
          id: 'dummy-id-for-translation',
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
      explore.begin = 1680000000;
      explore.end = 1680100000;
      explore._onResetZoom();
      explore._resolveTimeRange();
    });

    // Wait for state reflection
    await page.waitForFunction(() => {
      const url = new URL(window.location.href);
      return (
        url.searchParams.get('begin') === '1572739200' &&
        url.searchParams.get('end') === '1585699200'
      );
    });

    const urlStr = await page.evaluate(() => window.location.href);
    const url = new URL(urlStr);
    expect(url.searchParams.get('begin')).to.equal('1572739200');
    expect(url.searchParams.get('end')).to.equal('1585699200');
  });

  it('should display tooltip when searching and hovering', async function () {
    this.timeout(120000);
    const page = testBed.page;

    // Wait for worker to be ready and initial filter to finish
    await page.waitForFunction(() => {
      const explore = document.querySelector('explore-multi-v2-sk') as any;
      return (
        explore &&
        explore._workerController &&
        explore._workerController.isReady() &&
        Object.keys(explore._optionsByKey).length > 0
      );
    });

    await page.setViewport({ width: 1200, height: 1000 });

    // Mock network requests using Puppeteer's request interception
    await page.setRequestInterception(true);
    const requestHandler = (request: any) => {
      (async () => {
        const url = request.url();
        if (url.endsWith('/_/frame/start')) {
          await request.respond({
            status: 200,
            contentType: 'application/json',
            body: JSON.stringify({
              dataframe: {
                traceset: {
                  ',arch=arm,config=8888,os=Ubuntu,project=Skia,': [10, 20, 30, 40, 50],
                },
                header: [
                  {
                    offset: 100,
                    timestamp: 1234567890,
                    hash: 'abc100',
                    author: 'alice',
                    message: 'commit 100',
                    url: 'http://git',
                  },
                  {
                    offset: 101,
                    timestamp: 1234567891,
                    hash: 'abc101',
                    author: 'bob',
                    message: 'commit 101',
                    url: 'http://git',
                  },
                  {
                    offset: 102,
                    timestamp: 1234567892,
                    hash: 'abc102',
                    author: 'charlie',
                    message: 'commit 102',
                    url: 'http://git',
                  },
                  {
                    offset: 103,
                    timestamp: 1234567893,
                    hash: 'abc103',
                    author: 'david',
                    message: 'commit 103',
                    url: 'http://git',
                  },
                  {
                    offset: 104,
                    timestamp: 1234567894,
                    hash: 'abc104',
                    author: 'eve',
                    message: 'commit 104',
                    url: 'http://git',
                  },
                ],
                paramset: { arch: ['arm'], config: ['8888'], os: ['Ubuntu'], project: ['Skia'] },
                skip: 0,
              },
              msg: '',
              display_mode: 'display_plot',
              anomalymap: {},
            }),
          });
        } else if (url.endsWith('/_/links_batch')) {
          await request.respond({
            status: 200,
            contentType: 'application/json',
            body: JSON.stringify({}),
          });
        } else if (url.endsWith('/_/login/status')) {
          await request.respond({
            status: 200,
            contentType: 'application/json',
            body: JSON.stringify({ email: 'test@google.com' }),
          });
        } else {
          await request.continue();
        }
      })().catch(console.error);
    };
    page.on('request', requestHandler);

    const recorder = new ScreencastRecorder('explore_multi_v2_tooltip');
    await recorder.start(page);

    try {
      // Inject styles to fix white-on-white visibility issue in test environment
      await page.evaluate(() => {
        (window as any).perf = (window as any).perf || {};
        (window as any).perf.bug_host_url = 'https://example.bug.url';
        const style = document.createElement('style');
        style.textContent = `
          :root {
            --md-sys-color-surface: #3c4043 !important;
            --on-surface: white !important;
          }
          .query-bar-container {
            background-color: #3c4043 !important;
            color: white !important;
          }
        `;
        document.head.appendChild(style);
      });

      // Set query directly to trigger search
      await page.evaluate(async () => {
        const explore = document.querySelector('explore-multi-v2-sk') as any;
        explore.queries = [{ arch: ['arm'] }];
        explore.requestUpdate();
        await explore._fetchData();
      });

      // Take a screenshot after setting query
      let screenshotBuffer = await page.screenshot();
      console.log('SCREENSHOT_AFTER_SET_QUERY_BASE64:', screenshotBuffer.toString('base64'));

      // Wait for trace-chart-sk to be rendered and filled with data
      await page.waitForFunction(
        () => {
          const explore = document.querySelector('explore-multi-v2-sk');
          const chart = explore?.shadowRoot?.querySelector('trace-chart-sk') as any;
          return (
            chart !== null &&
            chart._processedSeries &&
            chart._processedSeries.length > 0 &&
            chart._processedSeries[0].rows &&
            chart._processedSeries[0].rows.length > 0
          );
        },
        { timeout: 120000 }
      );

      // Set regressions AFTER chart is rendered to avoid being cleared by _fetchData
      await page.evaluate(() => {
        const explore = document.querySelector('explore-multi-v2-sk') as any;
        const chart = explore.shadowRoot.querySelector('trace-chart-sk') as any;
        const series = chart._processedSeries[0];
        const row = series.rows[0];
        explore._regressions = {
          [series.id]: {
            [row.commit_number]: {
              id: 'test-anomaly-id',
              bug_id: 12345,
              bugs: [],
              is_improvement: false,
              commit_number: row.commit_number,
              median_after: 20.0,
              median_before: 10.0,
            },
          },
        };
        explore._showRegressions = true;
        explore.requestUpdate();
      });

      // Take a screenshot after chart rendered and regressions set
      screenshotBuffer = await page.screenshot();
      console.log('SCREENSHOT_AFTER_CHART_RENDERED_BASE64:', screenshotBuffer.toString('base64'));

      // Mock hovered point to trigger tooltip
      const tooltipFound = await page.evaluate(() => {
        const explore = document.querySelector('explore-multi-v2-sk') as any;
        const chart = explore?.shadowRoot?.querySelector('trace-chart-sk') as any;
        if (chart && chart._processedSeries && chart._processedSeries.length > 0) {
          const series = chart._processedSeries[0];
          const row = series.rows[0];
          if (row) {
            chart._hoveredPoint = { series: series, row: row, x: 500, y: 200 };
            chart.requestUpdate();
            return true;
          }
        }
        return false;
      });

      expect(tooltipFound).to.be.true;

      // Verify Bisect button is present
      const hasBisectBtn = await page.evaluate(async () => {
        const explore = document.querySelector('explore-multi-v2-sk') as any;
        await explore.updateComplete;
        const traceChart = explore.shadowRoot.querySelector('trace-chart-sk') as any;
        await traceChart.updateComplete;
        const tooltip = traceChart.shadowRoot.querySelector('trace-chart-tooltip-sk');
        const bisectBtn = tooltip ? tooltip.shadowRoot.querySelector('#bisect') : null;
        return bisectBtn !== null;
      });
      expect(hasBisectBtn).to.be.true;

      // Verify triage-menu-sk is present and hidden because a bug is associated
      const triageMenuDetails = await page.evaluate(async () => {
        const explore = document.querySelector('explore-multi-v2-sk') as any;
        await explore.updateComplete;
        const traceChart = explore.shadowRoot.querySelector('trace-chart-sk') as any;
        await traceChart.updateComplete;
        const tooltip = traceChart.shadowRoot.querySelector('trace-chart-tooltip-sk');
        const triageMenu = tooltip
          ? tooltip.shadowRoot.querySelector('#tooltip-triage-menu')
          : null;
        if (!triageMenu) {
          return null;
        }
        return {
          present: true,
          hidden: triageMenu.hasAttribute('hidden'),
        };
      });
      expect(triageMenuDetails).to.not.be.null;
      expect(triageMenuDetails!.present).to.be.true;
      expect(triageMenuDetails!.hidden).to.be.true;

      // Verify Bug ID link is present
      const bugLinkDetails = await page.evaluate(async () => {
        const explore = document.querySelector('explore-multi-v2-sk') as any;
        await explore.updateComplete;
        const traceChart = explore.shadowRoot.querySelector('trace-chart-sk') as any;
        await traceChart.updateComplete;
        const tooltip = traceChart.shadowRoot.querySelector('trace-chart-tooltip-sk');
        if (!tooltip) {
          return null;
        }
        await tooltip.updateComplete;
        const link = tooltip.shadowRoot.querySelector('a[href="https://example.bug.url/12345"]');
        if (!link) {
          return null;
        }
        return {
          href: link.getAttribute('href'),
          text: link.textContent?.trim(),
        };
      });
      expect(bugLinkDetails).to.not.be.null;
      expect(bugLinkDetails!.text).to.equal('12345');

      const hasNewBugBtn = await page.evaluate(async () => {
        const explore = document.querySelector('explore-multi-v2-sk') as any;
        await explore.updateComplete;
        const traceChart = explore.shadowRoot.querySelector('trace-chart-sk') as any;
        await traceChart.updateComplete;
        const tooltip = traceChart.shadowRoot.querySelector('trace-chart-tooltip-sk');
        const triageMenu = tooltip
          ? (tooltip.shadowRoot.querySelector('#tooltip-triage-menu') as any)
          : null;
        if (!triageMenu) {
          return false;
        }
        await triageMenu.updateComplete;
        const buttons = triageMenu.getElementsByTagName('button');
        let found = false;
        for (let i = 0; i < buttons.length; i++) {
          if (buttons[i].id === 'new-bug') {
            found = true;
            break;
          }
        }
        return found;
      });
      expect(hasNewBugBtn).to.be.true;

      // Verify Commit Range link is present
      const hasCommitRangeLink = await page.evaluate(async () => {
        const explore = document.querySelector('explore-multi-v2-sk') as any;
        await explore.updateComplete;
        const traceChart = explore.shadowRoot.querySelector('trace-chart-sk') as any;
        await traceChart.updateComplete;
        const tooltip = traceChart.shadowRoot.querySelector('trace-chart-tooltip-sk');
        const link = tooltip
          ? tooltip.shadowRoot.querySelector('#tooltip-commit-range-link')
          : null;
        return link !== null;
      });
      expect(hasCommitRangeLink).to.be.true;

      // Take a screenshot of the tooltip
      const screenshot = await page.screenshot();
      console.log('SCREENSHOT_TOOLTIP_BASE64:', screenshot.toString('base64'));

      // Verify that hovering and clicking inside the tooltip does not close it
      // Wait for updates to complete to ensure tooltip is fully rendered
      await page.evaluate(async () => {
        const explore = document.querySelector('explore-multi-v2-sk') as any;
        await explore.updateComplete;
        const traceChart = explore.shadowRoot.querySelector('trace-chart-sk') as any;
        await traceChart.updateComplete;
      });

      const tooltipRect = await page.evaluate(() => {
        const explore = document.querySelector('explore-multi-v2-sk');
        const traceChart = explore?.shadowRoot?.querySelector('trace-chart-sk') as any;
        const tooltip = traceChart?.shadowRoot?.querySelector('trace-chart-tooltip-sk');
        const hoverTooltip = tooltip?.shadowRoot?.querySelector('.hover-tooltip');
        if (!hoverTooltip) return null;
        const r = hoverTooltip.getBoundingClientRect();
        return { x: r.left, y: r.top, width: r.width, height: r.height };
      });

      expect(tooltipRect, 'tooltipRect should not be null').to.not.be.null;
      expect(tooltipRect!.width, 'tooltip width should be > 0').to.be.greaterThan(0);
      expect(tooltipRect!.height, 'tooltip height should be > 0').to.be.greaterThan(0);

      // Move mouse to the center of the tooltip
      const tooltipCenterX = tooltipRect!.x + tooltipRect!.width / 2;
      const tooltipCenterY = tooltipRect!.y + tooltipRect!.height / 2;
      console.log(`Moving mouse to tooltip center: (${tooltipCenterX}, ${tooltipCenterY})`);
      await page.mouse.move(tooltipCenterX, tooltipCenterY);

      // Wait to ensure no debounced hover logic closes it
      await new Promise((resolve) => setTimeout(resolve, 300));

      // Check if tooltip moved
      const tooltipRectAfterMove = await page.evaluate(() => {
        const explore = document.querySelector('explore-multi-v2-sk');
        const traceChart = explore?.shadowRoot?.querySelector('trace-chart-sk') as any;
        const tooltip = traceChart?.shadowRoot?.querySelector('trace-chart-tooltip-sk');
        const hoverTooltip = tooltip?.shadowRoot?.querySelector('.hover-tooltip');
        if (!hoverTooltip) return null;
        const r = hoverTooltip.getBoundingClientRect();
        return { x: r.left, y: r.top, width: r.width, height: r.height };
      });
      console.log('Tooltip rect after move:', tooltipRectAfterMove);

      // Verify it remains open
      let isTooltipOpen = await page.evaluate(() => {
        const explore = document.querySelector('explore-multi-v2-sk');
        const traceChart = explore?.shadowRoot?.querySelector('trace-chart-sk') as any;
        const tooltip = traceChart?.shadowRoot?.querySelector('trace-chart-tooltip-sk');
        return tooltip !== null;
      });
      expect(isTooltipOpen, 'tooltip should remain open after moving mouse inside it').to.be.true;

      expect(tooltipRectAfterMove, 'tooltipRectAfterMove should not be null').to.not.be.null;

      // Click in a safe area of the tooltip (top-left, avoiding links/buttons)
      const safeClickX = tooltipRectAfterMove!.x + 15;
      const safeClickY = tooltipRectAfterMove!.y + 15;
      console.log(`Clicking safe area in tooltip: (${safeClickX}, ${safeClickY})`);
      await page.mouse.click(safeClickX, safeClickY);

      // Verify it remains open after click
      isTooltipOpen = await page.evaluate(() => {
        const explore = document.querySelector('explore-multi-v2-sk');
        const traceChart = explore?.shadowRoot?.querySelector('trace-chart-sk') as any;
        const tooltip = traceChart?.shadowRoot?.querySelector('trace-chart-tooltip-sk');
        return tooltip !== null;
      });
      expect(isTooltipOpen, 'tooltip should remain open after clicking inside it').to.be.true;

      // Click Bisect button
      await page.evaluate(() => {
        const explore = document.querySelector('explore-multi-v2-sk') as any;
        const traceChart = explore.shadowRoot.querySelector('trace-chart-sk') as any;
        const tooltip = traceChart.shadowRoot.querySelector('trace-chart-tooltip-sk');
        const bisectBtn = tooltip ? (tooltip.shadowRoot.querySelector('#bisect') as any) : null;
        if (bisectBtn) {
          bisectBtn.click();
        }
      });

      // Wait for Bisect dialog to be visible
      await page.waitForFunction(
        () => {
          const explore = document.querySelector('explore-multi-v2-sk') as any;
          if (!explore) return false;
          const traceChart = explore.shadowRoot.querySelector('trace-chart-sk') as any;
          if (!traceChart) return false;
          const tooltip = traceChart.shadowRoot.querySelector('trace-chart-tooltip-sk') as any;
          if (!tooltip) return false;
          const bisectDialogSk = tooltip.shadowRoot.querySelector('#bisect-dialog-sk') as any;
          if (!bisectDialogSk) return false;
          const dialog = bisectDialogSk.querySelector('#bisect-dialog');
          if (!dialog) return false;
          const dialogRect = dialog.getBoundingClientRect();
          return dialogRect.width > 0 && dialogRect.height > 0;
        },
        { timeout: 5000 }
      );

      // Take a screenshot of the dialog
      const screenshotDialog = await page.screenshot();
      console.log('SCREENSHOT_DIALOG_BASE64:', screenshotDialog.toString('base64'));
    } finally {
      await recorder.stop();
      page.off('request', requestHandler);
      await page.setRequestInterception(false);
    }
  });

  it('should render the floating help button and trigger the walkthrough tour', async () => {
    const page = testBed.page;
    await page.goto(testBed.baseUrl);
    await page.waitForSelector('explore-multi-v2-sk');

    // Wait for the custom elements to be defined and fully updated
    await page.evaluate(async () => {
      const explore = document.querySelector('explore-multi-v2-sk') as any;
      await explore.updateComplete;

      await customElements.whenDefined('help-hub-sk');
      const helpHub = explore.shadowRoot.querySelector('help-hub-sk') as any;
      await helpHub.updateComplete;

      await customElements.whenDefined('interactive-tour-sk');
      const tour = explore.shadowRoot.querySelector('interactive-tour-sk') as any;
      await tour.updateComplete;
    });

    // Click the FAB to open the Help Hub Panel
    await page.evaluate(async () => {
      const explore = document.querySelector('explore-multi-v2-sk') as any;
      const helpHub = explore.shadowRoot.querySelector('help-hub-sk') as any;
      const fab = helpHub.shadowRoot.querySelector('.help-fab') as HTMLElement;
      fab.click();
      await helpHub.updateComplete;
    });

    // Click 'Start Tour' trigger button
    await page.evaluate(async () => {
      const explore = document.querySelector('explore-multi-v2-sk') as any;
      const helpHub = explore.shadowRoot.querySelector('help-hub-sk') as any;
      const tourBtn = helpHub.shadowRoot.querySelector('.tour-trigger-btn') as HTMLElement;
      tourBtn.click();
      await explore.updateComplete;
    });

    // Verify Tour Overlay has spawned and first step title is correct
    const tourTitle = await page.evaluate(async () => {
      const explore = document.querySelector('explore-multi-v2-sk') as any;
      const tour = explore.shadowRoot.querySelector('interactive-tour-sk') as any;
      await tour.updateComplete;
      const titleEl = tour.shadowRoot.querySelector('.bubble-title');
      return titleEl ? titleEl.textContent.trim() : null;
    });

    expect(tourTitle).to.equal('Dynamic Setup');
  });

  it('should update anomaly position when nudging without refresh', async () => {
    const page = testBed.page;

    await page.evaluate(() => {
      const explore = document.querySelector('explore-multi-v2-sk') as any;
      explore._regressions = {
        'trace-a': { 100: { id: 'test-anomaly-id', commit_number: 100 } },
      };

      explore.dispatchEvent(
        new CustomEvent('anomaly-changed', {
          detail: {
            traceNames: ['trace-a'],
            anomalies: [{ id: 'test-anomaly-id', display_commit_number: 101 }],
          },
        })
      );
    });

    const regressions = await page.evaluate(() => {
      const explore = document.querySelector('explore-multi-v2-sk') as any;
      return explore._regressions['trace-a'] || {};
    });

    expect(regressions[101]).to.not.be.undefined;
    expect(regressions[100]).to.be.undefined;
  });

  it('should support arbitrary pill selection and copying via keyboard shortcuts', async () => {
    const page = testBed.page;

    // Set up query and options so pills render
    await page.evaluate(async () => {
      const explore = document.querySelector('explore-multi-v2-sk') as any;
      explore.queries = [{ benchmark: ['v8'], bot: ['MacM1'] }];
      explore._optionsByKeyPerQuery = [
        {
          benchmark: [{ value: 'v8', count: 1 }],
          bot: [{ value: 'MacM1', count: 1 }],
        },
      ];
      await explore.updateComplete;

      const queryBar = explore.shadowRoot.querySelector('query-bar-sk') as any;
      queryBar.availableParams = [
        { key: 'benchmark', value: 'v8', count: 1 },
        { key: 'bot', value: 'MacM1', count: 1 },
      ];
      await queryBar.updateComplete;
    });

    // Verify both pills exist and are NOT highlighted initially
    let pillHighlightStates = await page.evaluate(() => {
      const explore = document.querySelector('explore-multi-v2-sk') as any;
      const queryBar = explore.shadowRoot.querySelector('query-bar-sk') as any;
      const pills = queryBar.shadowRoot.querySelectorAll('explore-multi-v2-select-sk');
      return Array.from(pills).map((p: any) => p.isHighlighted);
    });
    expect(pillHighlightStates).to.deep.equal([false, false]);

    // Simulate Ctrl+Click on the first pill inside the page
    await page.evaluate(() => {
      const explore = document.querySelector('explore-multi-v2-sk') as any;
      const queryBar = explore.shadowRoot.querySelector('query-bar-sk') as any;
      const firstPill = queryBar.shadowRoot.querySelectorAll('explore-multi-v2-select-sk')[0];
      firstPill.dispatchEvent(new MouseEvent('click', { ctrlKey: true, bubbles: true }));
    });

    // Verify first pill is highlighted, second is not
    pillHighlightStates = await page.evaluate(() => {
      const explore = document.querySelector('explore-multi-v2-sk') as any;
      const queryBar = explore.shadowRoot.querySelector('query-bar-sk') as any;
      const pills = queryBar.shadowRoot.querySelectorAll('explore-multi-v2-select-sk');
      return Array.from(pills).map((p: any) => p.isHighlighted);
    });
    expect(pillHighlightStates).to.deep.equal([true, false]);

    // Verify dropdown is NOT open
    const isDropdownOpen = await page.evaluate(() => {
      const explore = document.querySelector('explore-multi-v2-sk') as any;
      const queryBar = explore.shadowRoot.querySelector('query-bar-sk') as any;
      return queryBar._openPillIndex !== null;
    });
    expect(isDropdownOpen).to.be.false;

    // Now simulate a normal click on the second pill
    await page.evaluate(() => {
      const explore = document.querySelector('explore-multi-v2-sk') as any;
      const queryBar = explore.shadowRoot.querySelector('query-bar-sk') as any;
      const secondPill = queryBar.shadowRoot.querySelectorAll('explore-multi-v2-select-sk')[1];
      secondPill.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });

    // Verify selection is cleared
    pillHighlightStates = await page.evaluate(() => {
      const explore = document.querySelector('explore-multi-v2-sk') as any;
      const queryBar = explore.shadowRoot.querySelector('query-bar-sk') as any;
      const pills = queryBar.shadowRoot.querySelectorAll('explore-multi-v2-select-sk');
      return Array.from(pills).map((p: any) => p.isHighlighted);
    });
    expect(pillHighlightStates).to.deep.equal([false, false]);
  });
});
