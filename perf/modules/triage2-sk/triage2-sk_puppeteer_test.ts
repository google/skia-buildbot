import * as path from 'path';
import { expect } from 'chai';
import {
  inBazel,
  loadCachedTestBed, takeScreenshot, TestBed,
} from '../../../puppeteer-tests/util';

describe('triage2-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed(
        path.join(__dirname, '..', '..', 'webpack.config.ts')
    );
  });

  beforeEach(async () => {
    await testBed.page.goto(
      inBazel() ? testBed.baseUrl : `${testBed.baseUrl}/dist/triage2-sk.html`);
    await testBed.page.setViewport({ width: 600, height: 800 });
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('triage2-sk')).to.have.length(12);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'perf', 'triage2-sk');
    });
  });
});
