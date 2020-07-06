import * as path from 'path';
import { expect } from 'chai';
import {
  setUpPuppeteerAndDemoPageServer,
  takeScreenshot,
} from '../../../puppeteer-tests/util';

describe('alerts-page-sk', () => {
  const testBed = setUpPuppeteerAndDemoPageServer(
    path.join(__dirname, '..', '..', 'webpack.config.ts')
  );

  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/dist/alerts-page-sk.html`);
    await testBed.page.setViewport({ width: 800, height: 400 });
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('alerts-page-sk')).to.have.length(2);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'perf', 'alerts-page-sk');
    });
  });
});
