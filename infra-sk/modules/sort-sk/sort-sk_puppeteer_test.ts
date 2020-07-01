import * as path from 'path';
import { expect } from 'chai';
import {
  setUpPuppeteerAndDemoPageServer,
  takeScreenshot,
} from '../../../puppeteer-tests/util';

describe('sort-sk', () => {
  const testBed = setUpPuppeteerAndDemoPageServer(
    path.join(__dirname, '..', '..', 'webpack.config.ts')
  );

  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/sort-sk.html`);
    await testBed.page.setViewport({ width: 400, height: 900 });
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('sort-sk')).to.have.length(5);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'infra-sk', 'sort-sk');
    });
  });
});
