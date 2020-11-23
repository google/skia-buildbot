import * as path from 'path';
import { expect } from 'chai';
import {
  inBazel,
  setUpPuppeteerAndDemoPageServer,
  takeScreenshot,
} from '../../../puppeteer-tests/util';

describe('theme-chooser-sk', () => {
  const testBed = setUpPuppeteerAndDemoPageServer(
    path.join(__dirname, '..', '..', 'webpack.config.ts')
  );

  beforeEach(async () => {
    await testBed.page.goto(
      inBazel() ? testBed.baseUrl : `${testBed.baseUrl}/theme-chooser-sk.html`);
    await testBed.page.setViewport({ width: 400, height: 400 });
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('theme-chooser-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('shows the default theme', async () => {
      await takeScreenshot(testBed.page, 'infra-sk', 'theme-chooser-sk_default');
    });

    it('shows the dark theme', async () => {
      await testBed.page.click('theme-chooser-sk');
      await takeScreenshot(testBed.page, 'infra-sk', 'theme-chooser-sk_dark');
    });
  });
});
