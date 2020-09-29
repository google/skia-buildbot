import * as path from 'path';
import { expect } from 'chai';
import {
  setUpPuppeteerAndDemoPageServer,
  takeScreenshot,
} from '../../../puppeteer-tests/util';

describe('task-graph-sk', () => {
  const testBed = setUpPuppeteerAndDemoPageServer(
    path.join(__dirname, '..', '..', 'webpack.config.ts')
  );

  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/dist/task-graph-sk.html`);
    await testBed.page.setViewport({ width: 850, height: 400 });
  });

  it('should render the demo page (smoke test)', async () => {
    expect(await testBed.page.$$('task-graph-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'task_scheduler', 'task-graph-sk');
    });
  });
});
