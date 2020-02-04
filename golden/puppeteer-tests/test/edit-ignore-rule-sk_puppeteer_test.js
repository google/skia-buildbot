const expect = require('chai').expect;
const setUpPuppeteerAndDemoPageServer = require('./util').setUpPuppeteerAndDemoPageServer;
const takeScreenshot = require('./util').takeScreenshot;

describe('edit-ignore-rule-sk', function() {
  setUpPuppeteerAndDemoPageServer();  // Sets up this.page and this.baseUrl.

  beforeEach(async function() {
    await this.page.goto(`${this.baseUrl}/dist/edit-ignore-rule-sk.html`);
  });

  it('should render the demo page', async function() {
    // Smoke test.
    expect(await this.page.$$('edit-ignore-rule-sk')).to.have.length(3);
  });

  describe('screenshots', function() {
    it('a view with nothing selected', async function() {
      const editor = await this.page.$('#empty');
      await takeScreenshot(editor, 'edit-ignore-rule-sk');
    });

    it('All inputs filled out', async function() {
      const editor = await this.page.$('#filled');
      await takeScreenshot(editor, 'edit-ignore-rule-sk_with-data');
    });

    it('invalid inputs', async function() {
      const editor = await this.page.$('#missing');
      await takeScreenshot(editor, 'edit-ignore-rule-sk_missing-data');
    });
  });
});
