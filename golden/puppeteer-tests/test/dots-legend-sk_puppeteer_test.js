const expect = require('chai').expect;
const setUpPuppeteerAndDemoPageServer = require('./util').setUpPuppeteerAndDemoPageServer;
const takeScreenshot = require('./util').takeScreenshot;

describe('dots-legend-sk', () => {
  const pp = setUpPuppeteerAndDemoPageServer(); // Contains page and baseUrl.

  beforeEach(async () => {
    await pp.page.goto(`${pp.baseUrl}/dist/dots-legend-sk.html`);
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await pp.page.$$('dots-legend-sk')).to.have.length(2);
  });

  describe('screenshots', () => {
    it('some digests', async () => {
      const dotsLegendSk = await pp.page.$('#some-digests');
      await takeScreenshot(dotsLegendSk, 'dots-legend-sk');
    });

    it('too many digests', async () => {
      const dotsLegendSk = await pp.page.$('#too-many-digests');
      await takeScreenshot(
        dotsLegendSk, 'dots-legend-sk_too-many-digests',
      );
    });
  });
});
