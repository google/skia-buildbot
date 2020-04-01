const expect = require('chai').expect;
const setUpPuppeteerAndDemoPageServer = require('./util').setUpPuppeteerAndDemoPageServer;
const takeScreenshot = require('./util').takeScreenshot;

describe('edit-ignore-rule-sk', () => {
  const testBed = setUpPuppeteerAndDemoPageServer(); // Contains page and baseUrl.

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
      await takeScreenshot(editor, 'edit-ignore-rule-sk');
    });

    it('All inputs filled out', async () => {
      const editor = await testBed.page.$('#filled');
      await takeScreenshot(editor, 'edit-ignore-rule-sk_with-data');
    });

    it('invalid inputs', async () => {
      const editor = await testBed.page.$('#missing');
      await takeScreenshot(editor, 'edit-ignore-rule-sk_missing-data');
    });
  });
});
