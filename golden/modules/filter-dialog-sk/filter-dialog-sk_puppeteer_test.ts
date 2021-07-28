import { expect } from 'chai';
import {loadCachedTestBed, takeScreenshot, TestBed} from '../../../puppeteer-tests/util';
import { FilterDialogSkPO, NumericParamPO } from './filter-dialog-sk_po';

describe('filter-dialog-sk', () => {
  let filterDialogSkPO: FilterDialogSkPO;

  let testBed: TestBed;

  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport({width: 800, height: 800});
    filterDialogSkPO = new FilterDialogSkPO((await testBed.page.$('filter-dialog-sk'))!);
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('filter-dialog-sk')).to.have.length(1);
  });

  // We test the syncronizing behavior of the range/number input pairs using Puppeteer because
  // these inputs listen to each other's "input" event to keep their values in sync, and it is not
  // possible to realistically trigger said event from a Karma test.
  describe('numeric inputs (range/number input pairs)', () => {
    let numericParamPO: NumericParamPO;

    beforeEach(async () => {
      await openDialog();
      numericParamPO = await filterDialogSkPO.minRGBADeltaPO;
    });

    it('initially shows the same value on both inputs', async () => {
      expect(await numericParamPO.getRangeInputValue()).to.equal(0);
      expect(await numericParamPO.geNumberInputValue()).to.equal(0);
    });

    it('updates number input when range input changes', async () => {
      await numericParamPO.focusRangeInput();

      await testBed.page.keyboard.down('ArrowRight');
      expect(await numericParamPO.getRangeInputValue()).to.equal(1);
      expect(await numericParamPO.geNumberInputValue()).to.equal(1);

      await testBed.page.keyboard.down('ArrowLeft');
      expect(await numericParamPO.getRangeInputValue()).to.equal(0);
      expect(await numericParamPO.geNumberInputValue()).to.equal(0);
    });

    it('updates range input when number input changes', async () => {
      await numericParamPO.focusNumberInput();

      await testBed.page.keyboard.down('ArrowUp');
      expect(await numericParamPO.getRangeInputValue()).to.equal(1);
      expect(await numericParamPO.geNumberInputValue()).to.equal(1);

      await testBed.page.keyboard.down('ArrowDown');
      expect(await numericParamPO.getRangeInputValue()).to.equal(0);
      expect(await numericParamPO.geNumberInputValue()).to.equal(0);
    });
  });

  describe('screenshots', () => {
    it('should take a screenshot', async () => {
      await openDialog();
      await takeScreenshot(testBed.page, 'gold', 'filter-dialog-sk');
    });

    it('should take a screenshot with the query dialog visible', async () => {
      await openDialog();
      await (await filterDialogSkPO.traceFilterSkPO).clickEditBtn();
      await takeScreenshot(testBed.page, 'gold', 'filter-dialog-sk_query-dialog-open');
    });
  });

  const openDialog = async () => await testBed.page.click('#show-dialog');
});
