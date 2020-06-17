import * as path from 'path';
import { expect } from 'chai';
import { setUpPuppeteerAndDemoPageServer, takeScreenshot } from '../../../puppeteer-tests/util';

describe('blamelist-panel-sk', () => {
  // Contains page and baseUrl.
  const testBed = setUpPuppeteerAndDemoPageServer(path.join(__dirname, '..', '..', 'webpack.config.ts'));

  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/dist/blamelist-panel-sk.html`);
  });

  it('should render the demo page', async () => {
    expect(await testBed.page.$$('blamelist-panel-sk')).to.have.length(4); // Smoke test.
  });

  describe('screenshots', async () => {
    it('should show a single commit', async () => {
      const blamelistPanelSk = await testBed.page.$('#single_commit');
      await takeScreenshot(blamelistPanelSk!, 'gold', 'blamelist-panel-sk');
      expect(await testBed.page.$$('#single_commit tr')).to.have.length(1);
    });

    it('should show a single CL commit', async () => {
      const blamelistPanelSk = await testBed.page.$('#single_cl_commit');
      await takeScreenshot(blamelistPanelSk!, 'gold', 'blamelist-panel-sk_cl-commit');
      expect(await testBed.page.$$('#single_cl_commit tr')).to.have.length(1);
    });

    it('should show some commits', async () => {
      const blamelistPanelSk = await testBed.page.$('#some_commits');
      await takeScreenshot(blamelistPanelSk!, 'gold', 'blamelist-panel-sk_some-commits');
      expect(await testBed.page.$$('#some_commits tr')).to.have.length(3);
    });

    it('should truncate many commits', async () => {
      const blamelistPanelSk = await testBed.page.$('#many_commits');
      await takeScreenshot(blamelistPanelSk!, 'gold', 'blamelist-panel-sk_many-commits');
      expect(await testBed.page.$$('#many_commits tr')).to.have.length(15); // maxCommitsToDisplay
    });

  });

  describe('urls', async() => {
    it('should have a different URL for CL commits', async () => {
      const masterBranchURL = await testBed.page.$eval(
          '#single_commit table a',
          (e: Element) => (e as HTMLAnchorElement).href);
      expect(masterBranchURL).to.equal(
          'https://github.com/example/example/commit/dded3c7506efc5635e60ffb7a908cbe8f1f028f1');

      const changeListURL = await testBed.page.$eval(
          '#single_cl_commit table a',
          (e: Element) => (e as HTMLAnchorElement).href);
      expect(changeListURL).to.equal('https://skia-review.googlesource.com/12345');
    });

    it('should have a link to the full source blamelist', async() => {
      const manyCommitsURL = await testBed.page.$eval(
          '#many_commits .full_range a',
          (e: Element) => (e as HTMLAnchorElement).href);
      expect(manyCommitsURL).to.equal(
          'https://github.com/example/example/compare/667edf14ad72966ec36aa6cd705b98cb7d7eee28...dded3c7506efc5635e60ffb7a908cbe8f1f028f1');

      const someCommitsURL = await testBed.page.$eval(
          '#some_commits .full_range a',
          (e: Element) => (e as HTMLAnchorElement).href);
      expect(someCommitsURL).to.equal(
          'https://github.com/example/example/compare/9145f784f3261f227846e5b08dc2691a888b113c...dded3c7506efc5635e60ffb7a908cbe8f1f028f1');


    });
  });


});
