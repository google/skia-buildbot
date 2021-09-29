import * as path from 'path';
import { expect } from 'chai';
import {
  inBazel,
  loadCachedTestBed, takeScreenshot, TestBed,
} from '../../../puppeteer-tests/util';

describe('commit-detail-panel-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed(
      path.join(__dirname, '..', '..', 'webpack.config.ts'),
    );
  });

  beforeEach(async () => {
    await testBed.page.goto(
      inBazel() ? testBed.baseUrl : `${testBed.baseUrl}/dist/commit-detail-panel-sk.html`,
    );
    await testBed.page.setViewport({ width: 400, height: 550 });
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('commit-detail-panel-sk')).to.have.length(4);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'perf', 'commit-detail-panel-sk');
    });
  });
});
