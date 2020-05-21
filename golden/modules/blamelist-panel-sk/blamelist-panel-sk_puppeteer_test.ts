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
    expect(await testBed.page.$$('blamelist-panel-sk')).to.have.length(3); // Smoke test.
  });

  describe('screenshots', async () => {
    it('should show a single commit', async () => {
      const blamelistPanelSk = await testBed.page.$('#single_commit');
      await takeScreenshot(blamelistPanelSk!, 'gold', 'blamelist-panel-sk');
      expect(await testBed.page.$$('#single_commit tr')).to.have.length(1);
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
