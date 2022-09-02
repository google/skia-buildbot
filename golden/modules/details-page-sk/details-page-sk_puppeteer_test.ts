import { expect } from 'chai';
import { ElementHandle, Page } from 'puppeteer';
import {
  addEventListenersToPuppeteerPage,
  loadCachedTestBed,
  takeScreenshot,
  TestBed,
} from '../../../puppeteer-tests/util';
import { DetailsPageSkPO } from './details-page-sk_po';

describe('details-page-sk', () => {
  let testBed: TestBed;

  before(async () => {
    testBed = await loadCachedTestBed();
  });

  // The demo page pushes a "default" query string to the URL when no query string is provided. All
  // parameters in the below query string are different from said defaults. This allows this test
  // to verify that the page correctly parses out the query parameters.
  const baseParams = '?digest=99c58c7002073346ff55f446d47d6311&test=My%20test%20has%20spaces';

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
      const detailsPageSkPO = await navigateTo(testBed.page, testBed.baseUrl, baseParams);

      expect(await detailsPageSkPO.digestDetailsSkPO.getTestName())
        .to.equal('Test: My test has spaces');
      expect(await detailsPageSkPO.digestDetailsSkPO.getLeftDigest())
        .to.equal('Left: 99c58c7002073346ff55f446d47d6311');
      expect(await detailsPageSkPO.digestDetailsSkPO.getRightDigest())
        .to.equal('Right: 6246b773851984c726cb2e1cb13510c2');

      // This link should not have a changelist ID or CRS.
      expect(await detailsPageSkPO.digestDetailsSkPO.getDiffPageLink()).to.equal(
        '/diff?grouping=name%3DMy%2520test%2520has%2520spaces%26source_type%3Dinfra'
          + '&left=99c58c7002073346ff55f446d47d6311&right=6246b773851984c726cb2e1cb13510c2',
      );
    });

    it('correctly extracts the changelist ID and CRS if provided', async () => {
      const detailsPageSkPO = await navigateTo(
        testBed.page,
        testBed.baseUrl,
        `${baseParams}&changelist_id=65432&crs=gerrit-internal`,
      );

      expect(await detailsPageSkPO.digestDetailsSkPO.getTestName())
        .to.equal('Test: My test has spaces');
      expect(await detailsPageSkPO.digestDetailsSkPO.getLeftDigest())
        .to.equal('Left: 99c58c7002073346ff55f446d47d6311');
      expect(await detailsPageSkPO.digestDetailsSkPO.getRightDigest())
        .to.equal('Right: 6246b773851984c726cb2e1cb13510c2');

      // The changelist ID and CRS should be reflected in this link.
      expect(await detailsPageSkPO.digestDetailsSkPO.getDiffPageLink()).to.equal(
        '/diff?grouping=name%3DMy%2520test%2520has%2520spaces%26source_type%3Dinfra'
          + '&left=99c58c7002073346ff55f446d47d6311&right=6246b773851984c726cb2e1cb13510c2'
          + '&changelist_id=65432&crs=gerrit-internal',
      );
    });
  });
});

async function navigateTo(page: Page, base: string, queryParams = ''): Promise<DetailsPageSkPO> {
  const eventPromise = await addEventListenersToPuppeteerPage(page, ['busy-end']);
  const loaded = eventPromise('busy-end'); // Emitted from gold-scaffold when page is loaded.
  await page.goto(`${base}${queryParams}`);
  await loaded;
  return new DetailsPageSkPO(page.$('details-page-sk'));
}

async function getPropertyAsJSON(ele: ElementHandle, propName: string) {
  const prop = await ele.getProperty(propName);
  return prop!.jsonValue();
}
