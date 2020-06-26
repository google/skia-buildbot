import { expect } from 'chai';
import { takeScreenshot, TestBed } from '../../../puppeteer-tests/util';
import { loadGoldWebpack } from '../common_puppeteer_test/common_puppeteer_test';

describe('query-dialog-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadGoldWebpack();
  });
  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/dist/query-dialog-sk.html`);
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('query-dialog-sk')).to.have.length(1);
  });

  it('is initially empty', async () => {
    await testBed.page.click('#show-dialog');
    await takeScreenshot(testBed.page, 'gold', 'query-dialog-sk_no-selection');
  });

  it('can select a key', async () => {
    await testBed.page.click('#show-dialog');
    await testBed.page.click('select-sk div:nth-child(1)'); // Click on the first key.
    await takeScreenshot(testBed.page, 'gold', 'query-dialog-sk_key-selected');
  });

  it('can select a key and a value', async () => {
    await testBed.page.click('#show-dialog');
    await testBed.page.click('select-sk div:nth-child(1)'); // Click on the first key.
    await testBed.page.click('multi-select-sk div:nth-child(1)'); // Click on the first value.
    await takeScreenshot(testBed.page, 'gold', 'query-dialog-sk_key-and-value-selected');
  });

  it('can select multiple values', async () => {
    await testBed.page.click('#show-dialog');
    await testBed.page.click('select-sk div:nth-child(1)'); // Click on the first key.
    await testBed.page.click('multi-select-sk div:nth-child(1)'); // Click on the first value.
    await testBed.page.click('multi-select-sk div:nth-child(2)'); // Click on the second value.
    await testBed.page.click('multi-select-sk div:nth-child(3)'); // Click on the third value.
    await testBed.page.click('select-sk div:nth-child(2)'); // Click on the second key.
    await testBed.page.click('multi-select-sk div:nth-child(1)'); // Click on the first value.
    await testBed.page.click('select-sk div:nth-child(3)'); // Click on the third key.
    await testBed.page.click('multi-select-sk div:nth-child(1)'); // Click on the first value.
    await testBed.page.click('multi-select-sk div:nth-child(2)'); // Click on the second value.
    await testBed.page.click('select-sk div:nth-child(4)'); // Click on the fourth key.
    await testBed.page.click('multi-select-sk div:nth-child(1)'); // Click on the first value.
    await testBed.page.click('multi-select-sk div:nth-child(2)'); // Click on the second value.
    await testBed.page.click('multi-select-sk div:nth-child(3)'); // Click on the third value.
    await testBed.page.click('multi-select-sk div:nth-child(4)'); // Click on the fourth value.
    await takeScreenshot(testBed.page, 'gold', 'query-dialog-sk_multiple-values-selected');
  });

  it('can be opened with an initial non-empty selection', async () => {
    await testBed.page.click('#show-dialog-with-selection');
    await takeScreenshot(testBed.page, 'gold', 'query-dialog-sk_nonempty-initial-selection');
  });
});
