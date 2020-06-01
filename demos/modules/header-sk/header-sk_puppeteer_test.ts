import * as path from 'path';
import { expect } from 'chai';
import { setUpPuppeteerAndDemoPageServer, takeScreenshot } from '../../../puppeteer-tests/util';

describe('header-sk', () => {
  const testBed = setUpPuppeteerAndDemoPageServer(path.join(__dirname, '..', '..', 'webpack.config.ts'));

  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/dist/header-sk.html`);
    await testBed.page.setViewport({ width: 1500, height: 500 });
  });

  it('should render the main page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('header-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'skia-demos', 'header-sk');
    });
  });
});
