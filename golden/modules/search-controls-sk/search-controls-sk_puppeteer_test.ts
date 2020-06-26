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
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('search-controls-sk')).to.have.length(1);
  });

  it('should take a screenshot', async () => {
    await takeScreenshot(testBed.page, 'gold', 'search-controls-sk');
  });
});
