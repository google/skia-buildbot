import { expect } from 'chai';
import { takeScreenshot, TestBed } from '../../../puppeteer-tests/util';
import { loadGoldWebpack } from '../common_puppeteer_test/common_puppeteer_test';

describe('search-page-sk', () => {
  let testBed: TestBed;

  before(async () => {
    testBed = await loadGoldWebpack();
  });

  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/dist/search-page-sk.html`);
    await testBed.page.setViewport({width: 1200, height: 800});
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('search-page-sk')).to.have.length(1);
  });

  // it('shows an empty search criteria', async () => {
  //   await testBed.page.click('button#clear');
  //   await takeScreenshot(testBed.page, 'gold', 'search-controls-sk_empty');
  // });
});
