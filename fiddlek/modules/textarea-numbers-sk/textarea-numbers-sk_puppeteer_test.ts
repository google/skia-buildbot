import { expect } from 'chai';
import {
  loadCachedTestBed,
  takeScreenshot, TestBed,
} from '../../../puppeteer-tests/util';

describe('textarea-numbers-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport({ width: 400, height: 550 });
  });

  it('should render the demo page (smoke test)', async () => {
    expect(await testBed.page.$$('textarea-numbers-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'fiddle', 'textarea-numbers-sk');
    });

    it('error lines can be cleared', async () => {
      await testBed.page.click('#clear_error_lines');
      await takeScreenshot(
        testBed.page,
        'fiddle',
        'textarea-numbers-sk_cleared',
      );
    });
  });
});
