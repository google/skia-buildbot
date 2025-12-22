import { expect, assert } from 'chai';
import { loadCachedTestBed, takeScreenshot, TestBed } from '../../../puppeteer-tests/util';
import { CommitDetailPanelSkPO } from './commit-detail-panel-sk_po';
import { twoCommits } from './test_data';
import { CommitDetailPanelSkCommitSelectedDetails } from './commit-detail-panel-sk';
import { DEFAULT_VIEWPORT } from '../common/puppeteer-test-util';

// Helper function to await the 'commit-selected' event.
const awaitCommitSelectedEvent = async (page: any): Promise<any> => {
  return page.evaluate(async () => {
    return await new Promise((resolve) => {
      document.addEventListener(
        'commit-selected',
        (e) => {
          resolve((e as CustomEvent).detail);
        },
        { once: true }
      );
    });
  });
};

describe('commit-detail-panel-sk', () => {
  let testBed: TestBed;

  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl, { waitUntil: 'networkidle0' });
    await testBed.page.setViewport(DEFAULT_VIEWPORT);
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('commit-detail-panel-sk')).to.have.length(4);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      const commitDetailPanelSkPO = new CommitDetailPanelSkPO(
        (await testBed.page.$('commit-detail-panel-sk:nth-of-type(1)'))!
      );
      await commitDetailPanelSkPO.setDetails(twoCommits);
      expect(await commitDetailPanelSkPO.getRowCount()).to.equal(twoCommits.length);
    });
  });

  describe('interaction', () => {
    it('selects a commit on click', async () => {
      const commitDetailPanelSkPO = new CommitDetailPanelSkPO(
        (await testBed.page.$('commit-detail-panel-sk:nth-of-type(2)'))!
      );
      await commitDetailPanelSkPO.setDetails(twoCommits);

      // Get the promise BEFORE the click.
      const commitSelectedEvent = awaitCommitSelectedEvent(testBed.page);

      // Now click, which will trigger the event.
      await commitDetailPanelSkPO.clickRow(1);

      // Now wait for the event to be processed.
      const detail = (await commitSelectedEvent) as CommitDetailPanelSkCommitSelectedDetails;

      expect(detail.selected).to.equal(1);
      expect(detail.commit.hash).to.equal(twoCommits[1].hash);
      expect(await commitDetailPanelSkPO.getSelectedRow()).to.equal(1);
    });

    it('sets the selected commit', async () => {
      const commitDetailPanelSkPO = new CommitDetailPanelSkPO(
        (await testBed.page.$('commit-detail-panel-sk:nth-of-type(2)'))!
      );
      await commitDetailPanelSkPO.setDetails(twoCommits);
      await commitDetailPanelSkPO.setSelectedRow(0);
      expect(await commitDetailPanelSkPO.getSelectedRow()).to.equal(0);
    });

    it('is not selectable by default', async () => {
      const nonSelectablePO = new CommitDetailPanelSkPO(
        (await testBed.page.$('commit-detail-panel-sk:nth-of-type(1)'))!
      );
      await nonSelectablePO.setDetails(twoCommits);

      const commitSelectedEvent = awaitCommitSelectedEvent(testBed.page);

      await nonSelectablePO.clickRow(1);

      const winner = await Promise.race([
        commitSelectedEvent,
        new Promise((resolve) => setTimeout(() => resolve('timeout'), 1000)),
      ]);

      expect(winner).to.equal('timeout');
      expect(await nonSelectablePO.getSelectedRow()).to.equal(-1);
    });

    it('hides the table', async () => {
      const commitDetailPanelSkPO = new CommitDetailPanelSkPO(
        (await testBed.page.$('commit-detail-panel-sk:nth-of-type(1)'))!
      );
      await commitDetailPanelSkPO.setDetails(twoCommits);
      await commitDetailPanelSkPO.setHidden(true);
      assert.isTrue(await commitDetailPanelSkPO.isHidden());
      await takeScreenshot(testBed.page, 'perf', 'commit-detail-panel-sk-hidden');
    });
  });
});
