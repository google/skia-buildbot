import { expect } from 'chai';
import { addEventListenersToPuppeteerPage, EventName,
    takeScreenshot, TestBed } from '../../../puppeteer-tests/util';
import { loadGoldWebpack } from '../common_puppeteer_test/common_puppeteer_test';
import { ElementHandle } from 'puppeteer';
import { positiveDigest, negativeDigest, untriagedDigest } from './test_data';

describe('cluster-digests-sk', () => {
    let testBed: TestBed;
    before(async () => {
        testBed = await loadGoldWebpack();
    });

    let promiseFactory: <T>(eventName: EventName) => Promise<T>;
    let clusterDigestsSk: ElementHandle;

    beforeEach(async () => {
        promiseFactory = await addEventListenersToPuppeteerPage(testBed.page,
            ['layout-complete', 'selection-changed']);
        const loaded = promiseFactory('layout-complete'); // Emitted when layout stabilizes.
        await testBed.page.goto(`${testBed.baseUrl}/dist/cluster-digests-sk.html`);
        await loaded;
        clusterDigestsSk = (await testBed.page.$('#cluster svg'))!;
    });

    it('should render the demo page', async () => {
        // Smoke test.
        expect(await testBed.page.$$('cluster-digests-sk')).to.have.length(1);
    });

    it('should take a screenshot', async () => {
        await takeScreenshot(clusterDigestsSk, 'gold', 'cluster-digests-sk');
    });

    it('supports single digest selection via clicking', async () => {
        await clickNodeAndExpectSelectionChangedEvent(positiveDigest, [positiveDigest]);

        await takeScreenshot(clusterDigestsSk, 'gold', 'cluster-digests-sk_one-positive-selected');

        await clickNodeAndExpectSelectionChangedEvent(untriagedDigest, [untriagedDigest]);

        await takeScreenshot(clusterDigestsSk, 'gold',
            'cluster-digests-sk_one-untriaged-selected');
    });

    it('supports multiple digest selection via shift clicking', async () => {
        await clickNodeAndExpectSelectionChangedEvent(negativeDigest, [negativeDigest]);

        await shiftClickNodeAndExpectSelectionChangedEvent(positiveDigest,
            [negativeDigest, positiveDigest]);

        await takeScreenshot(clusterDigestsSk, 'gold', 'cluster-digests-sk_two-digests-selected');

        await shiftClickNodeAndExpectSelectionChangedEvent(untriagedDigest,
            [negativeDigest, positiveDigest, untriagedDigest]);

        await takeScreenshot(clusterDigestsSk, 'gold',
            'cluster-digests-sk_three-digests-selected');
    });

    it('clears selection by clicking anywhere on the svg that is not on a node', async () => {
        await clickNodeAndExpectSelectionChangedEvent(negativeDigest, [negativeDigest]);

        const clickEvent = promiseFactory<Array<string>>('selection-changed');
        await clusterDigestsSk.click();
        const evt = await clickEvent;
        expect(evt).to.deep.equal([]);
    });

    async function clickNodeWithDigest(digest: string) {
        await testBed.page.click(`circle.node[data-digest="${digest}"]`);
    }

    async function shiftClickNodeWithDigest(digest: string) {
        await testBed.page.keyboard.down('Shift');
        await clickNodeWithDigest(digest);
        await testBed.page.keyboard.up('Shift');
    }

    async function clickNodeAndExpectSelectionChangedEvent(digest: string, expectedSelection: string[]) {
        const clickEvent = promiseFactory<Array<string>>('selection-changed');
        await clickNodeWithDigest(digest);
        const evt = await clickEvent;
        expect(evt).to.deep.equal(expectedSelection);
    }

    async function shiftClickNodeAndExpectSelectionChangedEvent(digest: string, expectedSelection: string[]) {
        const clickEvent = promiseFactory<Array<string>>('selection-changed');
        await shiftClickNodeWithDigest(digest);
        const evt = await clickEvent;
        expect(evt).to.deep.equal(expectedSelection);
    }
});
