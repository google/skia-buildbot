import { expect } from 'chai';
import { loadCachedTestBed, takeScreenshot, TestBed } from '../../../puppeteer-tests/util';

describe('commit-detail-picker-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport({ width: 800, height: 800 });
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('commit-detail-picker-sk')).to.have.length(2);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'perf', 'commit-detail-picker-sk');
    });

    it('opens the dialog', async () => {
      await testBed.page.$eval('commit-detail-picker-sk:nth-of-type(1) button', (el) =>
        (el as HTMLElement).click()
      );
      await testBed.page.waitForSelector('commit-detail-picker-sk:nth-of-type(1) dialog[open]');
      await takeScreenshot(testBed.page, 'perf', 'commit-detail-picker-sk_open');
    });

    it('opens the date range details', async () => {
      await testBed.page.$eval('commit-detail-picker-sk:nth-of-type(1) button', (el) =>
        (el as HTMLElement).click()
      );
      await testBed.page.waitForSelector('commit-detail-picker-sk:nth-of-type(1) dialog[open]');
      await testBed.page.$eval('commit-detail-picker-sk:nth-of-type(1) summary', (el) =>
        (el as HTMLElement).click()
      );
      await takeScreenshot(testBed.page, 'perf', 'commit-detail-picker-sk_date_range');
    });
  });
});
