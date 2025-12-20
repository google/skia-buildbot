import { expect } from 'chai';
import { loadCachedTestBed, takeScreenshot, TestBed } from '../../../puppeteer-tests/util';

describe('algo-select-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.reload();
    await testBed.page.setViewport({ width: 500, height: 500 });
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('algo-select-sk')).to.have.length(3);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'perf', 'algo-select-sk');
    });

    it('clicks on "Individual"', async () => {
      // Target the second element on the page (inside the first div).
      const elements = await testBed.page.$$('algo-select-sk');
      await (await elements[1].$('.stepfit'))!.click();
      await takeScreenshot(testBed.page, 'perf', 'algo-select-sk_stepfit');
    });

    it('clicks on "K-Means"', async () => {
      // Target the first element on the page (direct child of body).
      const elements = await testBed.page.$$('algo-select-sk');
      await (await elements[0].$('.kmeans'))!.click();
      await takeScreenshot(testBed.page, 'perf', 'algo-select-sk_kmeans');
    });
  });
});
