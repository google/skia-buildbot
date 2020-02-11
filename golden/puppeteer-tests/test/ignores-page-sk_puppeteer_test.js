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
      await navigateTo(this.page, this.baseUrl);
      await this.page.setViewport({ width: 1300, height: 2100 });
      await takeScreenshot(this.page, 'ignores-page-sk');
    });

    it('should show the counts of all traces', async function() {
      await navigateTo(this.page, this.baseUrl, '?count_all=true');
      await this.page.setViewport({ width: 1300, height: 2100 });
      await takeScreenshot(this.page, 'ignores-page-sk_all-traces');
    });

    it('should show the delete confirmation dialog', async function() {
      await navigateTo(this.page, this.baseUrl);
      // Focus in a little to see better.
      await this.page.setViewport({ width: 1300, height: 1300 });
      await this.page.click(
        'ignores-page-sk tbody > tr:nth-child(1) > td.mutate-icons > delete-icon-sk');
      await takeScreenshot(this.page, 'ignores-page-sk_delete-dialog');
    });

    it('should show the edit ignore rule modal when update is clicked', async function() {
      await navigateTo(this.page, this.baseUrl);
      // Focus in a little to see better.
      await this.page.setViewport({ width: 1300, height: 1300 });
      await this.page.click(
        'ignores-page-sk tbody > tr:nth-child(5) > td.mutate-icons > mode-edit-icon-sk');
      await takeScreenshot(this.page, 'ignores-page-sk_update-modal');
    });

    it('should show the create ignore rule modal when create is clicked', async function() {
      await navigateTo(this.page, this.baseUrl );
      // Focus in a little to see better.
      await this.page.setViewport({ width: 1300, height: 1300 });
      await this.page.click('ignores-page-sk .controls button.create');
      await takeScreenshot(this.page, 'ignores-page-sk_create-modal');
    });
  });
});

async function navigateTo(page, base, queryParams = '') {
  const eventPromise =
    await addEventListenersToPuppeteerPage(page, ['busy-end']);
  const loaded = eventPromise('busy-end');  // Emitted when page is loaded.
  await page.goto(`${base}/dist/ignores-page-sk.html${queryParams}`);
  await loaded;
}
