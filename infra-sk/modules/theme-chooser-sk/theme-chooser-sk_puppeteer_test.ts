import { expect } from 'chai';
import {
  loadCachedTestBed,
  takeScreenshot, TestBed,
} from '../../../puppeteer-tests/util';

describe('theme-chooser-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport({ width: 1500, height: 2500 });
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('theme-chooser-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    [false, true].forEach((darkmode) => {
      describe(darkmode ? 'default theme' : 'light theme', () => {
        const suffix = darkmode ? '_default' : '_light';

        beforeEach(async () => {
          if (!darkmode) {
            await testBed.page.click('theme-chooser-sk');
          }
        });

        it(`shows all examples`, async () => {
          await takeScreenshot(testBed.page, 'infra-sk', `theme-chooser-sk${suffix}`);
        });

        it('shows a toast', async () => {
          await testBed.page.click('#make-toast');
          await new Promise((resolve) => setTimeout(resolve, 1000)); // Wait until toast is visible.
          const element = await testBed.page.$('toast-sk');
          await takeScreenshot(element!, 'infra-sk', `theme-chooser-sk_toast-sk${suffix}`);
        });

        it('shows an error toast', async () => {
          await testBed.page.click('#show-error-toast');
          await new Promise((resolve) => setTimeout(resolve, 1000)); // Wait until toast is visible.
          const element = await testBed.page.$('error-toast-sk toast-sk');
          await takeScreenshot(element!, 'infra-sk', `theme-chooser-sk_error-toast-sk${suffix}`);
        });
      });
    });
  });
});
