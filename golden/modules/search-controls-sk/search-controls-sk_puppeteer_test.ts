import { expect } from 'chai';
import { takeScreenshot, TestBed } from '../../../puppeteer-tests/util';
import { loadGoldWebpack } from '../common_puppeteer_test/common_puppeteer_test';

describe('search-controls-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadGoldWebpack();
  });
  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/dist/search-controls-sk.html`);
    await testBed.page.setViewport({width: 1200, height: 800});
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('search-controls-sk')).to.have.length(1);
  });

  it('shows an empty search criteria', async () => {
    await testBed.page.click('button#clear');
    await takeScreenshot(testBed.page, 'gold', 'search-controls-sk_empty');
  });

  it('shows a non-empty search criteria', async () => {
    await takeScreenshot(testBed.page, 'gold', 'search-controls-sk');
  });

  it('shows the left-hand trace filter editor', async () => {
    await testBed.page.click('.traces button.edit-query');
    await takeScreenshot(testBed.page, 'gold', 'search-controls-sk_left-hand-trace-filter-editor');
  });

  it('shows more filters', async () => {
    await testBed.page.click('button.more-filters');
    await takeScreenshot(testBed.page, 'gold', 'search-controls-sk_more-filters');
  });

  it('shows the left-hand trace filter editor', async () => {
    await testBed.page.click('button.more-filters');
    await testBed.page.click('filter-dialog-sk button.edit-query')
    await takeScreenshot(
      testBed.page, 'gold', 'search-controls-sk_right-hand-trace-filter-editor');
  });

});
