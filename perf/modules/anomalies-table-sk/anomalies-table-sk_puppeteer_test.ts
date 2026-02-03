import { expect } from 'chai';
import { loadCachedTestBed, takeScreenshot, TestBed } from '../../../puppeteer-tests/util';
import { AnomaliesTableSkPO } from './anomalies-table-sk_po';
import { anomaly_table, GROUP_REPORT_RESPONSE_WITH_SID, GROUP_REPORT_RESPONSE } from './test_data';
import { ElementHandle } from 'puppeteer';
import { Page } from 'puppeteer';
import { assert } from 'chai';

describe('anomalies-table-sk', () => {
  let testBed: TestBed;

  before(async () => {
    testBed = await loadCachedTestBed();
  });
  let anomaliesTableSk: ElementHandle;
  let anomaliesTableSkPO: AnomaliesTableSkPO;

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    anomaliesTableSk = (await testBed.page.$('anomalies-table-sk'))!;
    anomaliesTableSkPO = new AnomaliesTableSkPO(anomaliesTableSk);
  });

  it('should render the demo page', async () => {
    expect(await testBed.page.$$('anomalies-table-sk')).to.have.length(2); // Smoke test.
  });

  describe('with anomalies', () => {
    beforeEach(async () => {
      await testBed.page.setRequestInterception(true);
      testBed.page.on('request', (request) => {
        if (request.url().endsWith('/_/anomalies/group_report') && request.method() === 'POST') {
          request.respond({
            status: 200,
            contentType: 'application/json',
            body: JSON.stringify(GROUP_REPORT_RESPONSE_WITH_SID),
          });
        } else {
          request.continue();
        }
      });
      await testBed.page.click('#populate-tables');
    });

    afterEach(async () => {
      await testBed.page.setRequestInterception(false);
      testBed.page.removeAllListeners('request');
      await takeScreenshot(testBed.page, 'perf', 'anomalies-table-sk');
    });

    it('shows the default view', async () => {
      const rowCount = await anomaliesTableSkPO.getRowCount();
      expect(rowCount).to.be.greaterThanOrEqual(anomaly_table.length);
    });

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

    it('should be able to hide and show rows', async () => {
      // Initially, the child row should be hidden.
      expect(await anomaliesTableSkPO.getChildRowCount()).to.equal(0);

      // Click the expand button of the first group.
      await anomaliesTableSkPO.clickExpandButton(0);
      expect(await anomaliesTableSkPO.getChildRowCount()).to.be.greaterThan(0);

      // Click it again to collapse.
      await anomaliesTableSkPO.clickExpandButton(0);
      expect(await anomaliesTableSkPO.getChildRowCount()).to.equal(0);
    });

    it('should be able to expand collapsed rows', async () => {
      const initialRowCount = await anomaliesTableSkPO.getRowCount();
      await anomaliesTableSkPO.clickExpandButton(0);
      const expandedRowCount = await anomaliesTableSkPO.getRowCount();
      // Expect 2 additional rows (group size 2).
      expect(expandedRowCount).to.be.equal(initialRowCount + 2);
    });

    it('should be able to click triage button once it clicks one row', async () => {
      await anomaliesTableSkPO.clickCheckbox(1);
      await anomaliesTableSkPO.clickTriageButton();
      const triageMenu = await testBed.page.$('triage-menu-sk');
      assert.isNotNull(triageMenu);
    });

    it('should be able to click expand checkbox', async () => {
      await anomaliesTableSkPO.clickExpandButton(0);
      const summaryRowCount: number = await anomaliesTableSkPO.getParentExpandRowCount();
      expect(summaryRowCount).to.equal(1);
    });

    it('should display the correct bug ids', async () => {
      const bugLinks = await anomaliesTableSkPO.bugLinks;
      const bugId = await (await bugLinks.item(0)).innerText;

      expect(bugId).to.equal(anomaly_table[0].bug_id.toString());
    });

    it('should have the correct bug link hrefs', async () => {
      const bugLinks = await anomaliesTableSkPO.bugLinks;
      const link: string = (await (await bugLinks.item(0)).getAttribute('href'))!;
      expect(link).to.equal(`http://b/${anomaly_table[0].bug_id}`);
    });

    it('opens a new tab with the correct URL for the trending icon', async () => {
      await anomaliesTableSkPO.clickCheckbox(1);
      const openMultiGraphUrl = async () => await testBed.page.click('#open-multi-graph');

      // Start waiting for the popup and click the link at the same time.
      const [popup] = await Promise.all([
        new Promise<Page>((resolve) => testBed.page.once('popup', resolve)),
        openMultiGraphUrl(),
      ]);
      assert.isNotNull(popup);
    });

    it('opens a new report page with the correct URL for a multiple anomalies', async () => {
      await anomaliesTableSkPO.clickExpandButton(0);
      await anomaliesTableSkPO.clickCheckbox(0);
      await anomaliesTableSkPO.clickCheckbox(1);

      await anomaliesTableSkPO.clickGraphButton();
      const reportPageUrl = await navigateTo(testBed.page, testBed.baseUrl, `/u/?sid=test-sid`);
      assert.exists(reportPageUrl);
    });
  });

  describe('open report page with single anomaly id', async () => {
    beforeEach(async () => {
      // This test needs to set up its own mocks and navigate, so we
      // clear the listeners from the parent beforeEach.
      testBed.page.removeAllListeners('request');
      await testBed.page.setRequestInterception(true);

      testBed.page.on('request', (request) => {
        if (request.url().endsWith('/_/anomalies/group_report') && request.method() === 'POST') {
          request.respond({
            status: 200,
            contentType: 'application/json',
            body: JSON.stringify(GROUP_REPORT_RESPONSE),
          });
        } else {
          request.continue();
        }
      });
      await testBed.page.click('#populate-tables');
    });

    it('opens a new report page with the correct URL for single anomaly', async () => {
      await anomaliesTableSkPO.clickExpandButton(0);
      await anomaliesTableSkPO.clickCheckbox(0);
      await anomaliesTableSkPO.clickCheckbox(1);

      await anomaliesTableSkPO.clickGraphButton();
      const reportPageUrl = await navigateTo(testBed.page, testBed.baseUrl, `/u/?anomalyIDs=1`);
      assert.exists(reportPageUrl);
    });
  });

  describe('grouping configuration', () => {
    beforeEach(async () => {
      testBed.page.removeAllListeners('request');
      await testBed.page.setRequestInterception(true);
      testBed.page.on('request', (request) => {
        if (request.url().endsWith('/_/anomalies/group_report') && request.method() === 'POST') {
          request.respond({
            status: 200,
            contentType: 'application/json',
            body: JSON.stringify(GROUP_REPORT_RESPONSE),
          });
        } else {
          request.continue();
        }
      });
      await testBed.page.click('#populate-tables-for-grouping');

      await anomaliesTableSkPO.setRevisionMode('OVERLAPPING');
      await anomaliesTableSkPO.setGroupBy('BENCHMARK', true);
      await anomaliesTableSkPO.setGroupBy('BOT', false);
      await anomaliesTableSkPO.setGroupBy('TEST', false);
      await anomaliesTableSkPO.setGroupSingles(false);
    });

    afterEach(async () => {
      await testBed.page.setRequestInterception(false);
      testBed.page.removeAllListeners('request');
      await takeScreenshot(testBed.page, 'perf', 'anomalies-table-sk_grouping');
    });

    it('1. Revision: EXACT | GroupBy: NONE', async () => {
      await anomaliesTableSkPO.setRevisionMode('EXACT');
      await anomaliesTableSkPO.setGroupBy('BENCHMARK', false);

      // Groups:
      // 1. Bug 12345 (merged) -> 1 Group.
      // 2. Rev A (3 items) -> 1 Group (Multi-item).
      // 3. Rev B (1 item) -> 1 Group (Single).
      // 4. Single 1 (1 item) -> 1 Group.
      // 5. Single 2 (1 item) -> 1 Group.
      // Total Groups = 5.
      // Total Rows = 5 Groups + 1 Header Row (normalized by browser) = 6 Rows.
      expect(await anomaliesTableSkPO.getRowCount()).to.equal(6);
    });

    it('2. Revision: OVERLAPPING | GroupBy: NONE', async () => {
      await anomaliesTableSkPO.setRevisionMode('OVERLAPPING');
      await anomaliesTableSkPO.setGroupBy('BENCHMARK', false);

      // Groups:
      // 1. Bug 12345 (merged) -> 1 Group.
      // 2. Rev A (3 items) -> 1 Group (Exclude from overlap check).
      // 3. Rev B (1 item) -> 1 Group (Matches EXACT first, separate from Rev A).
      // 4. Single 1 (1 item) -> 1 Group.
      // 5. Single 2 (1 item) -> 1 Group.
      // Total Groups = 5.
      // Total Rows = 5 Groups + 1 Header Row = 6 Rows.
      expect(await anomaliesTableSkPO.getRowCount()).to.equal(6);
    });

    it('3. Revision: ANY | GroupBy: NONE', async () => {
      await anomaliesTableSkPO.setRevisionMode('ANY');
      await anomaliesTableSkPO.setGroupBy('BENCHMARK', false);

      // Groups:
      // 1. Bug 12345 (merged) -> 1 Group.
      // 2. All others (6 items) -> 1 Revision Group (due to ANY mode).
      // Total Groups = 2.
      // Total Rows = 2 Groups + 1 Header Row = 3 Rows.
      expect(await anomaliesTableSkPO.getRowCount()).to.equal(3);
    });

    it('4. Revision: ANY | GroupBy: BOT', async () => {
      await anomaliesTableSkPO.setRevisionMode('ANY');
      await anomaliesTableSkPO.setGroupBy('BENCHMARK', false);
      await anomaliesTableSkPO.setGroupBy('BOT', true);
      // Logic:
      // 1. Bug 12345 Group (merged) -> 1 Row.
      // 2. Revision Group (ANY) splits by BOT:
      //    - BotA Group (5 items) -> 1 Row.
      //    - BotB Group (1 item) -> 1 Row.
      // Total = 1 Bug + 1 BotA + 1 BotB + 1 Header = 4 Rows.
      expect(await anomaliesTableSkPO.getRowCount()).to.equal(4);
    });

    it('5. Revision: ANY | GroupBy: BENCHMARK', async () => {
      await anomaliesTableSkPO.setRevisionMode('ANY');
      expect(await anomaliesTableSkPO.getRowCount()).to.equal(4);
      // Logic:
      // 1. Bug 12345 Group (merged) -> 1 Group.
      // 2. Revision Group (ANY) contains 6 items.
      //    Split by BENCHMARK:
      //    - BenchX (Rev A 1,2,3; Rev B 1) -> 1 Group.
      //    - BenchZ (Single 1,2) -> 1 Group.
      // Total Groups = 3 (Bug + BenchX + BenchZ).
      // Total Rows = 3 Groups + 1 Header Row = 4 Rows.
      expect(await anomaliesTableSkPO.getRowCount()).to.equal(4);
    });

    it('6. GroupSingles: TRUE', async () => {
      await anomaliesTableSkPO.setRevisionMode('EXACT');
      await anomaliesTableSkPO.setGroupSingles(true);

      // Groups:
      // 1. Bug 12345 (merged) -> 1 Group.
      // 2. Rev A (Multi-item) -> 1 Group.
      // 3. Rev B (Single) matches BenchX.
      // 4. Single 1 matches BenchZ.
      // 5. Single 2 matches BenchZ.
      // GroupSingles=TRUE (default criteria: BENCHMARK).
      // - BenchX Group: Rev B (1 item).
      // - BenchZ Group: Single 1 + Single 2 (2 items).
      // Total Groups = 1 Bug + 1 Rev A + 1 RevB(BenchX) + 1 Singles(BenchZ) = 4 Groups.
      // Total Rows = 4 Groups + 1 Header Row = 5 Rows.
      expect(await anomaliesTableSkPO.getRowCount()).to.equal(5);
    });
  });

  async function navigateTo(
    page: Page,
    base: string,
    queryParams = ''
  ): Promise<AnomaliesTableSkPO> {
    await page.goto(`${base}${queryParams}`);
    return new AnomaliesTableSkPO(page.$('anomalies-table-sk'));
  }
});
