import { expect } from 'chai';
import { loadCachedTestBed, takeScreenshot, TestBed } from '../../../puppeteer-tests/util';

describe('pivot-query-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport({ width: 800, height: 600 });
  });

  it('should render the demo page (smoke test)', async () => {
    expect(await testBed.page.$$('pivot-query-sk')).to.have.length(1);
  });

  it('has unique IDs across multiple instances', async () => {
    // Add a second instance
    await testBed.page.evaluate(() => {
      const second = document.createElement('pivot-query-sk');
      document.body.appendChild(second);
    });

    // Get IDs of group_by elements
    const ids = await testBed.page.$$eval('multi-select-sk[id^="group_by-"]', (els) =>
      els.map((e) => e.id)
    );
    expect(ids).to.have.length(2);
    expect(ids[0]).to.not.equal(ids[1]);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'perf', 'pivot-query-sk');
    });
  });
});
