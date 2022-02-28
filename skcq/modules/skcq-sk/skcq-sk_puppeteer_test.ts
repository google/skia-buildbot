import { expect } from 'chai';
import {
  inBazel,
  loadCachedTestBed,
  takeScreenshot,
  TestBed,
} from '../../../puppeteer-tests/util';

describe('skcq-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });
  beforeEach(async () => {
    await testBed.page.goto(
        inBazel() ? testBed.baseUrl : `${testBed.baseUrl}/dist/skcq-sk.html`);
    await testBed.page.setViewport({ width: 1300, height: 1300 });
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('skcq-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'skcq', 'skcq-sk');
    });
  });
});
