import { expect } from 'chai';
import {
  loadCachedTestBed, takeScreenshot, TestBed,
} from '../../../puppeteer-tests/util';

describe('trybot-page-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport({ width: 800, height: 1024 });
  });

  it('should render the demo page (smoke test)', async () => {
    expect(await testBed.page.$$('trybot-page-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await testBed.page.waitForSelector('#load-complete pre');
      await takeScreenshot(testBed.page, 'perf', 'trybot-page-sk');
    });
  });
});
