import { expect, assert } from 'chai';
import { loadCachedTestBed, takeScreenshot, TestBed } from '../../../puppeteer-tests/util';
import { GraphTitleSkPO } from './graph-title-sk_po';
import { defaultEntries, longTitleEntries } from './test_data';
import { GraphTitleSk } from './graph-title-sk';

describe('graph-title-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  let defaultPO: GraphTitleSkPO;
  let longTitlePO: GraphTitleSkPO;
  let multiTracePO: GraphTitleSkPO;
  let noTracesPO: GraphTitleSkPO;

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport({ width: 800, height: 600 });

    // Setup the elements with test data.
    await testBed.page.evaluate(
      (de, lte) => {
        (document.querySelector('#good') as GraphTitleSk).set(new Map(de), 1);
        (document.querySelector('#partial') as GraphTitleSk).set(new Map(lte), 1);
        (document.querySelector('#generic') as GraphTitleSk).set(new Map(), 5);
        (document.querySelector('#empty') as GraphTitleSk).set(new Map(de), 0);
      },
      Array.from(defaultEntries.entries()),
      Array.from(longTitleEntries.entries())
    );

    defaultPO = new GraphTitleSkPO((await testBed.page.$('graph-title-sk#good'))!);
    longTitlePO = new GraphTitleSkPO((await testBed.page.$('graph-title-sk#partial'))!);
    multiTracePO = new GraphTitleSkPO((await testBed.page.$('graph-title-sk#generic'))!);
    noTracesPO = new GraphTitleSkPO((await testBed.page.$('graph-title-sk#empty'))!);
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('graph-title-sk')).to.have.length(4);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'perf', 'graph-title-sk');
    });

    it('shows the long title collapsed', async () => {
      const element = await testBed.page.$('#partial');
      await takeScreenshot(element!, 'perf', 'graph-title-sk-long-collapsed');
    });

    it('shows the long title expanded', async () => {
      await longTitlePO.clickShowMore();
      const element = await testBed.page.$('#partial');
      await takeScreenshot(element!, 'perf', 'graph-title-sk-long-expanded');
    });
  });

  describe('default title', () => {
    it('is not hidden', async () => {
      assert.isFalse(await defaultPO.isHidden());
    });

    it('shows the correct params', async () => {
      expect(await defaultPO.getParamCount()).to.equal(3); // one value is empty, so it's skipped
      expect(await defaultPO.getParamName(0)).to.equal('bot');
      expect(await defaultPO.getParamValue(0)).to.equal('linux-perf');
      expect(await defaultPO.getParamName(1)).to.equal('benchmark');
      expect(await defaultPO.getParamValue(1)).to.equal('Speedometer2');
      expect(await defaultPO.getParamName(2)).to.equal('subtest_1');
      expect(await defaultPO.getParamValue(2)).to.equal('100_objects_allocated_at_initialization');
    });

    it('does not have a show more button', async () => {
      assert.isFalse(await defaultPO.isShowMoreVisible());
    });
  });

  describe('long title', () => {
    it('is not hidden', async () => {
      assert.isFalse(await longTitlePO.isHidden());
    });

    it('has a show more button', async () => {
      assert.isTrue(await longTitlePO.isShowMoreVisible());
    });

    it('initially shows a truncated list of params', async () => {
      expect(await longTitlePO.getParamCount()).to.equal(8);
    });

    it('shows all params after clicking show more', async () => {
      await longTitlePO.clickShowMore();
      expect(await longTitlePO.getParamCount()).to.equal(9);
      assert.isFalse(await longTitlePO.isShowMoreVisible());
    });
  });

  describe('multi trace title', () => {
    it('is not hidden', async () => {
      assert.isFalse(await multiTracePO.isHidden());
    });
    it('shows the correct title', async () => {
      expect(await multiTracePO.getTitleText()).to.equal('Multi-trace Graph (5 traces)');
    });
  });

  describe('no traces title', () => {
    it('is hidden', async () => {
      assert.isTrue(await noTracesPO.isHidden());
    });
  });
});
