import { expect } from 'chai';
import { addEventListenersToPuppeteerPage, takeScreenshot, TestBed } from '../../../puppeteer-tests/util';
import { loadGoldWebpack } from '../common_puppeteer_test/common_puppeteer_test';
import { Page } from 'puppeteer';

describe('ignores-page-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadGoldWebpack();
  });

  it('should render the demo page', async () => {
    await navigateTo(testBed.page, testBed.baseUrl, '');
    // Smoke test.
    expect(await testBed.page.$$('ignores-page-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('should show the default page', async () => {
      await navigateTo(testBed.page, testBed.baseUrl);
      await testBed.page.setViewport({ width: 1300, height: 2100 });
      await takeScreenshot(testBed.page, 'gold', 'ignores-page-sk');
    });

    it('should show the counts of all traces', async () => {
      await navigateTo(testBed.page, testBed.baseUrl, '?count_all=true');
      await testBed.page.setViewport({ width: 1300, height: 2100 });
      await takeScreenshot(testBed.page, 'gold', 'ignores-page-sk_all-traces');
    });

    it('should show the delete confirmation dialog', async () => {
      await navigateTo(testBed.page, testBed.baseUrl);
      // Focus in a little to see better.
      await testBed.page.setViewport({ width: 1300, height: 1300 });
      await testBed.page.click(
        'ignores-page-sk tbody > tr:nth-child(1) > td.mutate-icons > delete-icon-sk',
      );
      await takeScreenshot(testBed.page, 'gold', 'ignores-page-sk_delete-dialog');
    });

    it('should show the edit ignore rule modal when update is clicked', async () => {
      await navigateTo(testBed.page, testBed.baseUrl);
      // Focus in a little to see better.
      await testBed.page.setViewport({ width: 1300, height: 1300 });
      await testBed.page.click(
        'ignores-page-sk tbody > tr:nth-child(5) > td.mutate-icons > mode-edit-icon-sk',
      );
      await takeScreenshot(testBed.page, 'gold', 'ignores-page-sk_update-modal');
    });

    it('should show the create ignore rule modal when create is clicked', async () => {
      await navigateTo(testBed.page, testBed.baseUrl);
      // Focus in a little to see better.
      await testBed.page.setViewport({ width: 1300, height: 1300 });
      await testBed.page.click('ignores-page-sk .controls button.create');
      await takeScreenshot(testBed.page, 'gold', 'ignores-page-sk_create-modal');
    });
  });
});

async function navigateTo(page: Page, base: string, queryParams = '') {
  const eventPromise = await addEventListenersToPuppeteerPage(page, ['end-task']);
  const loaded = eventPromise('end-task'); // Emitted when page is loaded.
  await page.goto(`${base}/dist/ignores-page-sk.html${queryParams}`);
  await loaded;
}
