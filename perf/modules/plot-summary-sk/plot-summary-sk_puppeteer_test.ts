import { expect } from 'chai';
import {
  loadCachedTestBed,
  takeScreenshot,
  TestBed,
} from '../../../puppeteer-tests/util';

describe('plot-summary-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport({ width: 400, height: 550 });
  });

  it('should render the demo page (smoke test)', async () => {
    expect(await testBed.page.$$('plot-summary-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'perf', 'plot-summary-sk');
    });
    it('makes a selection on the graph', async () => {
      testBed.page.mouse.move(100, 20);
      testBed.page.mouse.down();
      testBed.page.mouse.move(500, 20);
      testBed.page.mouse.up();

      await takeScreenshot(testBed.page, 'perf', 'plot-summary-sk');
    });
  });
});
