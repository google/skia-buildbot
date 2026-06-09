import { expect } from 'chai';
import { loadCachedTestBed, takeScreenshot, TestBed } from '../../../puppeteer-tests/util';
import { QuerySkPO } from '../../../infra-sk/modules/query-sk/query-sk_po';

describe('query-chooser-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport({ width: 400, height: 1500 });
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('query-chooser-sk')).to.have.length(3);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'perf', 'query-chooser-sk');
    });
  });

  describe('user interactions', () => {
    it('merges values for existing keys', async () => {
      const queryChooserSk = (await testBed.page.$('query-chooser-sk'))!;

      // Set an initial query.
      await queryChooserSk.evaluate((el: any) => {
        el.current_query = 'type=CPU';
      });

      // Open the edit dialog.
      const editButton = (await queryChooserSk.$('button'))!;
      await editButton.click();

      // Now select another value for the same key via the UI.
      const querySk = (await queryChooserSk.$('query-sk'))!;
      const querySkPO = new QuerySkPO(querySk);
      await querySkPO.clickKey('type');
      await querySkPO.clickValue('GPU');

      // The new value should be added to the existing ones.
      const query = await queryChooserSk.evaluate((el: any) => el.current_query);
      expect(query).to.equal('type=CPU&type=GPU');
    });

    it('removes a value from the query', async () => {
      const queryChooserSk = (await testBed.page.$('query-chooser-sk'))!;

      // Set an initial query with multiple values for a key.
      await queryChooserSk.evaluate((el: any) => {
        el.current_query = 'type=CPU&type=GPU';
      });

      // Wait for the remove button on the 'GPU' chip to appear and then click it.
      const removeButtonSelector = 'paramset-sk #type-GPU-remove';
      const removeButton = (await testBed.page.waitForSelector(removeButtonSelector))!;
      await removeButton.click();

      const query = await queryChooserSk.evaluate((el: any) => el.current_query);
      expect(query).to.equal('type=CPU');
    });
  });
});
