import { expect } from 'chai';
import { loadCachedTestBed, TestBed } from '../../../puppeteer-tests/util';
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
    exploreMultiV2SkPO = new ExploreMultiV2SkPO(exploreMultiV2Sk);
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
      await explore.updateComplete;

      await customElements.whenDefined('query-bar-sk');

      const queryBar = explore.shadowRoot.querySelector('query-bar-sk') as any;
      if (!queryBar) throw new Error('query-bar-sk not found');

      explore.queries = [{ test: ['Score'] }];
      explore._availableParams = [{ key: 'test', value: 'Score', count: 1 }];
      explore._optionsByKey = { test: [{ value: 'Score', count: 1 }] };
      explore._optionsByKeyPerQuery = [{ test: [{ value: 'Score', count: 1 }] }];
      explore.requestUpdate();
      await explore.updateComplete;
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

      // Verify triage-menu-sk is present
      const hasTriageMenu = await page.evaluate(async () => {
        const explore = document.querySelector('explore-multi-v2-sk') as any;
        await explore.updateComplete;
        const traceChart = explore.shadowRoot.querySelector('trace-chart-sk') as any;
        await traceChart.updateComplete;
        const tooltip = traceChart.shadowRoot.querySelector('trace-chart-tooltip-sk');
        const triageMenu = tooltip
          ? tooltip.shadowRoot.querySelector('#tooltip-triage-menu')
          : null;
        return triageMenu !== null;
      });
      expect(hasTriageMenu).to.be.true;

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
});
