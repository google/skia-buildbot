import * as path from 'path';
import { expect } from 'chai';
import {
  loadCachedTestBed,
  takeScreenshot,
  TestBed,
} from '../../../puppeteer-tests/util';

describe('bugs-central-scaffold-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed(
      path.join(__dirname, '..', '..', 'webpack.config.ts'),
    );
  });
  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/static/bugs-central-scaffold-sk.html`);
    await testBed.page.setViewport({ width: 400, height: 500 });
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('bugs-central-scaffold-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'bugs-central', 'bugs-central-scaffold-sk');
    });
  });
});
