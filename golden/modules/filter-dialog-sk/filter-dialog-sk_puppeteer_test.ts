import * as path from 'path';
import { expect } from 'chai';
import { setUpPuppeteerAndDemoPageServer, takeScreenshot } from '../../../puppeteer-tests/util';
import { ElementHandle } from 'puppeteer';

describe('filter-dialog-sk', () => {
  // Contains page and baseUrl.
  const testBed =
    setUpPuppeteerAndDemoPageServer(path.join(__dirname, '..', '..', 'webpack.config.ts'));

  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/dist/filter-dialog-sk.html`);
    await testBed.page.setViewport({width: 800, height: 800});
  });

  // We test the syncronizing behavior of the range/number input pairs using Puppeteer because
  // these inputs listen to each other's "input" event to keep their values in sync, and it is not
  // possible to realistically trigger said event from a Karma test.
  describe('numeric inputs (range/number input pairs)', () => {
    let range: ElementHandle; // Handle for the <input type="range"> element.
    let number: ElementHandle; // Handle for the <input type="number"> element.

    beforeEach(async () => {
      range = (await testBed.page.$('#min-rgba-delta-numeric-param input[type="range"]'))!;
      number = (await testBed.page.$('#min-rgba-delta-numeric-param input[type="number"]'))!;
      await testBed.page.click('#show-dialog'); // Open dialog.
    });

    it('initially shows the same value on both inputs', async () => {
      await expectBothInputValuesToEqual('0');
    });

    it('updates number input when range input changes', async () => {
      await range.focus();
      await testBed.page.keyboard.down('ArrowRight');
      await expectBothInputValuesToEqual('1');
      await testBed.page.keyboard.down('ArrowLeft');
      await expectBothInputValuesToEqual('0')
    });

    it('updates range input when number input changes', async () => {
      await number.focus();
      await testBed.page.keyboard.down('ArrowUp');
      await expectBothInputValuesToEqual('1')
      await testBed.page.keyboard.down('ArrowDown');
      await expectBothInputValuesToEqual('0')
    });

    const expectBothInputValuesToEqual = async (value: string) => {
      expect(await range.evaluate(i => (i as HTMLInputElement).value)).to.equal(value);
      expect(await number.evaluate(i => (i as HTMLInputElement).value)).to.equal(value);
    };
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('filter-dialog-sk')).to.have.length(1);
  });

  it('should take a screenshot', async () => {
    await testBed.page.click('#show-dialog');
    await takeScreenshot(testBed.page, 'gold', 'filter-dialog-sk');
  });

  it('should take a screenshot with the query editor dialog visible', async () => {
    await testBed.page.click('#show-dialog');
    await testBed.page.click('.edit-query');
    await takeScreenshot(testBed.page, 'gold', 'filter-dialog-sk_query-editor-open');
  });
});
