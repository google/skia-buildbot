import {
  addEventListenersToPuppeteerPage,
  loadCachedTestBed,
  takeScreenshot,
  TestBed,
} from '../../../puppeteer-tests/util';

describe('triagelog-page-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  describe('v1 APIs', () => {
    beforeEach(async () => {
      const eventPromise = await addEventListenersToPuppeteerPage(testBed.page, ['end-task']);
      const loaded = eventPromise('end-task'); // Emitted when page is loaded.
      await testBed.page.goto(testBed.baseUrl);
      await loaded;
    });

    it('should take a screenshot', async () => {
      await testBed.page.setViewport({ width: 1200, height: 2600 });
      await takeScreenshot(testBed.page, 'gold', 'triagelog-page-sk');
    });
  });

  describe('v2 APIs', () => {
    beforeEach(async () => {
      const eventPromise = await addEventListenersToPuppeteerPage(testBed.page, ['end-task']);
      const loaded = eventPromise('end-task'); // Emitted when page is loaded.
      await testBed.page.goto(`${testBed.baseUrl}?use_new_api=true`);
      await loaded;
    });

    it('should take a screenshot', async () => {
      await testBed.page.setViewport({ width: 1200, height: 1800 });
      await takeScreenshot(testBed.page, 'gold', 'triagelog-page-sk-v2');
    });
  });
});
