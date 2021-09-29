import * as path from 'path';
import { expect } from 'chai';
import {
  inBazel,
  loadCachedTestBed, takeScreenshot, TestBed,
} from '../../../puppeteer-tests/util';

describe('word-cloud-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed(
      path.join(__dirname, '..', '..', 'webpack.config.ts'),
    );
  });

  beforeEach(async () => {
    await testBed.page.goto(
      inBazel() ? testBed.baseUrl : `${testBed.baseUrl}/dist/word-cloud-sk.html`,
    );
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
