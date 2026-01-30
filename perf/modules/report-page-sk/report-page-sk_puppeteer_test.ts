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
    await takeScreenshot(testBed.page, 'perf', 'report-page-sk');
    // Smoke test.
    expect(await testBed.page.$$('report-page-sk')).to.have.length(1);
  });

  describe('anomalies list', () => {
    it('loads anomalies and creates a graph', async () => {
      const anomaliesTablePO = reportPageSkPO.anomaliesTable;
      const rowCount = await anomaliesTablePO.getRowCount();
      expect(rowCount).to.equal(4);
      const expectedRows: any[] = [
        {
          bugId: 'Bug ID',
          revisions: 'Revisions ',
          bot: 'Bot',
          testSuite: 'Test Suite ',
          test: 'Test',
          delta: 'Delta %',
        },
        {
          bugId: '',
          revisions: '67129 - 67130',
          bot: 'mac-m1_mini_2020-perf',
          testSuite: 'jetstream2',
          test: 'Babylon.First',
          delta: '+23.6228%',
        },
        {
          separator: 'Other groups, related to requested ones (with overlapping commits range)',
        },
        {
          bugId: '',
          revisions: '67129 - 67130',
          bot: '',
          testSuite: '',
          test: '',
          delta: '+35.1741%',
        },
      ];
      const rows = await anomaliesTablePO.rows;
      for (let i = 0; i < (await rows.length); i++) {
        const row = await rows.item(i);
        const expected = expectedRows[i];
        if (expected.separator) {
          expect(await row.innerText).to.contain(expected.separator);
        } else {
          const cells = await row.bySelectorAll('td, th');
          expect(await (await cells.item(3)).innerText).to.equal(expected.bugId);
          expect(await (await cells.item(4)).innerText).to.equal(expected.revisions);
          expect(await (await cells.item(5)).innerText).to.equal(expected.bot);
          expect(await (await cells.item(6)).innerText).to.equal(expected.testSuite);
          expect(await (await cells.item(7)).innerText).to.equal(expected.test);
          expect(await (await cells.item(8)).innerText).to.equal(expected.delta);
        }
      }

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
      // The cid response has one commit slice.
      // Must match with the first commitSlice in `perf/modules/common/test-util.ts`
      expect(await commitLinks.length).to.equal(2);
      expect(await (await commitLinks.item(0)).innerText).to.equal('0d7087e');
      const link1: string = (await (await commitLinks.item(0)).getAttribute('href'))!;
      expect(link1).to.equal(
        `https://skia.googlesource.com/skia/+show/0d7087e5b99087f5945f04dbda7b7a7a4b12e344`
      );
      expect(await (await commitLinks.item(1)).innerText).to.equal('2894e71');
      const link2: string = (await (await commitLinks.item(1)).getAttribute('href'))!;
      expect(link2).to.equal(
        `https://skia.googlesource.com/skia/+show/2894e7194406ad8014d3e85b39379ca0e4607ead`
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
