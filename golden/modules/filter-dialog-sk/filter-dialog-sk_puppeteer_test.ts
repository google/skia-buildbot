import { expect } from 'chai';
import { takeScreenshot, TestBed } from '../../../puppeteer-tests/util';
import { loadGoldWebpack } from '../common_puppeteer_test/common_puppeteer_test';
import { FilterDialogSkPO } from './filter-dialog-sk_po';
import { PageObjectElement } from '../../../infra-sk/modules/page_object/page_object_element';

describe('filter-dialog-sk', () => {
  let testBed: TestBed;
  let filterDialogSkPO: FilterDialogSkPO;

  before(async () => {
    testBed = await loadGoldWebpack();
  });

  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/dist/filter-dialog-sk.html`);
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
    let rangeInput: PageObjectElement;
    let numberInput: PageObjectElement;

    beforeEach(async () => {
      await openDialog();

      const numericParamPO = await filterDialogSkPO.getMinRGBADeltaPO();
      rangeInput = await numericParamPO.getRangeInput();
      numberInput = await numericParamPO.getNumberInput();
    });

    it('initially shows the same value on both inputs', async () => {
      expect(await rangeInput.value).to.equal('0');
      expect(await numberInput.value).to.equal('0');
    });

    it('updates number input when range input changes', async () => {
      await rangeInput.focus();

      await testBed.page.keyboard.down('ArrowRight');
      expect(await rangeInput.value).to.equal('1');
      expect(await numberInput.value).to.equal('1');

      await testBed.page.keyboard.down('ArrowLeft');
      expect(await rangeInput.value).to.equal('0');
      expect(await numberInput.value).to.equal('0');
    });

    it('updates range input when number input changes', async () => {
      await numberInput.focus();

      await testBed.page.keyboard.down('ArrowUp');
      expect(await rangeInput.value).to.equal('1');
      expect(await numberInput.value).to.equal('1');

      await testBed.page.keyboard.down('ArrowDown');
      expect(await rangeInput.value).to.equal('0');
      expect(await numberInput.value).to.equal('0');
    });
  });

  describe('screenshots', () => {
    it('should take a screenshot', async () => {
      await openDialog();
      await takeScreenshot(testBed.page, 'gold', 'filter-dialog-sk');
    });

    it('should take a screenshot with the query dialog visible', async () => {
      await openDialog();
      await (await filterDialogSkPO.getTraceFilterSkPO()).clickEditBtn();
      await takeScreenshot(testBed.page, 'gold', 'filter-dialog-sk_query-dialog-open');
    });
  });

  const openDialog = async () => await testBed.page.click('#show-dialog');
});
