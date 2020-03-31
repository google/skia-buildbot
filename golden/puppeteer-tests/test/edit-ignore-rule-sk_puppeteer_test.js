const expect = require('chai').expect;
const setUpPuppeteerAndDemoPageServer = require('./util').setUpPuppeteerAndDemoPageServer;
const takeScreenshot = require('./util').takeScreenshot;

describe('edit-ignore-rule-sk', () => {
  const pp = setUpPuppeteerAndDemoPageServer(); // Contains page and baseUrl.

  beforeEach(async () => {
    await pp.page.goto(`${pp.baseUrl}/dist/edit-ignore-rule-sk.html`);
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await pp.page.$$('edit-ignore-rule-sk')).to.have.length(3);
  });

  describe('screenshots', () => {
    it('a view with nothing selected', async () => {
      const editor = await pp.page.$('#empty');
      await takeScreenshot(editor, 'edit-ignore-rule-sk');
    });

    it('All inputs filled out', async () => {
      const editor = await pp.page.$('#filled');
      await takeScreenshot(editor, 'edit-ignore-rule-sk_with-data');
    });

    it('invalid inputs', async () => {
      const editor = await pp.page.$('#missing');
      await takeScreenshot(editor, 'edit-ignore-rule-sk_missing-data');
    });
  });
});
