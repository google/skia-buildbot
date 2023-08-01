import { assert } from 'chai';
import {
  loadCachedTestBed,
  takeScreenshot,
  TestBed,
} from '../../../puppeteer-tests/util';

describe('explore-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    testBed.page.on('dialog', async (dialog) => {
      await dialog.accept();
    });
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport({ width: 1600, height: 1600 });
    await testBed.page.click('#close_query_dialog');
  });

  it('should render the demo page', async () => {
    // Smoke test.
    assert.equal(await (await testBed.page.$$('explore-sk')).length, 1);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'perf', 'explore-sk');
    });
    it('shows the query dialog', async () => {
      await testBed.page.click('#demo-show-query-dialog');
      await testBed.page.waitForSelector('#query-dialog', {
        visible: true,
      });
      await takeScreenshot(testBed.page, 'perf', 'explore-sk_query_dialog');
    });
    it('loads the traces', async () => {
      await testBed.page.click('#demo-load-traces');
      await testBed.page.waitForSelector('#traceButtons', {
        visible: true,
      });
      await takeScreenshot(testBed.page, 'perf', 'explore-sk_traces_loaded');
    });

    it('displays bisect button if window.perf.fetch_chrome_perf_anomalies is true', async () => {
      await testBed.page.click('#demo-show-bisect-button');
      await testBed.page.waitForSelector('#traceButtons', {
        visible: true,
      });
      await takeScreenshot(
        testBed.page,
        'perf',
        'explore-sk_display_bisect_button'
      );
    });

    it('highlights a trace when the plot is clicked on', async () => {
      await testBed.page.click('#demo-select-trace');
      await testBed.page.waitForSelector('#controls', {
        visible: true,
      });
      await takeScreenshot(testBed.page, 'perf', 'explore-sk_trace_selected');
    });

    it('displays a subset of data when a calculated trace is clicked on', async () => {
      await testBed.page.click('#demo-select-calc-trace');
      await testBed.page.waitForSelector('#details', {
        visible: true,
      });
      await takeScreenshot(
        testBed.page,
        'perf',
        'explore-sk_trace_calc_selected'
      );
    });

    it('loads shows the help dialog on a keypress of ?', async () => {
      await testBed.page.click('#demo-show-help');
      await testBed.page.waitForSelector('#help', {
        visible: true,
      });
      await takeScreenshot(testBed.page, 'perf', 'explore-sk_help_dialog');
    });
  });
});
