import { expect } from 'chai';
import {
  addEventListenersToPuppeteerPage,
  loadCachedTestBed,
  takeScreenshot,
  TestBed,
} from '../../../puppeteer-tests/util';
import { END_BUSY_EVENT } from '../codesize-scaffold-sk/events';

describe('binary-diff-page-sk', () => {
  let testBed: TestBed;

  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.setViewport({ width: 1024, height: 768 });
    const eventPromise = await addEventListenersToPuppeteerPage(testBed.page, [END_BUSY_EVENT]);
    const loaded = eventPromise(END_BUSY_EVENT); // Emitted when page is loaded.
    await testBed.page.goto(testBed.baseUrl);
    await loaded;
  });

  it('should render the demo page', async () => {
    expect(await testBed.page.$$('binary-diff-page-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'codesize', 'binary-diff-page-sk');
    });
  });
});
