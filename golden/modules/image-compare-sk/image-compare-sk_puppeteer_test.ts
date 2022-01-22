import { expect } from 'chai';
import { loadCachedTestBed, takeScreenshot, TestBed } from '../../../puppeteer-tests/util';

describe('image-compare-sk', () => {
  let testBed: TestBed;

  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl, { waitUntil: 'networkidle0' });
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('image-compare-sk')).to.have.length(3);
  });

  describe('screenshots', () => {
    it('has the left and right image', async () => {
      const imageCompareSk = await testBed.page.$('#normal');
      await takeScreenshot(imageCompareSk!, 'gold', 'image-compare-sk');
    });

    it('shows the multi-zoom-sk dialog when zoom button clicked', async () => {
      await testBed.page.setViewport({ width: 1000, height: 800 });
      await testBed.page.click('#normal button.zoom_btn');
      await takeScreenshot(testBed.page, 'gold', 'image-compare-sk_zoom-dialog');
    });

    it('has just the left image', async () => {
      const imageCompareSk = await testBed.page.$('#no_right');
      await takeScreenshot(
        imageCompareSk!, 'gold', 'image-compare-sk_no-right',
      );
    });

    it('shows full size images', async () => {
      const imageCompareSk = await testBed.page.$('#full_size_images');
      await takeScreenshot(imageCompareSk!, 'gold', 'image-compare-sk_full-size-images');
    });
  });
});
