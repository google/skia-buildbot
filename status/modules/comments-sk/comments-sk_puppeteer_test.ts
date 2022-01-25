import { expect } from 'chai';
import {
  addEventListenersToPuppeteerPage,
  EventPromiseFactory, loadCachedTestBed,
  takeScreenshot, TestBed,
} from '../../../puppeteer-tests/util';

describe('comments-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });
  let eventPromise: EventPromiseFactory;
  beforeEach(async () => {
    eventPromise = await addEventListenersToPuppeteerPage(testBed.page, ['data-update']);
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport({ width: 600, height: 550 });
  });

  it('should render the demo page (smoke test)', async () => {
    expect(await testBed.page.$$('comments-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'status', 'comments-sk');
    });

    it('add comment flow', async () => {
      (await testBed.page.$('checkbox-sk[label=Flaky]'))!.click();
      (await testBed.page.$('checkbox-sk[label=IgnoreFailure]'))!.click();
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
