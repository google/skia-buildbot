import { expect } from 'chai';
import { addEventListenersToPuppeteerPage, takeScreenshot, TestBed } from '../../../puppeteer-tests/util';
import { loadGoldWebpack } from '../common_puppeteer_test/common_puppeteer_test';

describe('triagelog-page-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadGoldWebpack();
  });

  beforeEach(async () => {
    const eventPromise = await addEventListenersToPuppeteerPage(testBed.page, ['end-task']);
    const loaded = eventPromise('end-task'); // Emitted when page is loaded.
    await testBed.page.goto(`${testBed.baseUrl}/dist/triagelog-page-sk.html`);
    await loaded;
  });

  it('should render the demo page', async () => {
    // Basic smoke test.
    expect(await testBed.page.$$('triagelog-page-sk')).to.have.length(1);
  });

  it('should take a screenshot', async () => {
    await testBed.page.setViewport({ width: 1200, height: 2600 });
    await takeScreenshot(testBed.page, 'gold', 'triagelog-page-sk');
  });
});
