import { TestBed, loadCachedTestBed } from '../../../puppeteer-tests/util';

describe('explore_multi_page_test', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}/m`);
    await testBed.page.setViewport({
      width: 869,
      height: 807,
    });
  });

  it('test picker selection works', async () => {
    const page = testBed.page;
    const timeout = 5000;
    page.setDefaultTimeout(timeout);

    await page.waitForSelector('#input-vaadin-multi-select-combo-box-4');
    await page.click('#input-vaadin-multi-select-combo-box-4');
    await page.waitForSelector('#vaadin-multi-select-combo-box-item-0');
  });
});
