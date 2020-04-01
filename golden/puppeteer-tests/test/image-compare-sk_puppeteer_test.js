const expect = require('chai').expect;
const setUpPuppeteerAndDemoPageServer = require('./util').setUpPuppeteerAndDemoPageServer;
const takeScreenshot = require('./util').takeScreenshot;

describe('image-compare-sk', () => {
  const testBed = setUpPuppeteerAndDemoPageServer(); // Contains page and baseUrl.

  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/dist/image-compare-sk.html`, { waitUntil: 'networkidle0' });
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('image-compare-sk')).to.have.length(2);
  });

  describe('screenshots', () => {
    it('has the left and right image', async () => {
      const imageCompareSk = await testBed.page.$('#normal');
      await takeScreenshot(imageCompareSk, 'image-compare-sk');
    });

    it('has just the left image', async () => {
      const imageCompareSk = await testBed.page.$('#no_right');
      await takeScreenshot(
        imageCompareSk, 'image-compare-sk_no-right',
      );
    });
  });
});
