import { expect } from 'chai';
import { loadCachedTestBed, takeScreenshot, TestBed } from '../../../puppeteer-tests/util';
import { QueryCountSkPO } from './query-count-sk_po';

describe('query-count-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  let queryCountSkPO: QueryCountSkPO;

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport({ width: 400, height: 400 });
    // We need to wrap the element in a PageObject since the PO methods
    // are on QueryCountSkPO, not the element itself.
    const queryCountSk = await testBed.page.$('query-count-sk:first-of-type');
    queryCountSkPO = new QueryCountSkPO(queryCountSk!);
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('query-count-sk')).to.have.length(3);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'perf', 'query-count-sk');
      expect(await queryCountSkPO.getCount()).to.equal(0);
    });

    it('shows the count after querying', async () => {
      await testBed.page.click('#make_query');
      await takeScreenshot(testBed.page, 'perf', 'query-count-sk-queried');
      expect(await queryCountSkPO.getCount()).to.equal(12);
    });
  });

  it('updates the count when the query changes', async () => {
    const queryCountSk = await testBed.page.$('query-count-sk');
    const po = new QueryCountSkPO(queryCountSk!);

    // Initial state should be empty because query is not set initially.
    expect(await po.getCount()).equal(0);

    // Click the button to trigger a query change.
    await testBed.page.click('#make_query');

    // Wait for the count to update.
    await testBed.page.waitForFunction(
      (selector) => {
        const el = document.querySelector(selector);
        return el && el.textContent !== '0';
      },
      {},
      'query-count-sk div > span'
    );

    // Expect 12. The count starts at 11. There are 3 widgets, each adds a listener.
    // When clicked, the first listener runs, increments count to 12, sets query on first widget.
    // Fetch mock evaluates immediately (likely), returning 12.
    // Subsequent listeners will increment further, but for other widgets.
    expect(await po.getCount()).to.equal(12);
  });

  it('shows spinner while loading', async () => {
    const queryCountSk = await testBed.page.$('query-count-sk');
    const po = new QueryCountSkPO(queryCountSk!);

    // Delay the fetch to ensure we can catch the spinner state.
    await testBed.page.evaluate(() => {
      const originalFetch = window.fetch;
      window.fetch = async (input, init) => {
        await new Promise((resolve) => setTimeout(resolve, 500));
        return originalFetch(input, init);
      };
    });

    // Click the button to trigger a query change.
    await testBed.page.click('#make_query');

    // Check that spinner is active.
    await testBed.page.waitForFunction(
      (selector) => document.querySelector(selector)?.hasAttribute('active'),
      {},
      'query-count-sk spinner-sk'
    );
    expect(await po.isSpinnerActive()).to.be.true;

    // Wait for the spinner to become inactive.
    await testBed.page.waitForFunction(
      (selector) => !document.querySelector(selector)?.hasAttribute('active'),
      {},
      'query-count-sk spinner-sk'
    );

    // Check that spinner is inactive.
    expect(await po.isSpinnerActive()).to.be.false;
  });
});
