import { expect } from 'chai';
import { ElementHandle } from 'puppeteer';
import { loadCachedTestBed, takeScreenshot, TestBed } from '../../../puppeteer-tests/util';
import { PivotTableSkPO } from './pivot-table-sk_po';

describe('pivot-table-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  let pivotTableSk: ElementHandle;
  let pivotTableSkPO: PivotTableSkPO;

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport({ width: 600, height: 600 });
    pivotTableSk = (await testBed.page.$('pivot-table-sk'))!;
    pivotTableSkPO = new PivotTableSkPO(pivotTableSk);
  });

  it('should render the demo page (smoke test)', async () => {
    expect(await testBed.page.$$('pivot-table-sk')).to.have.length(3);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'perf', 'pivot-table-sk');
    });
  });

  describe('sorting', () => {
    it('sorts when a key column header is clicked', async () => {
      // Sort ascending by the second column ('arch', index 1).
      await pivotTableSkPO.clickSortIcon(1);
      const ascValues = await pivotTableSkPO.getColumnValues(1);

      await takeScreenshot(testBed.page, 'perf', 'pivot-table-sk-sort-asc');

      // Sort descending by the second column.
      await pivotTableSkPO.clickSortIcon(1);
      const descValues = await pivotTableSkPO.getColumnValues(1);

      await takeScreenshot(testBed.page, 'perf', 'pivot-table-sk-sort-desc');

      // The descending list of strings should be the reverse of ascending.
      expect(descValues).to.deep.equal([...ascValues].reverse());
      if (ascValues.length > 1 && ascValues[0] !== ascValues[1]) {
        expect(descValues).to.not.deep.equal(ascValues);
      }
    });
  });
});
