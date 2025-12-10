import { assert } from 'chai';
import { loadCachedTestBed, takeScreenshot, TestBed } from '../../../puppeteer-tests/util';
// TODO(b/450184956) Rewrite demo to plot-google-chart
describe('explore-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    testBed.page.on('dialog', async (dialog) => {
      await dialog.accept();
    });
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport({ width: 1600, height: 1600 });
    await testBed.page.click('#close_query_dialog');
  });

  it('should render the demo page', async () => {
    // Smoke test.
    assert.equal(await (await testBed.page.$$('explore-sk')).length, 1);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'perf', 'explore-sk');
    });
    it('shows the query dialog', async () => {
      await testBed.page.click('#demo-show-query-dialog');
      await testBed.page.waitForSelector('#query-dialog', {
        visible: true,
      });
      await takeScreenshot(testBed.page, 'perf', 'explore-sk_query_dialog');
    });

    it('loads shows the help dialog on a keypress of ?', async () => {
      await testBed.page.click('#demo-show-help');
      await testBed.page.waitForSelector('keyboard-shortcuts-help-sk');

      // Wait for the help dialog to be visible, handling both Light and Shadow DOM.
      await testBed.page.waitForFunction(() => {
        const help = document.querySelector('keyboard-shortcuts-help-sk');
        if (!help) {
          return false;
        }
        const dialog = help.shadowRoot
          ? help.shadowRoot.querySelector('#help-dialog')
          : help.querySelector('#help-dialog');
        if (!dialog) {
          return false;
        }
        const style = window.getComputedStyle(dialog);
        return style.display !== 'none' && style.visibility !== 'hidden' && style.opacity !== '0';
      });

      await takeScreenshot(testBed.page, 'perf', 'explore-sk_help_dialog');
    });
  });
});
