import * as path from 'path';
import { expect } from 'chai';
import {
  loadCachedTestBed,
  takeScreenshot,
  TestBed,
} from '../../../puppeteer-tests/util';

describe('task-queue-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed(
      path.join(__dirname, '..', '..', 'webpack.config.ts'),
    );
  });
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

    it('shows the task details', async () => {
      await testBed.page.click('a.details');
      await takeScreenshot(testBed.page, 'ct', 'task-queue-sk_task-details');
    });
  });
});
