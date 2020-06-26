import { expect } from 'chai';
import { addEventListenersToPuppeteerPage, takeScreenshot, TestBed } from '../../../puppeteer-tests/util';
import { loadGoldWebpack } from '../common_puppeteer_test/common_puppeteer_test';
import { ElementHandle, Page } from 'puppeteer';

describe('details-page-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadGoldWebpack();
  });

  const baseParams = '?digest=6246b773851984c726cb2e1cb13510c2&test=My%20test%20has%20spaces';

  it('should render the demo page', async () => {
    await navigateTo(testBed.page, testBed.baseUrl, baseParams);
    // Smoke test.
    expect(await testBed.page.$$('details-page-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('should show the default page', async () => {
      await navigateTo(testBed.page, testBed.baseUrl, baseParams);
      await testBed.page.setViewport({
        width: 1300,
        height: 700,
      });
      await takeScreenshot(testBed.page, 'gold', 'details-page-sk');
    });

    it('should show the digest even if it is not in the index', async () => {
      await navigateTo(testBed.page, testBed.baseUrl, baseParams);
      await testBed.page.setViewport({
        width: 1300,
        height: 700,
      });
      await testBed.page.click('#simulate-not-found-in-index');
      await takeScreenshot(testBed.page, 'gold', 'details-page-sk_not-in-index');
    });

    it('should show a message if the backend had an error', async () => {
      await navigateTo(testBed.page, testBed.baseUrl, baseParams);
      await testBed.page.setViewport({
        width: 1300,
        height: 700,
      });
      await testBed.page.click('#simulate-rpc-error');
      await takeScreenshot(testBed.page, 'gold', 'details-page-sk_backend-error');
    });
  });

  describe('url params', () => {
    it('correctly extracts the grouping and digest', async () => {
      await navigateTo(testBed.page, testBed.baseUrl, baseParams);
      const detailsPageSk = await testBed.page.$('details-page-sk');

      expect(await getPropertyAsJSON(detailsPageSk!, '_grouping')).to.equal('My test has spaces');
      expect(await getPropertyAsJSON(detailsPageSk!, '_digest')).to.equal('6246b773851984c726cb2e1cb13510c2');
      expect(await getPropertyAsJSON(detailsPageSk!, '_changeListID')).to.equal('');
    });

    it('correctly extracts the changelistID (issue) if provided', async () => {
      await navigateTo(testBed.page, testBed.baseUrl, `${baseParams}&issue=65432`);
      const detailsPageSk = await testBed.page.$('details-page-sk');

      expect(await getPropertyAsJSON(detailsPageSk!, '_grouping')).to.equal('My test has spaces');
      expect(await getPropertyAsJSON(detailsPageSk!, '_digest')).to.equal('6246b773851984c726cb2e1cb13510c2');
      expect(await getPropertyAsJSON(detailsPageSk!, '_changeListID')).to.equal('65432');

      const digestDetails = await testBed.page.$('details-page-sk digest-details-sk');
      expect(await getPropertyAsJSON(digestDetails!, '_issue')).to.equal('65432');
    });
  });
});

async function navigateTo(page: Page, base: string, queryParams = '') {
  const eventPromise = await addEventListenersToPuppeteerPage(page, ['busy-end']);
  const loaded = eventPromise('busy-end'); // Emitted from gold-scaffold when page is loaded.
  await page.goto(`${base}/dist/details-page-sk.html${queryParams}`);
  await loaded;
}

async function getPropertyAsJSON(ele: ElementHandle, propName: string) {
  const prop = await ele.getProperty(propName);
  return prop.jsonValue();
}
