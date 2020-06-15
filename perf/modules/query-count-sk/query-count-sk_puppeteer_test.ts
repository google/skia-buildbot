import * as path from 'path';
import { expect } from 'chai';
import {
  setUpPuppeteerAndDemoPageServer,
  takeScreenshot,
} from '../../../puppeteer-tests/util';

describe('query-count-sk', () => {
  const testBed = setUpPuppeteerAndDemoPageServer(
    path.join(__dirname, '..', '..', 'webpack.config.ts')
  );

  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/dist/query-count-sk.html`);
    await testBed.page.setViewport({ width: 400, height: 400 });
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('query-count-sk')).to.have.length(3);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'perf', 'query-count-sk');
    });
  });
});
