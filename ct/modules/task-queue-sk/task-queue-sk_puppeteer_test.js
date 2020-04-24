const expect = require('chai').expect;
const path = require('path');
const setUpPuppeteerAndDemoPageServer = require('../../../puppeteer-tests/util').setUpPuppeteerAndDemoPageServer;
const takeScreenshot = require('../../../puppeteer-tests/util').takeScreenshot;

describe('task-queue-sk', () => {
  const testBed = setUpPuppeteerAndDemoPageServer(path.join(__dirname, '..', '..', 'webpack.config.js'));

  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/dist/task-queue-sk.html`);
    await testBed.page.setViewport({ width: 1500, height: 500 });
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('task-queue-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'ct', 'task-queue-sk');
    });

    it('shows the delete dialog', async () => {
      await testBed.page.click('delete-icon-sk');
      await takeScreenshot(testBed.page, 'ct', 'task-queue-sk_delete');
    });

    it('shows the task details', async () => {
      await testBed.page.click('a.details');
      await takeScreenshot(testBed.page, 'ct', 'task-queue-sk_task-details');
    });
  });
});
