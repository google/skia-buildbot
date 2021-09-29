import * as path from 'path';
import { expect } from 'chai';
import {
  loadCachedTestBed,
  takeScreenshot,
  TestBed,
} from '../../../puppeteer-tests/util';

describe('bugs-slo-popup-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed(
      path.join(__dirname, '..', '..', 'webpack.config.ts'),
    );
  });
  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/static/bugs-slo-popup-sk.html`);
    await testBed.page.setViewport({ width: 800, height: 800 });

    await openDialog();
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('bugs-slo-popup-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await openDialog();
      await takeScreenshot(testBed.page, 'bugs-central', 'bugs-slo-popup-sk');
    });
  });

  const openDialog = async () => testBed.page.click('#show-dialog');
});
