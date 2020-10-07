import * as path from 'path';
import { expect } from 'chai';
import {
  setUpPuppeteerAndDemoPageServer,
  takeScreenshot,
} from '../../../puppeteer-tests/util';

describe('gantt-chart-sk', () => {
  const testBed = setUpPuppeteerAndDemoPageServer(
    path.join(__dirname, '..', '..', 'webpack.config.ts')
  );

  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/dist/gantt-chart-sk.html`);
    await testBed.page.setViewport({ width: 1000, height: 600 });
  });

  describe('screenshots', () => {
    it('simple chart', async () => {
      await testBed.page.click("#simple");
      await takeScreenshot(testBed.page, 'task-scheduler', 'gantt-chart-sk_simple');
    });

    it('simple chart with start and end', async () => {
      await testBed.page.click("#simple-start-end");
      await takeScreenshot(testBed.page, 'task-scheduler', 'gantt-chart-sk_simple-start-end');
    });

    it('simple chart with epochs', async () => {
      await testBed.page.click("#simple-epochs");
      await takeScreenshot(testBed.page, 'task-scheduler', 'gantt-chart-sk_simple-epochs');
    });

    it('mouse behavior', async () => {
      await testBed.page.click("#simple");
      await testBed.page.mouse.move(150, 300);
      await takeScreenshot(testBed.page, 'task-scheduler', 'gantt-chart-sk_mouse-cursor');
      await testBed.page.mouse.move(225, 300);
      await takeScreenshot(testBed.page, 'task-scheduler', 'gantt-chart-sk_mouse-cursor-snap');
      await testBed.page.mouse.down();
      await testBed.page.mouse.move(400, 300);
      await takeScreenshot(testBed.page, 'task-scheduler', 'gantt-chart-sk_mouse-selecting');
      await testBed.page.mouse.move(450, 300);
      await testBed.page.mouse.up();
      await testBed.page.mouse.move(500, 300);
      await takeScreenshot(testBed.page, 'task-scheduler', 'gantt-chart-sk_mouse-selected');
    });
  });
});
