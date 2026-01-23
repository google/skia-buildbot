import { expect } from 'chai';
import { loadCachedTestBed, TestBed } from '../../../puppeteer-tests/util';
import { ElementHandle } from 'puppeteer';
import { ExploreSimpleSkPO } from './explore-simple-sk_po';
import { CLIPBOARD_READ_TIMEOUT_MS, DEFAULT_VIEWPORT } from '../common/puppeteer-test-util';
import { paramSet1 } from './test_data';
import { paramSet } from '../common/test-util';

const EXPECTED_QUERY_COUNT = 117;
describe('explore-simple-sk', () => {
  let testBed: TestBed;
  let exploreSimpleSk: ElementHandle;
  let simplePageSkPO: ExploreSimpleSkPO;

  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport(DEFAULT_VIEWPORT);
    try {
      await testBed.page.waitForFunction('window.customElements.get("explore-simple-sk")', {
        timeout: 10000,
      });
    } catch (e) {
      throw new Error(
        `Custom element "explore-simple-sk" was not defined within the timeout. Error: ${
          e instanceof Error
        }`
      );
    }
    const element = await testBed.page.$('explore-simple-sk');
    exploreSimpleSk = element!;
    await testBed.page.evaluate(async () => {
      await Promise.all([
        customElements.whenDefined('explore-simple-sk'),
        customElements.whenDefined('query-sk'),
        customElements.whenDefined('plot-google-chart-sk'),
        customElements.whenDefined('query-count-sk'),
        customElements.whenDefined('md-dialog'),
        customElements.whenDefined('dataframe-repository-sk'),
      ]);
    });
    try {
      await testBed.page.waitForFunction(
        (el: Element) => !!el,
        { timeout: 10000 },
        exploreSimpleSk
      );
    } catch (e) {
      await testBed.page.evaluate((el) => el.outerHTML, exploreSimpleSk);
      await testBed.page.evaluate((el) => el.shadowRoot !== null, exploreSimpleSk);
      throw new Error(
        `Element "explore-simple-sk" found, but .shadowRoot is null. Error: ${e instanceof Error}`
      );
    }
    simplePageSkPO = new ExploreSimpleSkPO(exploreSimpleSk);

    // Make the buttons visible for the tests
    await testBed.page.evaluate((el: Element) => {
      if (el) {
        const buttons = el.querySelector<HTMLElement>('#buttons');
        if (buttons) {
          buttons.style.display = 'block';
        }
      }
    }, exploreSimpleSk);
  });

  afterEach(async () => {
    if (testBed && testBed.page) {
      await testBed.page.setRequestInterception(false);
      testBed.page.removeAllListeners();
    }
  });

  it('should render the demo page', async () => {
    expect(await testBed.page.$$('explore-simple-sk')).to.have.length(1); // Smoke test.
  });

  describe('query dialog interaction', () => {
    it('should have the query dialog element in the page on initial load', async () => {
      await testBed.page.click('#demo-show-query-dialog');
      const dialogHandle = await testBed.page.waitForSelector('#query-dialog', {
        visible: true,
      });
      expect(dialogHandle).to.not.be.null;
    });

    it('should display the correct count for a query', async () => {
      await testBed.page.click('#demo-show-query-dialog');
      const querySkPO = await simplePageSkPO.querySk;
      await querySkPO.setCurrentQuery(paramSet);

      const initialKeys = await querySkPO.getKeys();
      expect(initialKeys.length).to.deep.equal(2);

      const keyToClick = initialKeys[0]; // Pick the first key to click
      await querySkPO.clickKey(keyToClick);
      await simplePageSkPO.queryCountSkPO.waitForSpinnerInactive();
      await querySkPO.clickValue('arm');

      await simplePageSkPO.queryCountSkPO.waitForSpinnerInactive();
      const query = await simplePageSkPO.queryCountSkPO.getCount();
      expect(query).to.equal(EXPECTED_QUERY_COUNT);
    });
  });

  describe('query-sk interactions', async () => {
    it('should allow updating the query', async () => {
      await testBed.page.click('#demo-show-query-dialog');
      const querySkPO = await simplePageSkPO.querySk;

      await querySkPO.setCurrentQuery(paramSet);
      const currentValue1 = await querySkPO.getCurrentQuery();
      await querySkPO.clickClearSelections();
      await querySkPO.setCurrentQuery(paramSet1);
      const currentValue2 = await querySkPO.getCurrentQuery();
      expect(currentValue1).not.deep.equal(currentValue2);
    });

    it('should display initial query from state', async () => {
      const simplePageSkPO = new ExploreSimpleSkPO((await testBed.page.$('explore-simple-sk'))!);
      await testBed.page.click('#demo-show-query-dialog');

      const querySkPO = await simplePageSkPO.querySk;
      const initialValue = await querySkPO.getCurrentQuery();
      expect(initialValue).is.not.null;
    });

    it('should update query values when a key is clicked', async () => {
      await testBed.page.click('#demo-show-query-dialog');
      const querySkPO = await simplePageSkPO.querySk;
      const initialKeys = await querySkPO.getKeys();
      expect(initialKeys).to.have.length.greaterThan(0);

      const keyToClick = initialKeys[0]; // Pick the first key to click
      await querySkPO.clickKey(keyToClick);

      // After clicking a key, the displayed values should update for that key.
      const selectedKey = await querySkPO.getSelectedKey();
      expect(selectedKey).to.equal(keyToClick);

      // It's hard to predict exact options without more context, so just check it's an array for now.
      const availableOptions = await querySkPO.queryValuesSkPO.getOptions();
      expect(availableOptions).to.be.an('array');
    });
  });

  describe('plot-google-chart-sk display', () => {
    it('should not render the chart on initial load', async () => {
      await simplePageSkPO.querySk;
      await testBed.page.waitForSelector('explore-simple-sk');
      const plotPO = await simplePageSkPO.plotGoogleChartSk;
      expect(await plotPO.isChartVisible()).to.be.false;
    });
  });

  describe('Graph interactions', () => {
    it('plots a graph and verifies traces', async () => {
      await testBed.page.click('#demo-show-query-dialog');
      const querySkPO = await simplePageSkPO.querySk;
      await querySkPO.setCurrentQuery(paramSet1);

      await simplePageSkPO.clickPlotButton();

      // Poll for the traces to appear.
      let traceKeys: string[] = [];
      await new Promise<void>((resolve, reject) => {
        const interval = setInterval(async () => {
          traceKeys = await simplePageSkPO.getTraceKeys();
          if (traceKeys.length > 0) {
            clearInterval(interval);
            resolve();
          }
        }, 100);

        setTimeout(() => {
          clearInterval(interval);
          if (traceKeys.length === 0) {
            reject(new Error('Timed out waiting for traces to load.'));
          } else {
            resolve();
          }
        }, CLIPBOARD_READ_TIMEOUT_MS); // 5s timeout
      });

      expect(traceKeys.length).to.equal(1);
      expect(traceKeys[0]).to.include(',arch=arm,os=Android,');
    });

    it('switches x-axis to "date" mode', async () => {
      await testBed.page.click('#demo-show-query-dialog');
      const querySkPO = await simplePageSkPO.querySk;
      await querySkPO.setCurrentQuery(paramSet);

      const tabPanel = await testBed.page.waitForSelector('tabs-panel-sk');
      const btn = await tabPanel!.waitForSelector('button.action');
      await btn!.click();

      const plotPO = await simplePageSkPO.plotGoogleChartSk;
      await plotPO.waitForChartVisible({ timeout: CLIPBOARD_READ_TIMEOUT_MS });
      expect(await simplePageSkPO.getXAxisDomain()).to.equal('commit');
      await simplePageSkPO.clickXAxisSwitch();
      // After click, should be 'date'
      expect(await simplePageSkPO.getXAxisDomain()).to.equal('date');
    });

    it('switches zoom direction to horizontal', async () => {
      await testBed.page.click('#demo-show-query-dialog');
      const querySkPO = await simplePageSkPO.querySk;
      await querySkPO.setCurrentQuery(paramSet);

      const tabPanel = await testBed.page.waitForSelector('tabs-panel-sk');
      const btn = await tabPanel!.waitForSelector('button.action');
      await btn!.click();

      const plotPO = await simplePageSkPO.plotGoogleChartSk;
      await plotPO.waitForChartVisible({ timeout: CLIPBOARD_READ_TIMEOUT_MS });
      expect(await simplePageSkPO.getHorizontalZoom()).to.be.false;
      await simplePageSkPO.clickZoomDirectionSwitch();
      // After click, should be true
      expect(await simplePageSkPO.getHorizontalZoom()).to.be.true;
    });

    it('switches to even x-axis spacing', async () => {
      await testBed.page.click('#demo-show-query-dialog');
      const querySkPO = await simplePageSkPO.querySk;
      await querySkPO.setCurrentQuery(paramSet);

      const tabPanel = await testBed.page.waitForSelector('tabs-panel-sk');
      const btn = await tabPanel!.waitForSelector('button.action');
      await btn!.click();

      const plotPO = await simplePageSkPO.plotGoogleChartSk;
      await plotPO.waitForChartVisible({ timeout: CLIPBOARD_READ_TIMEOUT_MS });
      expect(await simplePageSkPO.getEvenXAxisSpacing()).to.be.false;
      await simplePageSkPO.clickEvenXAxisSpacingSwitch();
      // After click, should be true
      expect(await simplePageSkPO.getEvenXAxisSpacing()).to.be.true;
    });
  });
});
