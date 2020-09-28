import * as path from 'path';
import { expect } from 'chai';
import {
  addEventListenersToPuppeteerPage,
  EventPromiseFactory,
  setUpPuppeteerAndDemoPageServer,
  takeScreenshot,
} from '../../../puppeteer-tests/util';

describe('comments-sk', () => {
  const testBed = setUpPuppeteerAndDemoPageServer(
    path.join(__dirname, '..', '..', 'webpack.config.ts')
  );
  let eventPromise: EventPromiseFactory;
  beforeEach(async () => {
    eventPromise = await addEventListenersToPuppeteerPage(testBed.page, ['data-update']);
    await testBed.page.goto(`${testBed.baseUrl}/dist/comments-sk.html`);
    await testBed.page.setViewport({ width: 400, height: 550 });
  });

  it('should render the demo page (smoke test)', async () => {
    expect(await testBed.page.$$('comments-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'status', 'comments-sk');
    });

    it('add comment flow', async () => {
      (await testBed.page.$$('checkbox-sk'))[0].click();
      (await testBed.page.$$('checkbox-sk'))[1].click();
      ((await testBed.page.$('input-sk')) as any).value = 'This is flaky, lets ignore it.';
      const updated = eventPromise('data-update');
      (await testBed.page.$('button'))!.click();
      await updated;
      await takeScreenshot(testBed.page, 'status', 'comments-sk add comment');
    });

    it('delete comment flow', async () => {
      const updated = eventPromise('data-update');
      (await testBed.page.$('delete-icon-sk'))!.click();
      await updated;
      await takeScreenshot(testBed.page, 'status', 'comments-sk delete comment');
    });
  });
});
