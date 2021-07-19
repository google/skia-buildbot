import * as path from 'path';
import { expect } from 'chai';
import {
  loadCachedTestBed,
  takeScreenshot,
  TestBed,
} from '../../../puppeteer-tests/util';

describe('skcq-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed(
      path.join(__dirname, '..', '..', 'webpack.config.ts'),
    );
  });
  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/dist/skcq-sk.html`);
    await testBed.page.setViewport({ width: 1300, height: 1300 });
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('skcq-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'skcq', 'skcq-sk');
    });
  });
});
