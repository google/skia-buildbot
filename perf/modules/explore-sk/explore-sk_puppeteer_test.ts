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
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport({ width: 1200, height: 1200 });
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
  });
});