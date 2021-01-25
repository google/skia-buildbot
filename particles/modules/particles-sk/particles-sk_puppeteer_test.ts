import * as path from 'path';
// eslint-disable-next-line import/no-extraneous-dependencies
import { expect } from 'chai';
import {
  loadCachedTestBed, takeScreenshot, TestBed,
} from '../../../puppeteer-tests/util';

describe('particles-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed(
      path.join(__dirname, '..', '..', 'webpack.config.ts'),
    );
  });

  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/dist/particles-sk.html`);
    await testBed.page.setViewport({ width: 1536, height: 1024 });
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('particles-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('shows controls for uniforms', async () => {
      // Wait for JSON editor to load.
      await testBed.page.waitForSelector('div.jsoneditor');
      await takeScreenshot(testBed.page, 'particles', 'particles-sk');
    });
  });
});
