import { expect } from 'chai';
import { loadCachedTestBed, TestBed } from '../../../puppeteer-tests/util';
import { ElementHandle } from 'puppeteer';
import { ExploreSimpleSkPO } from './explore-simple-sk_po';
import {
  CLIPBOARD_READ_TIMEOUT_MS,
  STANDARD_LAPTOP_VIEWPORT,
  poll,
} from '../common/puppeteer-test-util';
import { default_container_title, expected_trace_key, paramSet1 } from './test_data';
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
    await testBed.page.setViewport(STANDARD_LAPTOP_VIEWPORT);
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
        customElements.whenDefined('chart-tooltip-sk'),
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
    expect(await testBed.page.$$('explore-simple-sk')).to.have.lengthOf(1); // Smoke test.
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
      const querySkPO = simplePageSkPO.querySk;
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

    it('should update matches count after clearing selections via query-sk cancel button', async () => {
      await testBed.page.click('#demo-show-query-dialog');
      const querySkPO = simplePageSkPO.querySk;

      await querySkPO.setCurrentQuery(paramSet);
      await simplePageSkPO.queryCountSkPO.waitForSpinnerInactive();

      const initialCount = await simplePageSkPO.queryCountSkPO.getCount();
      expect(initialCount).to.be.greaterThan(0);

      await querySkPO.clickClearSelections();

      await simplePageSkPO.queryCountSkPO.waitForSpinnerInactive();
      const updatedCount = await simplePageSkPO.queryCountSkPO.getCount();

      // Assert that the updated query count is different.
      expect(updatedCount).to.equal(0);
      expect(updatedCount).to.not.equal(initialCount);
    });

    it('should update matches count when a query is refined and then a parameter is removed from paramset-sk', async () => {
      await simplePageSkPO.openQueryDialogButton.click();
      await testBed.page.waitForSelector('#query-dialog', { visible: true });

      const querySkPO = simplePageSkPO.querySk;

      await querySkPO.setCurrentQuery(paramSet);
      await simplePageSkPO.queryCountSkPO.waitForSpinnerInactive();
      const initialKeys = await querySkPO.getKeys();
      const initialCount = await simplePageSkPO.queryCountSkPO.getCount();
      expect(initialKeys).to.have.length.greaterThan(0);

      const keyToClick = initialKeys[0];
      await querySkPO.clickKey(keyToClick);
      await simplePageSkPO.queryCountSkPO.waitForSpinnerInactive();

      await querySkPO.clickValue(paramSet[keyToClick as keyof typeof paramSet][0]);
      await simplePageSkPO.queryCountSkPO.waitForSpinnerInactive();
      const refinedQuery = { ...paramSet, config: ['8888'] }; // Add a filter
      await querySkPO.setCurrentQuery(refinedQuery);
      await simplePageSkPO.queryCountSkPO.waitForSpinnerInactive();
      const countAfterRefinement = await querySkPO.getKeys();
      expect(initialKeys.length).to.be.lessThanOrEqual(
        countAfterRefinement.length,
        'Count should decrease after refining query'
      );

      const keyToRemove = initialKeys[0];
      const valueToRemove = paramSet[keyToClick as keyof typeof paramSet][0];
      await simplePageSkPO.summaryParamsetSkPO.removeSelectedValue(keyToRemove, valueToRemove);
      await simplePageSkPO.queryCountSkPO.waitForSpinnerInactive();

      const finalCount = await simplePageSkPO.queryCountSkPO.getCount();
      // Expect the count to revert to the initial broad query's count.
      expect(finalCount).to.equal(
        initialCount,
        'Count should revert to initial broad query count after removal'
      );
    });
  });

  describe('query-sk interactions', async () => {
    it('should allow updating the query', async () => {
      await testBed.page.click('#demo-show-query-dialog');
      const querySkPO = simplePageSkPO.querySk;

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

      const querySkPO = simplePageSkPO.querySk;
      const initialValue = await querySkPO.getCurrentQuery();
      expect(initialValue).is.not.null;
    });

    it('should update query values when a key is clicked', async () => {
      await testBed.page.click('#demo-show-query-dialog');
      const querySkPO = simplePageSkPO.querySk;
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
      simplePageSkPO.querySk;
      await testBed.page.waitForSelector('explore-simple-sk');
      const plotPO = await simplePageSkPO.plotGoogleChartSk;
      expect(await plotPO.isChartVisible()).to.be.false;
    });
  });

  describe('Graph interactions', () => {
    it('plots a graph and verifies traces', async () => {
      await testBed.page.click('#demo-show-query-dialog');
      const querySkPO = simplePageSkPO.querySk;
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
      const querySkPO = simplePageSkPO.querySk;
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
      const querySkPO = simplePageSkPO.querySk;
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
      const querySkPO = simplePageSkPO.querySk;
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

    it('displays a tooltip when clicking on a data point', async () => {
      // Set a query and plot the graph.
      await testBed.page.click('#demo-show-query-dialog');
      const querySkPO = simplePageSkPO.querySk;
      await querySkPO.setCurrentQuery(paramSet1);
      await simplePageSkPO.clickPlotButton();

      // Wait for the chart to be visible.
      const plotPO = simplePageSkPO.plotGoogleChartSk;
      await plotPO.waitForChartVisible({ timeout: CLIPBOARD_READ_TIMEOUT_MS });

      // Poll for the traces to appear.
      await poll(async () => {
        const traceKeys = await simplePageSkPO.getTraceKeys();
        return traceKeys.length === 1;
      }, 'timed out waiting for 1 trace to load');

      // Get the trace keys and coordinates of the first data point.
      const traceKeys = await simplePageSkPO.getTraceKeys();
      expect(traceKeys.length).deep.equal(1);
      const traceKey = traceKeys[0];
      const coords = await simplePageSkPO.getTraceCoordinates(traceKey, 0);
      // Click on the data point.
      await testBed.page.mouse.click(coords!.x + coords!.width / 2, coords!.y + coords!.height / 2);

      // Wait for the tooltip to be visible.
      await poll(
        async () => {
          // First, check if the container is visible.
          const isContainerVisible = await simplePageSkPO.chartTooltip.container.applyFnToDOMNode(
            (el) => {
              const style = window.getComputedStyle(el);
              return (
                style.display !== 'none' && style.visibility !== 'hidden' && style.opacity !== '0'
              );
            }
          );
          if (!isContainerVisible) {
            console.log('Tooltip container not visible yet.');
            return false;
          }

          const tooltipTitle = await simplePageSkPO.chartTooltip.title.innerText;
          return tooltipTitle.includes(default_container_title);
        },
        'timed out waiting for tooltip to be visible',
        10000
      );

      // Verify the tooltip tracekey.
      expect(traceKey).to.include(expected_trace_key);
    });
  });

  describe('Scrolling', () => {
    it('should scroll up and down', async () => {
      // Ensure the page is long enough to scroll.
      await testBed.page.setViewport({ width: 800, height: 300 });

      const getScrollY = () => testBed.page.evaluate(() => window.scrollY);

      const initialScrollY = await getScrollY();
      expect(initialScrollY).to.equal(0);

      // Scroll down.
      await testBed.page.evaluate(() => window.scrollBy(0, 200));
      let newScrollY = await getScrollY();
      expect(newScrollY).to.be.greaterThan(initialScrollY);

      // Scroll back up.
      await testBed.page.evaluate(() => window.scrollTo(0, 0));
      newScrollY = await getScrollY();
      expect(newScrollY).to.equal(0);
    });
  });

  describe('Summary bar', () => {
    it('show the summary bar and verify the initial range', async () => {
      // https://screenshot.googleplex.com/AmWdqZgvPYuvh9i
      await testBed.page.goto(`${testBed.baseUrl}?manual_plot_mode=true&plotSummary=true`);

      const element = await testBed.page.$('explore-simple-sk');
      const simplePageSkPO = new ExploreSimpleSkPO(element!);

      // Manually enable plotSummary as ExploreSimpleSk doesn't read URL params in demo mode
      await testBed.page.evaluate((el: any) => {
        el.state = { ...el.state, plotSummary: true };
      }, element);

      await testBed.page.click('#demo-show-query-dialog');
      const querySkPO = simplePageSkPO.querySk;
      await querySkPO.setCurrentQuery(paramSet1);

      await simplePageSkPO.clickPlotButton();

      // Wait for the chart to be visible.
      const plotPO = simplePageSkPO.plotGoogleChartSk;
      await plotPO.waitForChartVisible({ timeout: CLIPBOARD_READ_TIMEOUT_MS });

      // Poll for the traces to appear.
      await poll(async () => {
        const traceKeys = await simplePageSkPO.getTraceKeys();
        return traceKeys.length === 1;
      }, 'timed out waiting for 1 trace to load');

      const plotSummaryPO = simplePageSkPO.plotSummary;
      await plotSummaryPO.waitForPlotSummaryToLoad();
      const initialRange = await plotSummaryPO.getSelectedRange();

      // Verify the begin and end of the selected area in the Summary bar.
      const header = await testBed.page.evaluate((el: any) => el.getHeader(), element);
      const start = header[0].offset;
      const end = header[header.length - 1].offset;
      // Selected range must be within the summary bar's start and end points.
      expect(initialRange!.begin).to.be.at.least(start);
      expect(initialRange!.end).to.be.at.most(end);
    });
  });
});
