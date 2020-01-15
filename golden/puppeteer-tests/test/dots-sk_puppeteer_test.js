const expect = require('chai').expect;
const setUpPuppeteerAndDemoPageServer = require('./util').setUpPuppeteerAndDemoPageServer;
const takeScreenshot = require('./util').takeScreenshot;

describe('dots-sk', function() {
  setUpPuppeteerAndDemoPageServer();  // Sets up this.page and this.baseUrl.

  beforeEach(async function() {
    await this.page.goto(`${this.baseUrl}/dist/dots-sk.html`);
  });

  it('should render the demo page', async function() {
    // Smoke test.
    expect(await this.page.$$('dots-sk')).to.have.length(1);
  });

  describe('screenshots', function() {
    it('no highlighted traces', async function() {
      await this.page.setViewport({ width: 300, height: 100 });
      await takeScreenshot(this.page, 'dots-sk');
    });

    it('one highlighted trace', async function() {
      await this.page.setViewport({ width: 300, height: 100 });

      // Get canvas position.
      const canvas = await this.page.$('canvas');
      const boxModel = await canvas.boxModel();
      const x = boxModel.content[0].x, y = boxModel.content[0].y;

      // Hover over the leftmost dot of the first trace.
      await this.page.mouse.move(x + 10, y + 10);

      await takeScreenshot(this.page, 'dots-sk_highlighted');
    });
  });
});
