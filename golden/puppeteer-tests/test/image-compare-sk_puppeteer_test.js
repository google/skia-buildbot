const expect = require('chai').expect;
const setUpPuppeteerAndDemoPageServer = require('./util').setUpPuppeteerAndDemoPageServer;
const takeScreenshot = require('./util').takeScreenshot;

describe('image-compare-sk', () => {
  setUpPuppeteerAndDemoPageServer(); // Sets up this.page and this.baseUrl.

  beforeEach(async function () {
    await this.page.goto(`${this.baseUrl}/dist/image-compare-sk.html`, { waitUntil: 'networkidle0' });
  });

  it('should render the demo page', async function () {
    // Smoke test.
    expect(await this.page.$$('image-compare-sk')).to.have.length(2);
  });

  describe('screenshots', () => {
    it('has the left and right image', async function () {
      const imageCompareSk = await this.page.$('#normal');
      await takeScreenshot(imageCompareSk, 'image-compare-sk');
    });

    it('has just the left image', async function () {
      const imageCompareSk = await this.page.$('#no_right');
      await takeScreenshot(
        imageCompareSk, 'image-compare-sk_no-right',
      );
    });
  });
});
