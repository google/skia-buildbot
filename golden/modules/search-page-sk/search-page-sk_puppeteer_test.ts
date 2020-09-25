import { expect } from 'chai';
import { addEventListenersToPuppeteerPage, EventName, takeScreenshot, TestBed } from '../../../puppeteer-tests/util';
import { loadGoldWebpack } from '../common_puppeteer_test/common_puppeteer_test';
import { SearchPageSkPO } from './search-page-sk_po';

describe('search-page-sk', () => {
  let testBed: TestBed;
  let eventPromiseFactory:  <T>(eventName: EventName) => Promise<T>;

  let searchPageSkPO: SearchPageSkPO;

  before(async () => {
    testBed = await loadGoldWebpack();
  });

  const goToPage = async (queryString = '') => {
    const busyEnd = eventPromiseFactory('busy-end');
    await testBed.page.goto(`${testBed.baseUrl}/dist/search-page-sk.html${queryString}`);
    await busyEnd;

    await testBed.page.setViewport({width: 1400, height: 1200});

    searchPageSkPO = new SearchPageSkPO((await testBed.page.$('search-page-sk'))!);
  };

  beforeEach(async () => {
    eventPromiseFactory = await addEventListenersToPuppeteerPage(testBed.page, ['busy-end']);
  });

  it('should render the demo page', async () => {
    // Smoke test.
    await goToPage();
    expect(await testBed.page.$$('search-page-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('shows an empty results page', async () => {
      await goToPage('?untriaged=false');
      await takeScreenshot(testBed.page, 'gold', 'search-page-sk_empty');
    });

    it('shows search results', async () => {
      await goToPage('?untriaged=true&positive=true&negative=true');
      await takeScreenshot(testBed.page, 'gold', 'search-page-sk');
    });

    it('shows changelist controls', async () => {
      await goToPage('?untriaged=true&positive=true&negative=true&crs=gerrit&issue=123456');
      await takeScreenshot(testBed.page, 'gold', 'search-page-sk_changelist-controls');
    });

    it('shows the bulk triage dialog', async () => {
      await goToPage('?untriaged=true&positive=true&negative=true');
      await searchPageSkPO.clickBulkTriageBtn();
      await takeScreenshot(testBed.page, 'gold', 'search-page-sk_bulk-triage');
    });

    it('shows the bulk triage dialog with a CL', async () => {
      await goToPage('?untriaged=true&positive=true&negative=true&crs=gerrit&issue=123456');
      await searchPageSkPO.clickBulkTriageBtn();
      await takeScreenshot(testBed.page, 'gold', 'search-page-sk_bulk-triage-with-cl');
    });

    it('shows the help dialog', async () => {
      await goToPage();
      await searchPageSkPO.clickHelpBtn();
      await takeScreenshot(testBed.page, 'gold', 'search-page-sk_help-dialog');
    });
  });

  it('reads search params from the URL', async () => {
    await goToPage('?untriaged=true&positive=true&negative=true');

    let searchControlsSkPO = await searchPageSkPO.getSearchControlsSkPO();
    expect(await searchControlsSkPO.isIncludeUntriagedDigestsCheckboxChecked()).to.be.true;
    expect(await searchControlsSkPO.isIncludePositiveDigestsCheckboxChecked()).to.be.true;
    expect(await searchControlsSkPO.isIncludeNegativeDigestsCheckboxChecked()).to.be.true;

    await goToPage('?untriaged=true&positive=false&negative=false');
    searchControlsSkPO = await searchPageSkPO.getSearchControlsSkPO();
    expect(await searchControlsSkPO.isIncludeUntriagedDigestsCheckboxChecked()).to.be.true;
    expect(await searchControlsSkPO.isIncludePositiveDigestsCheckboxChecked()).to.be.false;
    expect(await searchControlsSkPO.isIncludeNegativeDigestsCheckboxChecked()).to.be.false;
  });

  // TODO(lovisolo): Test this more thoroughly (exercise all search parameters, etc.).
  it('updates the URL whe the search controls change', async () => {
    await goToPage();

    const searchControlsSkPO = await searchPageSkPO.getSearchControlsSkPO();

    // "Positive" is initially unchecked.
    expect(testBed.page.url()).to.not.include('positive=true');

    // Check "Positive" checkbox.
    let busyEnd = eventPromiseFactory('busy-end');
    await searchControlsSkPO.clickIncludePositiveDigestsCheckbox();
    await busyEnd;
    expect(testBed.page.url()).to.include('positive=true');

    // Uncheck "Positive" checkbox.
    busyEnd = eventPromiseFactory('busy-end');
    await searchControlsSkPO.clickIncludePositiveDigestsCheckbox();
    await busyEnd;
    expect(testBed.page.url()).to.not.include('positive=true');
  });

  it('supports the browser back/forward buttons', async () => {
    await goToPage();

    const searchControlsSkPO = await searchPageSkPO.getSearchControlsSkPO();

    expect(testBed.page.url()).to.not.include('positive=true');
    expect(testBed.page.url()).to.not.include('negative=true');
    expect(await searchControlsSkPO.isIncludePositiveDigestsCheckboxChecked()).to.be.false;
    expect(await searchControlsSkPO.isIncludeNegativeDigestsCheckboxChecked()).to.be.false;

    // Check "Positive" checkbox.
    let busyEnd = eventPromiseFactory('busy-end');
    await searchControlsSkPO.clickIncludePositiveDigestsCheckbox();
    await busyEnd;
    expect(testBed.page.url()).to.include('positive=true');
    expect(testBed.page.url()).to.not.include('negative=true');
    expect(await searchControlsSkPO.isIncludePositiveDigestsCheckboxChecked()).to.be.true;
    expect(await searchControlsSkPO.isIncludeNegativeDigestsCheckboxChecked()).to.be.false;

    // Check "Negative" checkbox.
    busyEnd = eventPromiseFactory('busy-end');
    await searchControlsSkPO.clickIncludeNegativeDigestsCheckbox();
    await busyEnd;
    expect(testBed.page.url()).to.include('positive=true');
    expect(testBed.page.url()).to.include('negative=true');
    expect(await searchControlsSkPO.isIncludePositiveDigestsCheckboxChecked()).to.be.true;
    expect(await searchControlsSkPO.isIncludeNegativeDigestsCheckboxChecked()).to.be.true;

    // Go back.
    busyEnd = eventPromiseFactory('busy-end');
    await testBed.page.goBack();
    await busyEnd;
    expect(testBed.page.url()).to.include('positive=true');
    expect(testBed.page.url()).to.not.include('negative=true');
    expect(await searchControlsSkPO.isIncludePositiveDigestsCheckboxChecked()).to.be.true;
    expect(await searchControlsSkPO.isIncludeNegativeDigestsCheckboxChecked()).to.be.false;

    // Go back.
    busyEnd = eventPromiseFactory('busy-end');
    await testBed.page.goBack();
    await busyEnd;
    expect(testBed.page.url()).to.not.include('positive=true');
    expect(testBed.page.url()).to.not.include('negative=true');
    expect(await searchControlsSkPO.isIncludePositiveDigestsCheckboxChecked()).to.be.false;
    expect(await searchControlsSkPO.isIncludeNegativeDigestsCheckboxChecked()).to.be.false;

    // Go forward.
    busyEnd = eventPromiseFactory('busy-end');
    await testBed.page.goForward();
    await busyEnd;
    expect(testBed.page.url()).to.include('positive=true');
    expect(testBed.page.url()).to.not.include('negative=true');
    expect(await searchControlsSkPO.isIncludePositiveDigestsCheckboxChecked()).to.be.true;
    expect(await searchControlsSkPO.isIncludeNegativeDigestsCheckboxChecked()).to.be.false;

    // Go forward.
    busyEnd = eventPromiseFactory('busy-end');
    await testBed.page.goForward();
    await busyEnd;
    expect(testBed.page.url()).to.include('positive=true');
    expect(testBed.page.url()).to.include('negative=true');
    expect(await searchControlsSkPO.isIncludePositiveDigestsCheckboxChecked()).to.be.true;
    expect(await searchControlsSkPO.isIncludeNegativeDigestsCheckboxChecked()).to.be.true;
  });
});
