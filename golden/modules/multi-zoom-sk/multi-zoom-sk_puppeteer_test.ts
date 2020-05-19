import * as path from 'path';
import { expect } from 'chai';
import { setUpPuppeteerAndDemoPageServer, takeScreenshot } from '../../../puppeteer-tests/util';

describe('multi-zoom-sk', () => {
  // Contains page and baseUrl.
  const testBed = setUpPuppeteerAndDemoPageServer(path.join(__dirname, '..', '..', 'webpack.config.ts'));

  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/dist/multi-zoom-sk.html`, { waitUntil: 'networkidle0' });
  });

  it('should render the demo page', async () => {
    // Smoke test. There are 5 test cases we send to Gold and 1 extra in a dialog that we do not.
    expect(await testBed.page.$$('multi-zoom-sk')).to.have.length(6);
  });

  describe('screenshots', () => {
    it('looks good when left and right are a normal size', async () => {
      const multiZoomSk = await testBed.page.$('#normal');
      await takeScreenshot(multiZoomSk!, 'gold', 'multi-zoom-sk');
    });

    it('shows two images of different size', async () => {
      const multiZoomSk = await testBed.page.$('#mismatch');
      await takeScreenshot(multiZoomSk!, 'gold', 'multi-zoom-sk_mismatch');
    });

    it('works for two small base64 encoded images', async () => {
      const multiZoomSk = await testBed.page.$('#base64');
      await takeScreenshot(multiZoomSk!, 'gold', 'multi-zoom-sk_base64-small');
    });

    it('is zoomed in with the grid on', async () => {
      const multiZoomSk = await testBed.page.$('#zoomed_grid');
      await takeScreenshot(multiZoomSk!, 'gold', 'multi-zoom-sk_zoomed-grid');
    });

    it('shows nth largest pixel', async () => {
      const multiZoomSk = await testBed.page.$('#base64_nthpixel');
      await takeScreenshot(multiZoomSk!, 'gold', 'multi-zoom-sk_nth-pixel');
    });
  });
});
