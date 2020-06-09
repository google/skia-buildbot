import * as path from 'path';
import { expect } from 'chai';
import { setUpPuppeteerAndDemoPageServer, takeScreenshot } from '../../../puppeteer-tests/util';

describe('patch-sk', () => {
  const testBed = setUpPuppeteerAndDemoPageServer(path.join(__dirname, '..', '..', 'webpack.config.ts'));

  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/dist/patch-sk.html`);
    await testBed.page.setViewport({ width: 1500, height: 500 });
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('patch-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'ct', 'patch-sk');
    });

    it('shows the delete dialog', async () => {
      await testBed.page.click('delete-icon-sk');
      await takeScreenshot(testBed.page, 'ct', 'patch-sk_delete');
    });

    it('shows the task details', async () => {
      await testBed.page.click('a.details');
      await takeScreenshot(testBed.page, 'ct', 'patch-sk_task-details');
    });
  });
});
