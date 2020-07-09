import { expect } from 'chai';
import { addEventListenersToPuppeteerPage, takeScreenshot, TestBed } from '../../../puppeteer-tests/util';
import { loadGoldWebpack } from '../common_puppeteer_test/common_puppeteer_test';

describe('cluster-digests-sk', () => {
    let testBed: TestBed;
    before(async () => {
        testBed = await loadGoldWebpack();
    });

    beforeEach(async () => {
        const eventPromise = await addEventListenersToPuppeteerPage(testBed.page,
            ['layout-complete']);
        const loaded = eventPromise('layout-complete'); // Emitted when layout stabilizes.
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
});
