import { expect } from 'chai';
import { loadCachedTestBed, TestBed } from '../../../puppeteer-tests/util';
import { AnomaliesTableSkPO } from './anomalies-table-sk_po';
import {
  anomaly_table,
  groupRowsCount,
  GROUP_REPORT_RESPONSE_WITH_SID,
  GROUP_REPORT_RESPONSE,
} from './test_data';
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
    });

    it('shows the default view', async () => {
      const rowCount = await anomaliesTableSkPO.getRowCount();
      expect(rowCount).to.equal(anomaly_table.length + groupRowsCount);
    });

    it('should be able to expand collapsed rows', async () => {
      const initialRowCount = await anomaliesTableSkPO.getRowCount();
      await anomaliesTableSkPO.clickExpandButton(0);
      const expandedRowCount = await anomaliesTableSkPO.getRowCount();
      expect(expandedRowCount).to.be.equal(initialRowCount);
    });

    it('should be able to click triage button once it clicks one row', async () => {
      await anomaliesTableSkPO.clickCheckbox(1);
      await anomaliesTableSkPO.clickTriageButton();
      const triageMenu = await testBed.page.$('triage-menu-sk');
      assert.isNotNull(triageMenu);
    });

    it('should render the correct number of rows', async () => {
      const totalRowCount: number = await anomaliesTableSkPO.getRowCount();
      expect(totalRowCount).to.equal(anomaly_table.length + groupRowsCount);
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
      const openMultiGraphUrl = () => testBed.page.click('#open-multi-graph');

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

  async function navigateTo(
    page: Page,
    base: string,
    queryParams = ''
  ): Promise<AnomaliesTableSkPO> {
    await page.goto(`${base}${queryParams}`);
    return new AnomaliesTableSkPO(page.$('anomalies-table-sk'));
  }
});
