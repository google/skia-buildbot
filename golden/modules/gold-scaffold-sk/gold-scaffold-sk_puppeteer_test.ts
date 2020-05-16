import * as path from 'path';
import { expect } from 'chai';
import { setUpPuppeteerAndDemoPageServer, takeScreenshot } from '../../../puppeteer-tests/util';

describe('gold-scaffold-sk', () => {
  // Contains page and baseUrl.
  const testBed = setUpPuppeteerAndDemoPageServer(path.join(__dirname, '..', '..', 'webpack.config.js'));

  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/dist/gold-scaffold-sk.html`);
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('gold-scaffold-sk')).to.have.length(1);
  });

  it('should take a screenshot', async () => {
    await testBed.page.setViewport({ width: 1200, height: 600 });
    await takeScreenshot(testBed.page, 'gold', 'gold-scaffold-sk');
  });
});
