import * as path from 'path';
import { expect } from 'chai';
import { setUpPuppeteerAndDemoPageServer, takeScreenshot } from '../../../puppeteer-tests/util';

describe('corpus-selector-sk', () => {
  // Contains page and baseUrl.
  const testBed = setUpPuppeteerAndDemoPageServer(path.join(__dirname, '..', '..', 'webpack.config.ts'));

  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/dist/corpus-selector-sk.html`);
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
