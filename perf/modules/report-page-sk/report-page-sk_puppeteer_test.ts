import { assert, expect } from 'chai';
import { loadCachedTestBed, takeScreenshot, TestBed } from '../../../puppeteer-tests/util';
import { ReportPageSkPO } from './report-page-sk_po';

describe('report-page-sk', () => {
  let testBed: TestBed;
  let reportPageSkPO: ReportPageSkPO;

  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.setRequestInterception(true);

    testBed.page.on('request', (request) => {
      request.continue();
    });

    await testBed.page.goto(testBed.baseUrl);
    const reportPageSk = await testBed.page.$('report-page-sk');
    if (!reportPageSk) {
      throw new Error('report-page-sk not found');
    }
    reportPageSkPO = new ReportPageSkPO(reportPageSk);
    await testBed.page.setViewport({ width: 1200, height: 800 });
    await testBed.page.waitForSelector('#loading-spinner', { hidden: true });
  });

  afterEach(async () => {
    testBed.page.removeAllListeners('request');
    await testBed.page.setRequestInterception(false);
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('report-page-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'perf', 'report-page-sk');
    });

    it('loads anomalies and creates a graph', async () => {
      const anomaliesTablePO = reportPageSkPO.anomaliesTable;
      const rowCount = await anomaliesTablePO.getRowCount();
      //TODO(b/479903517) Verify the rows.
      expect(rowCount).to.equal(4);

      // The selected anomalies should be checked by default because of selected_keys.
      const graphs = await reportPageSkPO.graphs;
      expect(await graphs.length).to.equal(1);
      // Wait for graphs to fully render
      await new Promise((resolve) => setTimeout(resolve, 2000));
      await takeScreenshot(testBed.page, 'perf', 'report-page-sk');
    });

    it('loads commits container', async () => {
      const commitsDiv = await reportPageSkPO.commonCommitsDiv;
      const commitLinks = await reportPageSkPO.commonCommitLinks;
      assert.isNotNull(commitsDiv);
      // the cid response has one commit slice.
      expect(await commitLinks.length).to.equal(2);
      const link: string = (await (await commitLinks.item(0)).getAttribute('href'))!;
      // Must match with the first commitSlice in `perf/modules/common/test-util.ts`
      expect(link).to.equal(
        `https://skia.googlesource.com/skia/+show/0d7087e5b99087f5945f04dbda7b7a7a4b12e344`
      );
    });
  });

  describe('graph interactions', () => {
    it('synchronizes x-axis toggle across graphs', async () => {
      const graphs = await reportPageSkPO.graphs;
      // there's 1 selected anomaly, therefore one graph will be loaded initially
      // See `GROUP_REPORT_RESPONSE.selected_keys` in `perf/modules/common/test-util.ts`
      expect(await graphs.length).to.equal(1);

      const graph1PO = await reportPageSkPO.getGraph(0);

      // Initial state should be 'commit'
      expect(await graph1PO.getXAxisDomain()).to.equal('commit');
    });
  });

  // TODO(b/479903517): There is no need to have '#open-trending-icon' to open
  // anomalies table and intract with it. The removed tests will be replaced
  // with new tests.

  it('should be able to scroll up and down', async () => {
    // Scroll down by 1000px.
    await testBed.page.evaluate(() => window.scrollBy(0, 1000));
    const scrollYAfterScrollDown = await testBed.page.evaluate(() => window.scrollY);
    expect(scrollYAfterScrollDown).to.be.greaterThan(0);

    // Scroll up by 1000px.
    await testBed.page.evaluate(() => window.scrollBy(0, -1000));
    const scrollYAfterScrollUp = await testBed.page.evaluate(() => window.scrollY);
    expect(scrollYAfterScrollUp).to.equal(0);
  });
});
