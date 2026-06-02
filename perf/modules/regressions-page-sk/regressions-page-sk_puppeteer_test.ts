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

  describe('Sheriff actions', () => {
    it('toggles triaged view when the button is clicked', async () => {
      // Select a sheriff to enable the button.
      await regressionsPageSkPO.selectSheriff('Sheriff Config 2');

      // Initial state should be "Show Triaged".
      let buttonText = await regressionsPageSkPO.triagedButton.innerText;
      expect(buttonText).to.equal('Show Triaged');

      // Click the button.
      await regressionsPageSkPO.triagedButton.click();

      // After clicking, the text should change to "Hide Triaged".
      buttonText = await regressionsPageSkPO.triagedButton.innerText;
      expect(buttonText).to.equal('Hide Triaged');
    });

    it('toggles improvements view when the button is clicked', async () => {
      // Select a sheriff to enable the button.
      await regressionsPageSkPO.selectSheriff('Sheriff Config 2');

      // Initial state should be "Show Improvements".
      let buttonText = await regressionsPageSkPO.improvementsButton.innerText;
      expect(buttonText).to.equal('Show Improvements');

      // Click the button.
      await regressionsPageSkPO.improvementsButton.click();

      // After clicking, the text should change to "Hide Improvements".
      buttonText = await regressionsPageSkPO.improvementsButton.innerText;
      expect(buttonText).to.equal('Hide Improvements');
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
      expect(await (await deltaCell!.getProperty('innerText')).jsonValue()).to.contain('+23.62%');
    });

    it('sheriff config 3: displays different anomalies table and clicks Show More', async () => {
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

      // Wait for the "Show More" button to become visible by checking that its
      // parent div no longer has the 'hidden' attribute.
      await testBed.page
        .waitForFunction(
          () => {
            const showMoreDiv = document.querySelector('#showmore');
            return showMoreDiv && !showMoreDiv.hasAttribute('hidden');
          },
          { timeout: 5000 }
        )
        .catch(() => {
          throw new Error('Timed out waiting for the "Show More" button to become visible.');
        });

      await regressionsPageSkPO.showMoreButton.click();

      // Wait for two rows to be present.
      await testBed.page.waitForFunction(
        () => document.querySelectorAll('anomalies-table-sk tbody[id^="rows-"] tr').length === 2
      );

      // Verify that one of the regression cells contains the expected text.
      const deltaCells = await anomaliesTable!.$$('tbody[id^="rows-"] tr td.regression');
      expect(deltaCells).to.have.length.of.at.least(1);
      const texts = await Promise.all(
        deltaCells.map((cell) => cell.evaluate((el) => el.textContent))
      );
      const hasRegression = texts.some((text) => text!.includes('+23.62%'));
      expect(hasRegression).to.be.true;
    });
  });
});
