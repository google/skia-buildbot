import * as path from 'path';
import { expect } from 'chai';
import { setUpPuppeteerAndDemoPageServer, takeScreenshot } from '../../../puppeteer-tests/util';

describe('byblameentry-sk', () => {
  // Contains page and baseUrl.
  const testBed = setUpPuppeteerAndDemoPageServer(path.join(__dirname, '..', '..', 'webpack.config.js'));

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
