import { expect } from 'chai';
import {inBazel, loadCachedTestBed, takeScreenshot, TestBed} from '../../../puppeteer-tests/util';
import path from "path";

describe('dots-legend-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed(
        path.join(__dirname, '..', '..', 'webpack.config.ts')
    );
  });

  beforeEach(async () => {
    await testBed.page.goto(
        inBazel() ? testBed.baseUrl : `${testBed.baseUrl}/dist/dots-legend-sk.html`);
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('dots-legend-sk')).to.have.length(2);
  });

  describe('screenshots', () => {
    it('some digests', async () => {
      const dotsLegendSk = await testBed.page.$('#some-digests');
      await takeScreenshot(dotsLegendSk!, 'gold', 'dots-legend-sk');
    });

    it('too many digests', async () => {
      const dotsLegendSk = await testBed.page.$('#too-many-digests');
      await takeScreenshot(dotsLegendSk!, 'gold', 'dots-legend-sk_too-many-digests');
    });
  });
});
