import { expect } from 'chai';
import { loadCachedTestBed, takeScreenshot, TestBed } from '../../../puppeteer-tests/util';

describe('cluster-lastn-page-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport({ width: 600, height: 600 });
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('cluster-lastn-page-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'perf', 'cluster-lastn-page-sk');
    });
  });
});
