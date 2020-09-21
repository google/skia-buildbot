import * as path from 'path';
import { expect } from 'chai';
import {
  setUpPuppeteerAndDemoPageServer,
  takeScreenshot,
} from '../../../puppeteer-tests/util';

describe('word-cloud-sk', () => {
  const testBed = setUpPuppeteerAndDemoPageServer(
    path.join(__dirname, '..', '..', 'webpack.config.ts'),
  );

  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/dist/word-cloud-sk.html`);
    await testBed.page.setViewport({ width: 400, height: 550 });
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('word-cloud-sk')).to.have.length(3);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'perf', 'word-cloud-sk');
    });
  });
});
