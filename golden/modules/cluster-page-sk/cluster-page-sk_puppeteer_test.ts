import { expect } from 'chai';
import { addEventListenersToPuppeteerPage, EventName,
  takeScreenshot, TestBed } from '../../../puppeteer-tests/util';
import { loadGoldWebpack } from '../common_puppeteer_test/common_puppeteer_test';
import { positiveDigest, negativeDigest, untriagedDigest } from '../cluster-page-sk/test_data';
import {
  clickNodeWithDigest,
  shiftClickNodeWithDigest
} from '../cluster-digests-sk/cluster-digests-sk_puppeteer_test';

describe('cluster-page-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadGoldWebpack();
  });

  let promiseFactory: <T>(eventName: EventName) => Promise<T>;

  beforeEach(async () => {
    promiseFactory = await addEventListenersToPuppeteerPage(testBed.page,
        ['layout-complete', 'selection-changed']);
    const loaded = promiseFactory('layout-complete'); // Emitted when layout stabilizes.
    await testBed.page.goto(`${testBed.baseUrl}/dist/cluster-page-sk.html`);
    await loaded;
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('cluster-page-sk')).to.have.length(1);
  });

  it('should take a screenshot', async () => {
    await testBed.page.setViewport({ width: 1200, height: 1200 });
    await takeScreenshot(testBed.page, 'gold', 'cluster-page-sk');
  });

  it('shows details about a single digest when clicked', async () => {
    await testBed.page.setViewport({ width: 1200, height: 1200 });
    await clickNodeWithDigest(testBed, positiveDigest);
    await takeScreenshot(testBed.page, 'gold', 'cluster-page-sk_one-digest-selected');
  });

  it('shows diff between two digests that are selected', async () => {
    await testBed.page.setViewport({ width: 1200, height: 1200 });
    await clickNodeWithDigest(testBed, positiveDigest);
    await shiftClickNodeWithDigest(testBed, negativeDigest);
    await takeScreenshot(testBed.page, 'gold', 'cluster-page-sk_two-digests-selected');
  });

  it('shows a summary when more than two digests are selected', async () => {
    await testBed.page.setViewport({ width: 1200, height: 1200 });
    await clickNodeWithDigest(testBed, positiveDigest);
    await shiftClickNodeWithDigest(testBed, negativeDigest);
    await shiftClickNodeWithDigest(testBed, untriagedDigest);
    await takeScreenshot(testBed.page, 'gold', 'cluster-page-sk_three-digests-selected');
  });
});
