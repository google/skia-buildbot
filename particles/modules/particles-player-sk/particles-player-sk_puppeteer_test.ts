// eslint-disable-next-line import/no-extraneous-dependencies
import { expect } from 'chai';
import {
  loadCachedTestBed, takeScreenshot, TestBed,
} from '../../../puppeteer-tests/util';

describe('particles-player-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport({ width: 400, height: 800 });
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('particles-player-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('shows controls for uniforms', async () => {
      // Wait for WASM to load.
      await testBed.page.waitForSelector('#results #loaded');
      await takeScreenshot(testBed.page, 'particles', 'particles-player-sk-with-uniforms');
    });
    it('does not show controls if there are no uniforms', async () => {
      // Wait for WASM to load.
      await testBed.page.click('#nouniforms');
      await testBed.page.waitForSelector('#results #loaded');
      await takeScreenshot(testBed.page, 'particles', 'particles-player-sk-without-uniforms');
    });
  });
});
