import { expect } from 'chai';
import { loadCachedTestBed, takeScreenshot, TestBed } from '../../../puppeteer-tests/util';

describe('status-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport({ width: 1600, height: 1000 });
  });

  it('should render the demo page (smoke test)', async () => {
    expect(await testBed.page.$$('status-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'status', 'status-sk');
    });
    it('changes repos', async () => {
      await testBed.page.select('#repoSelector', 'infra');
      await takeScreenshot(testBed.page, 'status', 'status-sk_repo_change');
    });
  });
});
