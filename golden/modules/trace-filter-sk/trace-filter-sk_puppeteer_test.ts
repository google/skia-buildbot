import * as path from 'path';
import { expect } from 'chai';
import { setUpPuppeteerAndDemoPageServer, takeScreenshot } from '../../../puppeteer-tests/util';

describe('trace-filter-sk', () => {
  // Contains page and baseUrl.
  const testBed =
    setUpPuppeteerAndDemoPageServer(path.join(__dirname, '..', '..', 'webpack.config.ts'));

  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/dist/trace-filter-sk.html`);
  });

  it('should render the demo page', async () => {
    // Basic smoke test that things loaded.
    expect(await testBed.page.$$('trace-filter-sk')).to.have.length(1);
  });

  describe('empty selection', () => {
    beforeEach(async () => {
      await testBed.page.click('button.edit-query'); // Open the query-editor-sk dialog.
      await testBed.page.click('button.clear_selections'); // Clear selection.
      await testBed.page.click('button.show-matches'); // Apply (this closes the dialog).
    });

    it('shows the user input', async () => {
      await takeScreenshot(testBed.page, 'gold', 'trace-filter-sk_empty-selection');
    });

    it('opens the query dialog', async () => {
      await testBed.page.click('button.edit-query');
      await testBed.page.click('query-dialog-sk select-sk div:nth-child(1)');
      await takeScreenshot(
        testBed.page, 'gold', 'trace-filter-sk_empty-selection_query-dialog-open');
    });
  });

  describe('non-empty selection', () => {
    it('shows the user input', async () => {
      await takeScreenshot(testBed.page, 'gold', 'trace-filter-sk_nonempty-selection');
    });

    it('opens the query dialog', async () => {
      await testBed.page.click('button.edit-query');
      await testBed.page.click('query-dialog-sk select-sk div:nth-child(1)');
      await takeScreenshot(
        testBed.page, 'gold', 'trace-filter-sk_nonempty-selection_query-dialog-open');
    });
  })
});
