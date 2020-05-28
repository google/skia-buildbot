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

    it('should have a different URL for CL commits', async () => {
      const masterBranchURL = await testBed.page.$eval(
          '#single_commit a',
          (e: Element) => (e as HTMLAnchorElement).href);
      expect(masterBranchURL).to.equal(
          'https://github.com/example/example/commit/dded3c7506efc5635e60ffb7a908cbe8f1f028f1');

      const changeListURL = await testBed.page.$eval(
          '#single_cl_commit a',
          (e: Element) => (e as HTMLAnchorElement).href);
      expect(changeListURL).to.equal('https://skia-review.googlesource.com/12345');
    });

    it('should show some commits commit', async () => {
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
});
