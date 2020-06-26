import { expect } from 'chai';
import { takeScreenshot, TestBed } from '../../../puppeteer-tests/util';
import { loadGoldWebpack } from '../common_puppeteer_test/common_puppeteer_test';

describe('byblameentry-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadGoldWebpack();
  });

  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/dist/byblameentry-sk.html`);
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('byblameentry-sk')).to.have.length(1);
  });

  it('should take a screenshot', async () => {
    const byBlameEntry = await testBed.page.$('byblameentry-sk');
    await takeScreenshot(byBlameEntry!, 'gold', 'byblameentry-sk');
  });
});
