import { expect } from 'chai';
import { loadCachedTestBed, takeScreenshot, TestBed } from '../../../puppeteer-tests/util';

describe('alerts-page-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport({ width: 1200, height: 800 });
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('alerts-page-sk')).to.have.length(2);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'perf', 'alerts-page-sk');
    });

    it('clicks on "New"', async () => {
      await testBed.page.click('button.action');
      await takeScreenshot(testBed.page, 'perf', 'alerts-page-sk_new_dialog');
    });

    it('clicks on "Show deleted configs"', async () => {
      await testBed.page.click('#showDeletedConfigs');
      await takeScreenshot(testBed.page, 'perf', 'alerts-page-sk_show_deleted');
    });

    it('clicks on "Edit"', async () => {
      await testBed.page.click('alerts-page-sk create-icon-sk');
      await takeScreenshot(testBed.page, 'perf', 'alerts-page-sk_edit_dialog');
    });
  });
});
