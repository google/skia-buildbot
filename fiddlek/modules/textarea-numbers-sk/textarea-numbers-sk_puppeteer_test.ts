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

    it('default fold with fold tokens', async () => {
      await testBed.page.click('#add_fold_tokens');
      await takeScreenshot(
        testBed.page,
        'fiddle',
        'textarea-numbers-sk_default-fold',
      );
    });

    it('expand outer fold', async () => {
      await testBed.page.click('#add_fold_tokens');
      await testBed.page.click('#expand_outer_fold');
      await takeScreenshot(
        testBed.page,
        'fiddle',
        'textarea-numbers-sk_expand-outer-fold',
      );
    });

    it('expand inner fold', async () => {
      await testBed.page.click('#add_fold_tokens');
      await testBed.page.click('#expand_outer_fold');
      await testBed.page.click('#expand_inner_fold');
      await takeScreenshot(
        testBed.page,
        'fiddle',
        'textarea-numbers-sk_expand-inner-fold',
      );
    });
  });
});
