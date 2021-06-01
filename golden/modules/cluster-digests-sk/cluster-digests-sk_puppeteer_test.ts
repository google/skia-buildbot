import { expect } from 'chai';
import {
  addEventListenersToPuppeteerPage, EventName, loadCachedTestBed,
  takeScreenshot, TestBed
} from '../../../puppeteer-tests/util';
import { ElementHandle } from 'puppeteer';
import { positiveDigest, negativeDigest, untriagedDigest } from '../cluster-page-sk/test_data';
import path from "path";
import {ClusterDigestsSkPO} from './cluster-digests-sk_po';

describe('cluster-digests-sk', () => {
  let testBed: TestBed;

  before(async () => {
    testBed = await loadCachedTestBed(
        path.join(__dirname, '..', '..', 'webpack.config.ts')
    );
  });

  let promiseFactory: <T>(eventName: EventName) => Promise<T>;

  let clusterDigestsSk: ElementHandle;
  let clusterDigestsSkPO: ClusterDigestsSkPO;

  beforeEach(async () => {
    promiseFactory =
        await addEventListenersToPuppeteerPage(
            testBed.page,
            ['layout-complete', 'selection-changed']);

    const loaded = promiseFactory('layout-complete'); // Emitted when layout stabilizes.
    await testBed.page.goto(`${testBed.baseUrl}/dist/cluster-digests-sk.html`);
    await loaded;

    clusterDigestsSk = (await testBed.page.$('cluster-digests-sk'))!;
    clusterDigestsSkPO = new ClusterDigestsSkPO(clusterDigestsSk);
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('cluster-digests-sk')).to.have.length(1);
  });

  it('should take a screenshot', async () => {
    await takeScreenshot(clusterDigestsSk, 'gold', 'cluster-digests-sk');
  });

  it('should show digest labels', async () => {
    const loaded = promiseFactory('layout-complete'); // Emitted when layout stabilizes.
    await testBed.page.click('#labels');
    await loaded;

    await takeScreenshot(clusterDigestsSk, 'gold', 'cluster-digests-sk_with_labels');
  });

  it('supports single digest selection via clicking', async () => {
    await clickNodeAndExpectSelectionChanged(positiveDigest, [positiveDigest]);

    await takeScreenshot(clusterDigestsSk, 'gold', 'cluster-digests-sk_one-positive-selected');

    await clickNodeAndExpectSelectionChanged(untriagedDigest, [untriagedDigest]);

    await takeScreenshot(
        clusterDigestsSk, 'gold', 'cluster-digests-sk_one-untriaged-selected');
  });

  it('supports multiple digest selection via shift clicking', async () => {
    await clickNodeAndExpectSelectionChanged(negativeDigest, [negativeDigest]);

    await shiftClickNodeAndExpectSelectionChanged(
        positiveDigest, [negativeDigest, positiveDigest]);

    await takeScreenshot(clusterDigestsSk, 'gold', 'cluster-digests-sk_two-digests-selected');

    await shiftClickNodeAndExpectSelectionChanged(
        untriagedDigest, [negativeDigest, positiveDigest, untriagedDigest]);

    await takeScreenshot(clusterDigestsSk, 'gold',
        'cluster-digests-sk_three-digests-selected');
  });

  it('clears selection by clicking anywhere on the svg that is not on a node', async () => {
    await clickNodeAndExpectSelectionChanged(negativeDigest, [negativeDigest]);

    const event = promiseFactory<Array<string>>('selection-changed');
    await clusterDigestsSkPO.clickBackground();
    expect(await event).to.deep.equal([]);
    expect(await clusterDigestsSkPO.getSelection()).to.be.empty;
  });

  async function clickNodeAndExpectSelectionChanged(digest: string, expectedSelection: string[]) {
    const event = promiseFactory<Array<string>>('selection-changed');
    await clusterDigestsSkPO.clickNode(digest);
    expect(await event).to.deep.equal(expectedSelection);
    expect(await clusterDigestsSkPO.getSelection()).to.have.members(expectedSelection);
  }

  async function shiftClickNodeAndExpectSelectionChanged(
      digest: string, expectedSelection: string[]) {
    const event = promiseFactory<Array<string>>('selection-changed');
    await clusterDigestsSkPO.shiftClickNode(digest);
    expect(await event).to.deep.equal(expectedSelection);
    expect(await clusterDigestsSkPO.getSelection()).to.have.members(expectedSelection);
  }
});
