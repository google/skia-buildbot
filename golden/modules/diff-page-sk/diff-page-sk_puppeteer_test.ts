import { expect } from 'chai';
import {
  addEventListenersToPuppeteerPage, inBazel,
  loadCachedTestBed,
  takeScreenshot,
  TestBed
} from '../../../puppeteer-tests/util';
import { Page } from 'puppeteer';
import path from "path";
import { DiffPageSkPO } from './diff-page-sk_po';

describe('diff-page-sk', () => {
  let testBed: TestBed;

  before(async () => {
    testBed = await loadCachedTestBed(
        path.join(__dirname, '..', '..', 'webpack.config.ts')
    );
  });

  // The demo page pushes a "default" query string to the URL when no query string is provided. All
  // parameters in the below query string are different from said defaults. This allows this test
  // to verify that the page correctly parses out the query parameters.
  const baseParams =
      '?left=99c58c7002073346ff55f446d47d6311&right=6246b773851984c726cb2e1cb13510c2'
      + '&test=My%20test%20has%20spaces';

  it('should render the demo page', async () => {
    await navigateTo(testBed.page, testBed.baseUrl, baseParams);
    // Smoke test.
    expect(await testBed.page.$$('diff-page-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('should show the default page', async () => {
      await navigateTo(testBed.page, testBed.baseUrl, baseParams);
      await testBed.page.setViewport({
        width: 1300,
        height: 700,
      });
      await takeScreenshot(testBed.page, 'gold', 'diff-page-sk');
    });
  });

  describe('url params', () => {
    it('correctly extracts the grouping, left and right digest', async () => {
      const diffPageSkPO = await navigateTo(testBed.page, testBed.baseUrl, baseParams);

      expect(await diffPageSkPO.digestDetailsSkPO.getTestName())
          .to.equal('Test: My test has spaces');
      expect(await diffPageSkPO.digestDetailsSkPO.getLeftDigest())
          .to.equal('Left: 99c58c7002073346ff55f446d47d6311');
      expect(await diffPageSkPO.digestDetailsSkPO.getRightDigest())
          .to.equal('Right: 6246b773851984c726cb2e1cb13510c2');

      // This link should not have a changelist ID or CRS.
      expect(await diffPageSkPO.digestDetailsSkPO.getDiffPageLink()).to.equal(
          '/diff?test=My test has spaces'
              + '&left=99c58c7002073346ff55f446d47d6311&right=6246b773851984c726cb2e1cb13510c2');
    });

    it('correctly extracts the changelist ID and CRS if provided', async () => {
      const diffPageSkPO =
          await navigateTo(
              testBed.page, testBed.baseUrl, `${baseParams}&changelist_id=65432&crs=gerrit`);

      expect(await diffPageSkPO.digestDetailsSkPO.getTestName())
          .to.equal('Test: My test has spaces');
      expect(await diffPageSkPO.digestDetailsSkPO.getLeftDigest())
          .to.equal('Left: 99c58c7002073346ff55f446d47d6311');
      expect(await diffPageSkPO.digestDetailsSkPO.getRightDigest())
          .to.equal('Right: 6246b773851984c726cb2e1cb13510c2');

      // The changelist ID and CRS should be reflected in this link.
      expect(await diffPageSkPO.digestDetailsSkPO.getDiffPageLink()).to.equal(
          '/diff?test=My test has spaces'
          + '&left=99c58c7002073346ff55f446d47d6311&right=6246b773851984c726cb2e1cb13510c2'
          + '&changelist_id=65432&crs=gerrit');
    });
  });
});

async function navigateTo(page: Page, base: string, queryParams = ''): Promise<DiffPageSkPO> {
  const eventPromise = await addEventListenersToPuppeteerPage(page, ['busy-end']);
  const loaded = eventPromise('busy-end'); // Emitted from gold-scaffold when page is loaded.
  await page.goto(
      inBazel() ? `${base}${queryParams}` : `${base}/dist/diff-page-sk.html${queryParams}`);
  await loaded;
  return new DiffPageSkPO(page.$('diff-page-sk'));
}

