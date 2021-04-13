import { expect } from 'chai';
import {
  addEventListenersToPuppeteerPage,
  loadCachedTestBed,
  takeScreenshot,
  TestBed
} from '../../../puppeteer-tests/util';
import { ElementHandle, Page } from 'puppeteer';
import path from "path";
describe('diff-page-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed(
        path.join(__dirname, '..', '..', 'webpack.config.ts')
    );
  });

  const baseParams = '?left=6246b773851984c726cb2e1cb13510c2&right=99c58c7002073346ff55f446d47d6311&test=My%20test%20has%20spaces';

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
      await navigateTo(testBed.page, testBed.baseUrl, baseParams);
      const diffPageSk = await testBed.page.$('diff-page-sk');

      expect(await getPropertyAsJSON(diffPageSk!, '_grouping')).to.equal('My test has spaces');
      expect(await getPropertyAsJSON(diffPageSk!, '_leftDigest')).to.equal('6246b773851984c726cb2e1cb13510c2');
      expect(await getPropertyAsJSON(diffPageSk!, '_rightDigest')).to.equal('99c58c7002073346ff55f446d47d6311');
      expect(await getPropertyAsJSON(diffPageSk!, '_changeListID')).to.equal('');
      expect(await getPropertyAsJSON(diffPageSk!, '_crs')).to.equal('');
    });

    it('correctly extracts the changelistIDif provided', async () => {
      await navigateTo(testBed.page, testBed.baseUrl, `${baseParams}&changelist_id=65432&crs=gerrit`);
      const diffPageSk = await testBed.page.$('diff-page-sk');

      expect(await getPropertyAsJSON(diffPageSk!, '_grouping')).to.equal('My test has spaces');
      expect(await getPropertyAsJSON(diffPageSk!, '_leftDigest')).to.equal('6246b773851984c726cb2e1cb13510c2');
      expect(await getPropertyAsJSON(diffPageSk!, '_rightDigest')).to.equal('99c58c7002073346ff55f446d47d6311');
      expect(await getPropertyAsJSON(diffPageSk!, '_changeListID')).to.equal('65432');
      expect(await getPropertyAsJSON(diffPageSk!, '_crs')).to.equal('gerrit');

      const digestDetails = await testBed.page.$('diff-page-sk digest-details-sk');
      expect(await getPropertyAsJSON(digestDetails!, 'changeListID')).to.equal('65432');
      expect(await getPropertyAsJSON(digestDetails!, 'crs')).to.equal('gerrit');
    });
  });
});

async function navigateTo(page: Page, base: string, queryParams = '') {
  const eventPromise = await addEventListenersToPuppeteerPage(page, ['busy-end']);
  const loaded = eventPromise('busy-end'); // Emitted from gold-scaffold when page is loaded.
  await page.goto(`${base}/dist/diff-page-sk.html${queryParams}`);
  await loaded;
}

async function getPropertyAsJSON(ele: ElementHandle, propName: string) {
  const prop = await ele.getProperty(propName);
  return prop!.jsonValue();
}
