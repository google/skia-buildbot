const expect = require('chai').expect;
const addEventListenersToPuppeteerPage = require('./util').addEventListenersToPuppeteerPage;
const setUpPuppeteerAndDemoPageServer = require('./util').setUpPuppeteerAndDemoPageServer;
const takeScreenshot = require('./util').takeScreenshot;

describe('ignores-page-sk', function() {
  setUpPuppeteerAndDemoPageServer();  // Sets up this.page and this.baseUrl.

  it('should render the demo page', async function() {
    await navigateTo(this.page, this.baseUrl, '');
    // Smoke test.
    expect(await this.page.$$('ignores-page-sk')).to.have.length(1);
  });

  describe('screenshots', function() {
    it('should show the default page', async function() {
      await navigateTo(this.page, this.baseUrl, '');
      await this.page.setViewport({ width: 1300, height: 2100 });
      await takeScreenshot(this.page, 'ignores-page-sk');
    });

    it('should show a popup when delete is clicked', async function() {
      await navigateTo(this.page, this.baseUrl, '');
      // zoom in a little to see better.
      await this.page.setViewport({ width: 1300, height: 1300 });
      await this.page.click('ignores-page-sk tbody > tr:nth-child(1) > td.mutate-icons > delete-icon-sk');
      await takeScreenshot(this.page, 'ignores-page-sk_delete-popup');
    });

    it('should show the counts of all traces', async function() {
      await navigateTo(this.page, this.baseUrl, '?count_all=true');
      await this.page.setViewport({ width: 1300, height: 2100 });
      await takeScreenshot(this.page, 'ignores-page-sk_all-traces');
    });
  });
});

async function navigateTo(page, base, queryParams) {
  const eventPromise =
    await addEventListenersToPuppeteerPage(page, ['end-task']);
  const loaded = eventPromise('end-task');  // Emitted when page is loaded.
  await page.goto(`${base}/dist/ignores-page-sk.html${queryParams}`);
  await loaded;
}
