import * as path from 'path';
import {
  setUpPuppeteerAndDemoPageServer,
  takeScreenshot,
} from '../../../puppeteer-tests/util';

describe('task-scheduler-scaffold-sk', () => {
  const testBed = setUpPuppeteerAndDemoPageServer(
    path.join(__dirname, '..', '..', 'webpack.config.ts')
  );

  beforeEach(async () => {
    await testBed.page.goto(
      `${testBed.baseUrl}/dist/task-scheduler-scaffold-sk.html`
    );
    await testBed.page.setViewport({ width: 650, height: 400 });
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(
        testBed.page,
        'task-scheduler',
        'task-scheduler-scaffold-sk'
      );
    });
  });
});
