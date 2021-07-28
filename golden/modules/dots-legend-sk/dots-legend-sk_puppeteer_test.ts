import { expect } from 'chai';
import {loadCachedTestBed, takeScreenshot, TestBed} from '../../../puppeteer-tests/util';

describe('dots-legend-sk', () => {
  let testBed: TestBed;

  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
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
