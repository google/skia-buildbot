import { expect } from 'chai';
import { takeScreenshot, TestBed } from '../../../puppeteer-tests/util';
import { loadGoldWebpack } from '../common_puppeteer_test/common_puppeteer_test';

describe('sort-toggle-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadGoldWebpack();
  });

  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/dist/sort-toggle-sk.html`);
  });

  it('should render the demo page', async () => {
    expect(await testBed.page.$$('sort-toggle-sk')).to.have.length(2); // Smoke test.
  });

  describe('screenshots', async () => {
    it('should show two sort toggles with age being selected by default', async () => {
      const sortToggleSk = await testBed.page.$('#two_sorts');
      await takeScreenshot(sortToggleSk!, 'gold', 'sort-toggle-sk');
    });

    it('should be negative', async () => {
      await testBed.page.click('#second');
      const sortToggleSk = await testBed.page.$('#two_sorts');
      await takeScreenshot(sortToggleSk!, 'gold', 'sort-toggle-sk_ascending-clicked');
    });
  });
});
