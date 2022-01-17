import { expect } from 'chai';
import {
  loadCachedTestBed, takeScreenshot, TestBed,
} from '../../../puppeteer-tests/util';

describe('alert-config-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport({ width: 2000, height: 2500 });
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('alert-config-sk')).to.have.length(2);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'perf', 'alert-config-sk');
    });
    it('does not show group_by if window.sk.perf.display_group_by is false', async () => {
      await testBed.page.click('#hide_group_by');
      await takeScreenshot(testBed.page, 'perf', 'alert-config-sk-no-group-by');
    });
  });
});
