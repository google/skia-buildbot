import * as path from 'path';
import { expect } from 'chai';
import {
  inBazel,
  loadCachedTestBed,
  takeScreenshot, TestBed,
} from '../../../puppeteer-tests/util';

describe('sort-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed(
      path.join(__dirname, '..', '..', 'webpack.config.ts'),
    );
  });

  beforeEach(async () => {
    await testBed.page.goto(inBazel() ? testBed.baseUrl : `${testBed.baseUrl}/sort-sk.html`);
    await testBed.page.setViewport({ width: 400, height: 900 });
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('sort-sk')).to.have.length(5);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'infra-sk', 'sort-sk');
    });
  });
});
