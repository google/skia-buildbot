import { expect } from 'chai';
import { loadCachedTestBed, takeScreenshot, TestBed } from '../../../puppeteer-tests/util';
import { RegressionPageSkPO } from './regressions-page-sk_po';

describe('regressions-page-sk', () => {
  let testBed: TestBed;
  let regressionsPageSkPO: RegressionPageSkPO;

  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport({ width: 400, height: 550 });
    regressionsPageSkPO = new RegressionPageSkPO((await testBed.page.$('regressions-page-sk'))!);
  });

  it('should render the demo page (smoke test)', async () => {
    expect(await testBed.page.$$('regressions-page-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('displays the default view', async () => {
      await takeScreenshot(testBed.page, 'perf', 'regressions-page-sk');
    });

    it('displays empty table if no regressions for selected subscription', async () => {
      await regressionsPageSkPO.selectSheriff('Sheriff Config 3');
      await takeScreenshot(testBed.page, 'perf', 'regressions-page-sk-no-regs');
    });

    it('displays table if some regressions present for selected subscription', async () => {
      await regressionsPageSkPO.selectSheriff('Sheriff Config 2');
      await takeScreenshot(testBed.page, 'perf', 'regressions-page-sk-some-regs');
    });
  });

  describe('anomalies list', () => {
    it('sheriff config 1: All anomalies are triaged!', async () => {
      // https://screenshot.googleplex.com/3c9AgSN3MQKUtLo
      await regressionsPageSkPO.selectSheriff('Sheriff Config 1');
      const selector = 'anomalies-table-sk h1[id^="clear-msg-"]';
      await testBed.page.waitForSelector(selector, { visible: true });
      const clearMsg = await testBed.page.$(selector);
      expect(await (await clearMsg!.getProperty('innerText')).jsonValue()).to.equal(
        'All anomalies are triaged!'
      );
    });

    it('sheriff config 2: displays anomalies table', async () => {
      // https://screenshot.googleplex.com/BsmzmxFoWHkHDk4
      await regressionsPageSkPO.selectSheriff('Sheriff Config 2');

      const anomaliesTable = await testBed.page.$('anomalies-table-sk');
      expect(anomaliesTable).to.not.be.null;

      // Wait for the table row to appear, indicating data is loaded.
      await testBed.page.waitForSelector('anomalies-table-sk tbody[id^="rows-"] tr');

      // Verify "All anomalies are triaged!" is hidden
      const clearMsg = await anomaliesTable!.$('h1[id^="clear-msg-"]');
      expect(await clearMsg!.evaluate((el) => el.hasAttribute('hidden'))).to.be.true;

      // Verify grouping settings exist
      const groupingSettings = await anomaliesTable!.$('anomalies-grouping-settings-sk');
      expect(groupingSettings).to.not.be.null;

      // Verify specific row content
      const deltaCell = await anomaliesTable!.$('tbody[id^="rows-"] tr td.regression');
      expect(deltaCell).to.not.be.null;
      expect(await (await deltaCell!.getProperty('innerText')).jsonValue()).to.contain('+23.6228%');
    });

    it('sheriff config 3: displays different anomalies table', async () => {
      // https://screenshot.googleplex.com/BHgQtN7VpFSndmu
      await regressionsPageSkPO.selectSheriff('Sheriff Config 3');

      const anomaliesTable = await testBed.page.$('anomalies-table-sk');
      expect(anomaliesTable).to.not.be.null;

      // Wait for the table row to appear, indicating data is loaded.
      await testBed.page.waitForSelector('anomalies-table-sk tbody[id^="rows-"] tr');

      const revisionsCell = await anomaliesTable!.$('tbody[id^="rows-"] tr td:nth-child(5)');
      expect(revisionsCell).to.not.be.null;
      expect(await (await revisionsCell!.getProperty('innerText')).jsonValue()).to.contain(
        '71321 - 71325'
      );
    });
  });
});
