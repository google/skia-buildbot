const expect = require('chai').expect;
const setUpPuppeteerAndDemoPageServer = require('./util').setUpPuppeteerAndDemoPageServer;
const takeScreenshot = require('./util').takeScreenshot;

describe('dots-legend-sk', function() {
  setUpPuppeteerAndDemoPageServer();  // Sets up this.page and this.baseUrl.

  beforeEach(async function() {
    await this.page.goto(`${this.baseUrl}/dist/dots-legend-sk.html`);
  });

  it('should render the demo page', async function() {
    // Smoke test.
    expect(await this.page.$$('dots-legend-sk')).to.have.length(2);
  });

  describe('screenshots', function() {
    it('some digests', async function() {
      const dotsLegendSk = await this.page.$('#some-digests');
      await takeScreenshot(dotsLegendSk, 'Test-Dots-Legend-Sk');
    });

    it('too many digests', async function() {
      const dotsLegendSk = await this.page.$('#too-many-digests');
      await takeScreenshot(
          dotsLegendSk, 'Test-Dots-Legend-Sk_Too-Many-Digests');
    });
  });
});
