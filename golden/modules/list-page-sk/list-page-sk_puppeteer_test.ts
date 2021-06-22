import { expect } from 'chai';
import {
    addEventListenersToPuppeteerPage,
    loadCachedTestBed,
    takeScreenshot,
    TestBed
} from '../../../puppeteer-tests/util';
import { Page } from 'puppeteer';
import path from 'path';

describe('list-page-sk', () => {
    let testBed: TestBed;
    before(async () => {
        testBed = await loadCachedTestBed(
            path.join(__dirname, '..', '..', 'webpack.config.ts')
        );
    });

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

    it('has the checkboxes respond to forward and back browser buttons', async() => {
        await navigateTo(testBed.page, testBed.baseUrl,'?corpus=gm');

        await expectCheckBoxSkToBe('checkbox-sk.ignore_rules', false);
        expect(testBed.page.url()).to.contain('?corpus=gm');

        // click on ignore rules checkbox
        await testBed.page.click('checkbox-sk.ignore_rules');

        await expectCheckBoxSkToBe('checkbox-sk.ignore_rules', true);
        expect(testBed.page.url()).to.contain('?corpus=gm&disregard_ignores=true');

        await testBed.page.goBack();

        await expectCheckBoxSkToBe('checkbox-sk.ignore_rules', false);
        expect(testBed.page.url()).to.contain('?corpus=gm');

        await testBed.page.goForward();

        await expectCheckBoxSkToBe('checkbox-sk.ignore_rules', true);
        expect(testBed.page.url()).to.contain('?corpus=gm&disregard_ignores=true');
    });

    it('has the corpus respond to forward and back browser buttons', async() => {
        await navigateTo(testBed.page, testBed.baseUrl,'?corpus=gm&query=alpha_type%3DOpaque');

        await expectSelectedCorpusToBe('gm');

        // click on colorImage corpus
        await testBed.page.click('corpus-selector-sk > ul > li:nth-child(1)');

        expect(testBed.page.url()).to.contain('?corpus=colorImage&query=alpha_type%3DOpaque');
        await expectSelectedCorpusToBe('colorImage');

        await testBed.page.goBack();

        expect(testBed.page.url()).to.contain('?corpus=gm&query=alpha_type%3DOpaque');
        await expectSelectedCorpusToBe('gm');

        await testBed.page.goForward();

        expect(testBed.page.url()).to.contain('?corpus=colorImage&query=alpha_type%3DOpaque');
        await expectSelectedCorpusToBe('colorImage');
    });

    it('has the corpus respond to forward and back browser buttons', async() => {
        await navigateTo(testBed.page, testBed.baseUrl,'?corpus=gm&query=alpha_type%3DOpaque');

        await expectDisplayedFilterToBe('source_type=gm, \nalpha_type=Opaque');

        // show dialog
        await testBed.page.click('button.show_query_dialog');
        // clear selected key/values
        await testBed.page.click('query-sk button.clear_selections');
        // apply that clear action.
        await testBed.page.click('query-dialog-sk button.show-matches');

        await expectDisplayedFilterToBe('source_type=gm');

        await testBed.page.goBack();

        await expectDisplayedFilterToBe('source_type=gm, \nalpha_type=Opaque');

        await testBed.page.goForward();

        await expectDisplayedFilterToBe('source_type=gm');
    });

    const expectCheckBoxSkToBe = async (selector: string, expectedToBeChecked: boolean) => {
        const isChecked =
            await testBed.page.$eval(
                selector,
                (e: Element) => (e as HTMLElement).hasAttribute('checked'));
        expect(isChecked).to.equal(expectedToBeChecked);
    };

    const expectSelectedCorpusToBe = async (corpus: string) => {
        const selectedTitle =
            await testBed.page.$eval(
                'corpus-selector-sk li.selected',
                (e: Element) => (e as HTMLLIElement).innerText);
        expect(selectedTitle).to.contain(corpus);
    };

    const expectDisplayedFilterToBe = async (filter: string) => {
        const displayedFilter =
            await testBed.page.$eval(
                '.query_params pre',
                (e: Element) => (e as HTMLPreElement).innerText);
        expect(displayedFilter).to.equal(filter);
    };
});

async function navigateTo(page: Page, base: string, queryParams = '') {
    const eventPromise = await addEventListenersToPuppeteerPage(page, ['busy-end']);
    const loaded = eventPromise('busy-end'); // Emitted when page is loaded.
    await page.goto(`${base}/dist/list-page-sk.html${queryParams}`);
    await loaded;
}
