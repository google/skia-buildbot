import * as path from 'path';
import {
  setUpPuppeteerAndDemoPageServer,
  takeScreenshot,
} from '../../../puppeteer-tests/util';

describe('job-sk', () => {
  const testBed = setUpPuppeteerAndDemoPageServer(
    path.join(__dirname, '..', '..', 'webpack.config.ts')
  );

  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/dist/job-sk.html`);
    await testBed.page.setViewport({ width: 835, height: 1110 });
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'task-scheduler', 'job-sk');
    });
    it('cancels jobs', async () => {
      await testBed.page.click('#cancelButton');
      await takeScreenshot(testBed.page, 'task-scheduler', 'job-sk-canceled');
    });
  });
});
