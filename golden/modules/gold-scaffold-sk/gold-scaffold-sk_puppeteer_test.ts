import { expect } from 'chai';
import {inBazel, loadCachedTestBed, takeScreenshot, TestBed} from '../../../puppeteer-tests/util';
import path from "path";

describe('gold-scaffold-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed(
        path.join(__dirname, '..', '..', 'webpack.config.ts')
    );
  });
  beforeEach(async () => {
    await testBed.page.goto(
        inBazel() ? testBed.baseUrl : `${testBed.baseUrl}/dist/gold-scaffold-sk.html`);
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('gold-scaffold-sk')).to.have.length(1);
  });

  it('should take a screenshot', async () => {
    await testBed.page.setViewport({ width: 1200, height: 600 });
    await takeScreenshot(testBed.page, 'gold', 'gold-scaffold-sk');
  });
});
