import { expect } from 'chai';
import { ElementHandle } from 'puppeteer';
import { loadCachedTestBed, takeScreenshot, TestBed } from '../../../puppeteer-tests/util';
import { JsonSourceSkPO } from './json-source-sk_po';

describe('json-source-sk', () => {
  let testBed: TestBed;

  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport({ width: 600, height: 600 });
  });

  it('should render the demo page', async () => {
    // Smoke test.
    expect(await testBed.page.$$('json-source-sk')).to.have.length(3);
  });

  it('should have the correct json', async () => {
    const jsonSourceSk = (await testBed.page.$('json-source-sk')) as ElementHandle;
    const jsonSourceSkPO = new JsonSourceSkPO(jsonSourceSk);
    const expectedJson = {
      Hello: 'world!',
    };
    const actualJson = await jsonSourceSkPO.getJson();
    expect(JSON.parse(actualJson)).to.deep.equal(expectedJson);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      // The screenshot is saved as a test output.
      await takeScreenshot(testBed.page, 'perf', 'json-source-sk');
    });
  });

  describe('interactions', () => {
    let jsonSourceSk: ElementHandle;
    let jsonSourceSkPO: JsonSourceSkPO;
    beforeEach(async () => {
      // Close all open dialogs, otherwise we can't click on backgroud buttons.
      await testBed.page.evaluate(() => {
        document.querySelectorAll('json-source-sk').forEach((el) => {
          const root = el.shadowRoot || el;
          const dialog = root.querySelector('#json-dialog');
          const closeBtn = root.querySelector('#closeIcon');
          if (dialog && dialog.hasAttribute('open') && closeBtn) {
            (closeBtn as HTMLElement).click();
          }
        });
      });

      // We have 3 elements on the page, select a random one for testing.
      jsonSourceSk = (await testBed.page.$('json-source-sk')) as ElementHandle;
      jsonSourceSkPO = new JsonSourceSkPO(jsonSourceSk);
      await jsonSourceSk.evaluate(async (el: any) => {
        el.cid = 123;
        el.traceid = ',some_trace_id,'; // Correct traceid to satisfy validKey
        await el.updateComplete;
      });
      await testBed.page.waitForFunction(
        () =>
          !document
            .querySelector('json-source-sk')!
            .querySelector('#controls')!
            .hasAttribute('hidden')
      );
    });

    afterEach(async () => {
      await testBed.page.evaluate(() => {
        (window as any).fetchMock.restore();
      });
    });

    it('should show the full json', async () => {
      // Configure the browser-side fetch-mock
      const expectedJson = { a: 1, b: 2 };
      await testBed.page.evaluate((json) => {
        const fetchMock = (window as any).fetchMock;
        fetchMock.reset();
        fetchMock.post('begin:/_/details/', json);
      }, expectedJson);

      // Perform the action (click "View Json File")
      await jsonSourceSkPO.clickViewJsonFileButton();
      // Wait for the result
      await testBed.page.waitForFunction(
        (el) => {
          const dialog = el.querySelector('#json-dialog');
          return dialog?.hasAttribute('open');
        },
        {},
        jsonSourceSk
      );

      // Check the results
      expect(await jsonSourceSkPO.isDialogVisible()).to.equal(true);
      const actualJson = await jsonSourceSkPO.getJsonFromDialog();
      expect(JSON.parse(actualJson)).to.deep.equal(expectedJson);
    });

    it('should show the short json', async () => {
      // Configure the browser-side fetch-mock
      const expectedJson = { a: 1 };
      await testBed.page.evaluate((json) => {
        const fetchMock = (window as any).fetchMock;
        fetchMock.reset();
        fetchMock.post('begin:/_/details/', json);
      }, expectedJson);

      // Perform the action (click "View Short Json File")
      await jsonSourceSkPO.clickViewShortJsonFileButton();
      // Wait for the result
      await testBed.page.waitForFunction(
        (el) => {
          const dialog = el.querySelector('#json-dialog');
          return dialog?.hasAttribute('open');
        },
        {},
        jsonSourceSk
      );

      // Check the results
      expect(await jsonSourceSkPO.isDialogVisible()).to.equal(true);
      const actualJson = await jsonSourceSkPO.getJsonFromDialog();
      expect(JSON.parse(actualJson)).to.deep.equal(expectedJson);
    });

    it('should handle server errors gracefully', async () => {
      // Mock a backend failure (500 Internal Server Error)
      await testBed.page.evaluate(() => {
        const fm = (window as any).fetchMock;
        fm.reset();
        fm.post('begin:/_/details/', {
          status: 500,
          body: 'THIS IS A TEST',
          headers: { 'Content-Type': 'text/plain' },
        });
      });

      // Perform the action
      await jsonSourceSkPO.clickViewShortJsonFileButton();

      // Check the results: error should be shown instead of modal.
      const isVisible = await jsonSourceSkPO.isDialogVisible();
      expect(isVisible).to.equal(false);
      // Check the error message
      const toastSelector = 'error-toast-sk toast-sk[shown]';
      await testBed.page.waitForSelector(toastSelector);
      const errorMessage = await testBed.page.$eval(
        'error-toast-sk toast-sk span',
        (el) => el.textContent
      );
      expect(errorMessage).to.not.equal(null);
      expect(errorMessage).to.contain('THIS IS A TEST');
    });
  });
});
