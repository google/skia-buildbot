import { expect } from 'chai';
import { addEventListenersToPuppeteerPage, EventName,
    takeScreenshot, TestBed } from '../../../puppeteer-tests/util';
import { loadGoldWebpack } from '../common_puppeteer_test/common_puppeteer_test';

describe('cluster-digests-sk', () => {
    let testBed: TestBed;
    before(async () => {
        testBed = await loadGoldWebpack();
    });

    let promiseFactory : <T>(eventName: EventName) => Promise<T>;

    beforeEach(async () => {
        promiseFactory = await addEventListenersToPuppeteerPage(testBed.page,
            ['layout-complete', 'selection-changed']);
        const loaded = promiseFactory('layout-complete'); // Emitted when layout stabilizes.
        await testBed.page.goto(`${testBed.baseUrl}/dist/cluster-digests-sk.html`);
        await loaded;
    });

    it('should render the demo page', async () => {
        // Smoke test.
        expect(await testBed.page.$$('cluster-digests-sk')).to.have.length(1);
    });

    it('should take a screenshot', async () => {
        const clusterDigestsSk = await testBed.page.$('#cluster svg');
        await takeScreenshot(clusterDigestsSk!, 'gold', 'cluster-digests-sk');
    });

    it('supports single digest selection via clicking', async () => {
        const clusterDigestsSk = await testBed.page.$('#cluster svg');

        let clickEvent = promiseFactory<Array<String>>('selection-changed');
        await clickNodeWithDigest(positiveDigest);
        let evt = await clickEvent;
        expect(evt).to.deep.equal([positiveDigest]);

        await takeScreenshot(clusterDigestsSk!, 'gold', 'cluster-digests-sk_one-positive-selected');

        clickEvent = promiseFactory<Array<String>>('selection-changed');
        await clickNodeWithDigest(untriagedDigest);
        evt = await clickEvent;
        expect(evt).to.deep.equal([untriagedDigest]);

        await takeScreenshot(clusterDigestsSk!, 'gold',
            'cluster-digests-sk_one-untriaged-selected');
    });

    it('supports multiple digest selection via shift clicking', async () => {
        const clusterDigestsSk = await testBed.page.$('#cluster svg');

        let clickEvent = promiseFactory<Array<String>>('selection-changed');
        await clickNodeWithDigest(negativeDigest);
        let evt = await clickEvent;
        expect(evt).to.deep.equal([negativeDigest]);

        clickEvent = promiseFactory<Array<String>>('selection-changed');
        await shiftClickNodeWithDigest(positiveDigest);
        evt = await clickEvent;
        expect(evt).to.deep.equal([negativeDigest, positiveDigest]);

        await takeScreenshot(clusterDigestsSk!, 'gold', 'cluster-digests-sk_two-digests-selected');

        clickEvent = promiseFactory<Array<String>>('selection-changed');
        await shiftClickNodeWithDigest(untriagedDigest);
        evt = await clickEvent;
        expect(evt).to.deep.equal([negativeDigest, positiveDigest, untriagedDigest]);

        await takeScreenshot(clusterDigestsSk!, 'gold',
            'cluster-digests-sk_three-digests-selected');
    });

    it('clears selection by clicking anywhere on the svg that is not on a node', async () => {
        let clickEvent = promiseFactory<Array<String>>('selection-changed');
        await clickNodeWithDigest(negativeDigest);
        let evt = await clickEvent;
        expect(evt).to.deep.equal([negativeDigest]);

        clickEvent = promiseFactory<Array<String>>('selection-changed');
        await testBed.page.click('#cluster svg');
        evt = await clickEvent;
        expect(evt).to.deep.equal([]);
    });

    // These digests are from test_data
    const positiveDigest = '99c58c7002073346ff55f446d47d6311';
    const negativeDigest = 'ec3b8f27397d99581e06eaa46d6d5837';
    const untriagedDigest = '6246b773851984c726cb2e1cb13510c2';
    async function clickNodeWithDigest(digest: String) {
        await testBed.page.click(`circle.node[data-digest="${digest}"]`);
    }

    async function shiftClickNodeWithDigest(digest: String) {
        await testBed.page.keyboard.down('Shift');
        await clickNodeWithDigest(digest);
        await testBed.page.keyboard.up('Shift');
    }
});
