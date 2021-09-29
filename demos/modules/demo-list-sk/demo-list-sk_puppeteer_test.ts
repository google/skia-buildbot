import * as path from 'path';
import { expect } from 'chai';
import {
  loadCachedTestBed,
  takeScreenshot,
  TestBed,
} from '../../../puppeteer-tests/util';

describe('demo-list-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed(
      path.join(__dirname, '..', '..', 'webpack.config.ts'),
    );
  });
  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/dist/demo-list-sk.html`);
    await testBed.page.setViewport({ width: 1500, height: 500 });
  });

  it('should render the main page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('demo-list-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'skia-demos', 'demo-list-sk');
    });
  });
});
