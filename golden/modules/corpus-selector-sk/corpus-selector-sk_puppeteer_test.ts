import { expect } from 'chai';
import { loadCachedTestBed, takeScreenshot, TestBed } from '../../../puppeteer-tests/util';

describe('corpus-selector-sk', () => {
  let testBed: TestBed;

  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
  });

  it('should render the demo page', async () => {
    // Basic smoke test that things loaded.
    expect(await testBed.page.$$('corpus-selector-sk')).to.have.length(3);
  });

  it('shows the default corpus renderer function', async () => {
    const selector = await testBed.page.$('#default');
    await takeScreenshot(selector!, 'gold', 'corpus-selector-sk');
  });

  it('supports a custom corpus renderer function', async () => {
    const selector = await testBed.page.$('#custom-fn');
    await takeScreenshot(selector!, 'gold', 'corpus-selector-sk_custom-fn');
  });

  it('handles very long strings', async () => {
    const selector = await testBed.page.$('#custom-fn-long-corpus');
    await takeScreenshot(selector!, 'gold', 'corpus-selector-sk_long-strings');
  });
});
