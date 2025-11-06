import { expect } from 'chai';
import { loadCachedTestBed, takeScreenshot, TestBed } from '../../../puppeteer-tests/util';

describe('regressions-page-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport({ width: 400, height: 550 });
  });

  it('should render the demo page (smoke test)', async () => {
    expect(await testBed.page.$$('regressions-page-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('displays the default view', async () => {
      await takeScreenshot(testBed.page, 'perf', 'regressions-page-sk');
    });

    it('displays empty table if no regressions for selected subscription', async () => {
      await testBed.page.select('select[id^="filter-"]', 'Sheriff Config 3');
      await takeScreenshot(testBed.page, 'perf', 'regressions-page-sk-no-regs');
    });

    it('displays table if some regressions present for selected subscription', async () => {
      await testBed.page.select('select[id^="filter-"]', 'Sheriff Config 2');
      await takeScreenshot(testBed.page, 'perf', 'regressions-page-sk-some-regs');
    });
  });
});
