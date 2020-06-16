import * as path from 'path';
import { expect } from 'chai';
import { setUpPuppeteerAndDemoPageServer, addEventListenersToPuppeteerPage, takeScreenshot } from '../../../puppeteer-tests/util';
import { Page } from 'puppeteer';

describe('list-page-sk', () => {
    // Contains page and baseUrl.
    const testBed = setUpPuppeteerAndDemoPageServer(path.join(__dirname, '..', '..', 'webpack.config.ts'));

    it('should render the demo page', async () => {
        await navigateTo(testBed.page, testBed.baseUrl, '');
        // Smoke test.
        expect(await testBed.page.$$('list-page-sk')).to.have.length(1);
    });

    describe('screenshots', () => {
        it('should show the default page', async () => {
            await navigateTo(testBed.page, testBed.baseUrl);
            await testBed.page.setViewport({ width: 1000, height: 1000 });
            await takeScreenshot(testBed.page, 'gold', 'list-page-sk');
        });

        it('should show a query dialog', async () => {
            await navigateTo(testBed.page, testBed.baseUrl,
                '?corpus=skp&disregard_ignores=true&query=alpha_type%3DOpaque');
            await testBed.page.setViewport({ width: 1000, height: 1000 });
            await testBed.page.click(
                'list-page-sk button.show_query_dialog',
            );
            await takeScreenshot(testBed.page, 'gold', 'list-page-sk_query-dialog');
        });

    });
});

async function navigateTo(page: Page, base: string, queryParams = '') {
    const eventPromise = await addEventListenersToPuppeteerPage(page, ['busy-end']);
    const loaded = eventPromise('busy-end'); // Emitted when page is loaded.
    await page.goto(`${base}/dist/list-page-sk.html${queryParams}`);
    await loaded;
}
