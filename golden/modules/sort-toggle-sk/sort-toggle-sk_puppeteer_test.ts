import { expect } from 'chai';
import {
    addEventListenersToPuppeteerPage, EventName, loadCachedTestBed,
    takeScreenshot, TestBed
} from '../../../puppeteer-tests/util';
import { ElementHandle } from 'puppeteer';
import path from "path";

describe('sort-toggle-sk', () => {
    let testBed: TestBed;
    before(async () => {
        testBed = await loadCachedTestBed(
            path.join(__dirname, '..', '..', 'webpack.config.ts')
        );
    });

    let promiseFactory: <T>(eventName: EventName) => Promise<T>;
    let sortToggleSk: ElementHandle;

    beforeEach(async () => {
        promiseFactory = await addEventListenersToPuppeteerPage(testBed.page,
            ['sort-changed']);
        const loaded = promiseFactory('sort-changed'); // Emitted when sorted.
        await testBed.page.goto(`${testBed.baseUrl}/dist/sort-toggle-sk.html`);
        await loaded;
        sortToggleSk = (await testBed.page.$('#container sort-toggle-sk'))!;
    });

    it('should render the demo page', async () => {
        // Smoke test.
        expect(await testBed.page.$$('sort-toggle-sk')).to.have.length(1);
    });

    it('should respect the default sort order', async () => {
        await expectSortOrderToMatch(['alfa', 'bravo', 'charlie', 'delta']);
        await takeScreenshot(sortToggleSk, 'gold', 'sort-toggle-sk_sort-alpha-ascending');
    });

    it('can sort alphabetically in descending order', async () => {
        await clickSortHeader('name');

        await expectSortOrderToMatch(['delta', 'charlie', 'bravo', 'alfa']);
        await takeScreenshot(sortToggleSk, 'gold', 'sort-toggle-sk_sort-alpha-descending');
    });

    it('it can sort by numeric values in descending order', async () => {
        await clickSortHeader('weight');

        await expectSortOrderToMatch(['charlie', 'bravo', 'alfa', 'delta']);
        await takeScreenshot(sortToggleSk, 'gold', 'sort-toggle-sk_sort-numeric-descending');
    });

    it('it can sort by numeric values in ascending order', async () => {
        await clickSortHeader('weight'); // first in descending order
        await clickSortHeader('weight'); // then should toggle to be in ascending order

        await expectSortOrderToMatch(['delta', 'alfa', 'bravo', 'charlie']);
        await takeScreenshot(sortToggleSk, 'gold', 'sort-toggle-sk_sort-numeric-ascending');
    });

    async function expectSortOrderToMatch(names: string[]) {
        const nameOrder = await sortToggleSk.$$eval('tbody tr td:first-child',
            (tds: Element[]) => tds.map(td => td.textContent));
        expect(names).to.deep.equal(nameOrder);
    }

    async function clickSortHeader(key: string) {
        const sortEvent = promiseFactory('sort-changed');
        const header = await sortToggleSk.$(`th[data-key="${key}"]`);
        await header!.click();
        await sortEvent;
    }
});
