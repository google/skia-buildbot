import { expect } from 'chai';
import {
  loadCachedTestBed, takeScreenshot, TestBed,
} from '../../../puppeteer-tests/util';

describe('autoroller-status-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport({ width: 400, height: 550 });
  });

  it('should render the demo page (smoke test)', async () => {
    expect(await testBed.page.$$('autoroller-status-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'status', 'autoroller-status-sk');
    });

    it('highlights hovering', async () => {
      testBed.page.hover('.roller');
      await takeScreenshot(testBed.page, 'status', 'autoroller-status-sk_hover');
    });
  });
});
