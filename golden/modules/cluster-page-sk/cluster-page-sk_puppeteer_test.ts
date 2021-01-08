import { expect } from 'chai';
import {
  addEventListenersToPuppeteerPage, EventName, loadCachedTestBed,
  takeScreenshot, TestBed
} from '../../../puppeteer-tests/util';
import { positiveDigest, negativeDigest, untriagedDigest } from '../cluster-page-sk/test_data';
import {
  clickNodeWithDigest,
  shiftClickNodeWithDigest
} from '../cluster-digests-sk/cluster-digests-sk_puppeteer_test';
import path from "path";

describe('cluster-page-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed(
        path.join(__dirname, '..', '..', 'webpack.config.ts')
    );
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

  it('shows all values when a paramset key is clicked', async () => {
    await testBed.page.setViewport({ width: 1200, height: 1200 });
    const done = promiseFactory('layout-complete');
    await clickParamKey(testBed, 'gpu');
    await done;
    await takeScreenshot(testBed.page, 'gold', 'cluster-page-sk_key-clicked');
  });

  it('shows nodes with matching values when a value is clicked', async () => {
    await testBed.page.setViewport({ width: 1200, height: 1200 });
    const done = promiseFactory('layout-complete');
    await clickParamValue(testBed, 'AMD');
    await done;
    await takeScreenshot(testBed.page, 'gold', 'cluster-page-sk_value-clicked');
  });

  it('can zoom in using the keyboard', async () => {
    await testBed.page.setViewport({ width: 1200, height: 1200 });
    const done = promiseFactory('layout-complete');
    await testBed.page.type('cluster-page-sk', 'aa');
    await done;
    await takeScreenshot(testBed.page, 'gold', 'cluster-page-sk_zoom-in');
  });

  it('can zoom out using the keyboard', async () => {
    await testBed.page.setViewport({ width: 1200, height: 1200 });
    const done = promiseFactory('layout-complete');
    await testBed.page.type('cluster-page-sk', 'zz');
    await done;
    await takeScreenshot(testBed.page, 'gold', 'cluster-page-sk_zoom-out');
  });

  it('can increase node spacing using the keyboard', async () => {
    await testBed.page.setViewport({ width: 1200, height: 1200 });
    const done = promiseFactory('layout-complete');
    await testBed.page.type('cluster-page-sk', 'ss');
    await done;
    await takeScreenshot(testBed.page, 'gold', 'cluster-page-sk_more-node-space');
  });

  it('can decrease node spacing using the keyboard', async () => {
    await testBed.page.setViewport({ width: 1200, height: 1200 });
    const done = promiseFactory('layout-complete');
    await testBed.page.type('cluster-page-sk', 'xx');
    await done;
    await takeScreenshot(testBed.page, 'gold', 'cluster-page-sk_less-node-space');
  });

  async function clickParamKey(testBed: TestBed, key: string) {
    await testBed.page.click(`paramset-sk[clickable] th[data-key="${key}"]`);
  }

  async function clickParamValue(testBed: TestBed, value: string) {
    await testBed.page.click(`paramset-sk[clickable] div[data-value="${value}"]`);
  }
});
