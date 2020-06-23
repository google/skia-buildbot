import * as path from 'path';
import { expect } from 'chai';
import { setUpPuppeteerAndDemoPageServer, takeScreenshot } from '../../../puppeteer-tests/util';

describe('search-controls-sk', () => {
  // Contains page and baseUrl.
  const testBed =
    setUpPuppeteerAndDemoPageServer(path.join(__dirname, '..', '..', 'webpack.config.ts'));

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
