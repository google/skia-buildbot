import * as path from 'path';
import {
  setUpPuppeteerAndDemoPageServer,
  takeScreenshot,
} from '../../../puppeteer-tests/util';
import { TaskSk } from './task-sk';
import { task2, FakeTaskSchedulerService } from '../rpc-mock';

describe('task-sk', () => {
  const testBed = setUpPuppeteerAndDemoPageServer(
    path.join(__dirname, '..', '..', 'webpack.config.ts')
  );

  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/dist/task-sk.html`);
    await testBed.page.setViewport({ width: 1429, height: 836 });
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'task-scheduler', 'task-sk');
    });
  });
});
