import * as path from 'path';
import { expect } from 'chai';
import {
  loadCachedTestBed, takeScreenshot, TestBed,
} from '../../../puppeteer-tests/util';

describe('day-range-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed(
        path.join(__dirname, '..', '..', 'webpack.config.ts')
    );
  });

  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/dist/day-range-sk.html`);
    await testBed.page.setViewport({ width: 400, height: 550 });
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('day-range-sk')).to.have.length(3);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'perf', 'day-range-sk');
    });
  });
});
