const expect = require('chai').expect;
const path = require('path');
const setUpPuppeteerAndDemoPageServer = require('../../../puppeteer-tests/util').setUpPuppeteerAndDemoPageServer;
const takeScreenshot = require('../../../puppeteer-tests/util').takeScreenshot;

describe('edit-ignore-rule-sk', () => {
  // Contains page and baseUrl.
  const testBed = setUpPuppeteerAndDemoPageServer(path.join(__dirname, '..', '..', 'webpack.config.js'));

  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/dist/edit-ignore-rule-sk.html`);
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('edit-ignore-rule-sk')).to.have.length(3);
  });

  describe('screenshots', () => {
    it('a view with nothing selected', async () => {
      const editor = await testBed.page.$('#empty');
      await takeScreenshot(editor, 'gold', 'edit-ignore-rule-sk');
    });

    it('All inputs filled out', async () => {
      const editor = await testBed.page.$('#filled');
      await takeScreenshot(editor, 'gold', 'edit-ignore-rule-sk_with-data');
    });

    it('invalid inputs', async () => {
      const editor = await testBed.page.$('#missing');
      await takeScreenshot(editor, 'gold', 'edit-ignore-rule-sk_missing-data');
    });
  });
});
