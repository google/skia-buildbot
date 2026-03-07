import { expect } from 'chai';
import { loadCachedTestBed, TestBed } from '../../../puppeteer-tests/util';
import { ExploreMultiSkPO } from './explore-multi-sk_po';
import {
  STANDARD_LAPTOP_VIEWPORT,
  poll,
  waitForElementNotHidden,
} from '../common/puppeteer-test-util';
import { TestPickerSkPO } from '../test-picker-sk/test-picker-sk_po';
import { ExploreSimpleSkPO } from '../explore-simple-sk/explore-simple-sk_po';
import { Page } from 'puppeteer';

/**
 * startUrlTracking injects a spy into the page to count history updates.
 */
const startUrlTracking = async (page: Page) => {
  await page.evaluate(() => {
    (window as any).urlChangeCount = 0;
    const originalPushState = history.pushState;
    const originalReplaceState = history.replaceState;
    history.pushState = function (...args) {
      (window as any).urlChangeCount++;
      return originalPushState.apply(history, args);
    };
    history.replaceState = function (...args) {
      (window as any).urlChangeCount++;
      return originalReplaceState.apply(history, args);
    };
  });
};

/**
 * getUrlChangeCount returns the number of URL changes since tracking started.
 */
const getUrlChangeCount = async (page: Page): Promise<number> => {
  return await page.evaluate(() => (window as any).urlChangeCount);
};

/**
 * resetUrlChangeCount resets the counter.
 */
const resetUrlChangeCount = async (page: Page) => {
  await page.evaluate(() => {
    (window as any).urlChangeCount = 0;
  });
};

/**
 * expectUrlParams verifies that the current URL contains the expected parameters.
 */
const expectUrlParams = async (page: Page, expected: Record<string, string | null>) => {
  const url = new URL(page.url());
  for (const [key, value] of Object.entries(expected)) {
    if (value === null) {
      expect(url.searchParams.get(key)).to.be.null;
    } else {
      expect(url.searchParams.get(key)).to.equal(value);
    }
  }
};

const clearSelections = async (testPickerPO: TestPickerSkPO) => {
  // Clear all existing selections first, in reverse order.
  const fields = await testPickerPO.pickerFields;
  for (let i = (await fields.length) - 1; i >= 0; i--) {
    const field = await testPickerPO.getPickerField(i);
    await field.clear();
    await testPickerPO.waitForSpinnerInactive();
  }
};

const addGraph = async (
  testPickerPO: TestPickerSkPO,
  explorePO: ExploreMultiSkPO,
  selections: { [index: number]: string[] },
  expectedGraphCount?: number
) => {
  await clearSelections(testPickerPO);

  for (const [indexStr, options] of Object.entries(selections)) {
    const index = parseInt(indexStr);
    await testPickerPO.waitForPickerField(index);
    const field = await testPickerPO.getPickerField(index);
    for (const option of options) {
      await field.select(option);
      await testPickerPO.waitForSpinnerInactive();
    }
  }
  await testPickerPO.clickPlotButton();
  if (expectedGraphCount !== undefined) {
    await explorePO.waitForGraphCount(expectedGraphCount);
  }
  await explorePO.waitForGraph(0);
};

const verifyGraphTitle = async (graphPO: ExploreSimpleSkPO, expectedOs: string) => {
  const graphTitle = await graphPO.bySelector('#graphTitle');
  expect(await graphTitle.isEmpty()).to.be.false;

  const columns = await graphTitle.bySelectorAll('.column');
  expect(await columns.length).to.equal(3);

  const expectedData = [
    { key: 'arch', value: 'arm' },
    { key: 'os', value: expectedOs },
    { key: 'test', value: 'Default' },
  ];

  for (let i = 0; i < expectedData.length; i++) {
    const column = await columns.item(i);
    const param = await column.bySelector('.param');
    const value = await column.bySelector('.hover-to-show-text');

    expect(await param.innerText).to.equal(expectedData[i].key);
    expect(await value.innerText).to.equal(expectedData[i].value);
  }
};

const LONG_TIMEOUT_MS = 30000;
const GRAPH_LOAD_TIMEOUT_MS = 20000;

describe('Anomalies and Traces', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    const queryParams = '?begin=1687855198&end=1687961973';
    await testBed.page.goto(testBed.baseUrl + queryParams);
    await testBed.page.setViewport(STANDARD_LAPTOP_VIEWPORT);
  });

  it('removes anomalies and trace when query removed from selector', async () => {
    const EXPECTED_ANOMALIES_COUNT_BEFORE_REMOVAL = 2;
    const EXPECTED_COUNT_AFTER_REMOVAL = 1;
    const explorePO = new ExploreMultiSkPO((await testBed.page.$('explore-multi-sk'))!);
    const testPickerPO = explorePO.testPicker;

    await addGraph(testPickerPO, explorePO, { 0: ['arm'], 1: ['Android', 'Ubuntu'] });

    const osField = await testPickerPO.getPickerField(1);

    // Verify anomalies are present (should be 2 as we selected both traces)
    const exploreSimplePO = explorePO.exploreSimpleSk;
    await testBed.page.waitForFunction(
      (expectedCount) => {
        const explore = document.querySelector('explore-multi-sk');
        const simple = explore?.querySelector('explore-simple-sk');
        const plot = simple?.querySelector('plot-google-chart-sk') as any;
        return plot && plot.anomalyMap && Object.keys(plot.anomalyMap).length === expectedCount;
      },
      {},
      EXPECTED_ANOMALIES_COUNT_BEFORE_REMOVAL
    );

    let anomalyMap = await exploreSimplePO.getAnomalyMap();
    expect(Object.keys(anomalyMap).length).to.equal(EXPECTED_ANOMALIES_COUNT_BEFORE_REMOVAL);

    // TODO(eduardoyap): Add URL tracking assertions
    // (e.g., expect(await getUrlChangeCount...).to.equal(1))
    // here once trace removal functionality is fully working. They were temporarily
    // removed as the feature is currently unsupported/broken.
    await startUrlTracking(testBed.page);
    await osField.removeSelectedOption('Android');

    // Wait for graph update (Android trace removal)
    await testBed.page.waitForFunction(
      (expectedCount) => {
        const explore = document.querySelector('explore-multi-sk');
        const simple = explore?.querySelector('explore-simple-sk');
        const plot = simple?.querySelector('plot-google-chart-sk') as any;
        return plot && plot.getAllTraces && plot.getAllTraces().length === expectedCount;
      },
      {},
      EXPECTED_COUNT_AFTER_REMOVAL
    );

    // Verify Android is gone and Ubuntu is present
    const traces = await exploreSimplePO.getTraceKeys();
    expect(traces).to.not.include(',arch=arm,os=Android,');
    expect(traces).to.include(',arch=arm,os=Ubuntu,');

    anomalyMap = await exploreSimplePO.getAnomalyMap();
    expect(anomalyMap).to.not.have.property(',arch=arm,os=Android,');
    expect(anomalyMap).to.have.property(',arch=arm,os=Ubuntu,');

    // Simulate mouse hover over Ubuntu trace
    const ubuntuKey = ',arch=arm,os=Ubuntu,';
    const pointIndex = 10; // Arbitrary point
    const coords = await exploreSimplePO.getTraceCoordinates(ubuntuKey, pointIndex);
    await testBed.page.mouse.move(coords!.x, coords!.y);

    // Verify hover indicator is visible
    const isHoverVisible = await exploreSimplePO.googleChart.applyFnToDOMNode((el) => {
      const indicator = el.shadowRoot?.querySelector('.hover-indicator') as HTMLElement;
      return indicator && indicator.style.display !== 'none';
    });
    expect(isHoverVisible).to.be.true;
  });

  it('should draw anomalies when domain is date', async () => {
    const explorePO = new ExploreMultiSkPO((await testBed.page.$('explore-multi-sk'))!);
    const testPickerPO = explorePO.testPicker;

    await addGraph(testPickerPO, explorePO, { 0: ['arm'], 1: ['Android'] });

    const exploreSimplePO = explorePO.exploreSimpleSk;

    // Wait for anomalies to populate
    await testBed.page.waitForFunction(() => {
      const explore = document.querySelector('explore-multi-sk');
      const simple = explore?.querySelector('explore-simple-sk');
      const plot = simple?.querySelector('plot-google-chart-sk') as any;
      // Expect at least 1 anomaly
      return plot && plot.anomalyMap && Object.keys(plot.anomalyMap).length > 0;
    });

    // Check if anomaly is visible in commit mode (default)
    const anomaliesCommitCount = await exploreSimplePO.googleChart.applyFnToDOMNode((el) => {
      return el.shadowRoot!.querySelectorAll('.anomaly').length;
    });
    expect(anomaliesCommitCount).to.be.greaterThan(0);

    // Switch to date domain
    await startUrlTracking(testBed.page);
    await exploreSimplePO.clickXAxisSwitch();

    // Wait for render/update
    await testBed.page.waitForFunction(() => {
      const explore = document.querySelector('explore-multi-sk');
      const simple = explore?.querySelector('explore-simple-sk');
      const plot = simple?.querySelector('plot-google-chart-sk') as any;
      return plot.domain === 'date';
    });

    // Check if anomaly is visible in date mode
    const anomaliesDateCount = await exploreSimplePO.googleChart.applyFnToDOMNode((el) => {
      return el.shadowRoot!.querySelectorAll('.anomaly').length;
    });
    expect(anomaliesDateCount).to.be.greaterThan(0);
    expect(anomaliesDateCount).to.equal(anomaliesCommitCount);

    // Get coordinates of an anomaly.
    const anomalyRect = await exploreSimplePO.googleChart.applyFnToDOMNode((el) => {
      const anomalyIcon = el.shadowRoot!.querySelector('div.anomaly > .anomaly');
      if (!anomalyIcon) return null;
      const rect = anomalyIcon.getBoundingClientRect();
      return { x: rect.x, y: rect.y, width: rect.width, height: rect.height };
    });
    expect(anomalyRect).to.not.be.null;

    // Click on it.
    await testBed.page.mouse.click(
      anomalyRect!.x + anomalyRect!.width / 2,
      anomalyRect!.y + anomalyRect!.height / 2
    );

    // Check for tooltip and triage menu.
    const chartTooltipPO = exploreSimplePO.chartTooltip;
    const containerPO = chartTooltipPO.container;

    // Wait for tooltip to be visible and have content
    await poll(async () => {
      if (await containerPO.isEmpty()) return false;
      return await containerPO.applyFnToDOMNode((el) => {
        if ((el as HTMLElement).style.display === 'none') return false;
        const h3 = el.querySelector('h3');
        return h3?.textContent?.includes('Anomaly') || false;
      });
    }, 'Tooltip did not show Anomaly or was not visible');

    const triageMenuPO = chartTooltipPO.getTriageMenu;
    await poll(async () => !(await triageMenuPO.isEmpty()), 'Triage menu did not appear');
  });
});

describe('Manual Plot Mode', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(`${testBed.baseUrl}?manual_plot_mode=true`);
    await testBed.page.setViewport(STANDARD_LAPTOP_VIEWPORT);
  });

  it('plot two graphs in manual plot mode', async () => {
    const explorePO = new ExploreMultiSkPO((await testBed.page.$('explore-multi-sk'))!);
    const testPickerPO = explorePO.testPicker;

    // Add first graph.
    await startUrlTracking(testBed.page);
    await addGraph(testPickerPO, explorePO, { 0: ['arm'], 1: ['Android'] });

    // Verify First Graph (Index 0)
    expect(await explorePO.getGraphCount()).to.equal(1);

    // Verify URL was updated correctly and only once.
    await testBed.page.waitForTimeout(500);
    expect(await getUrlChangeCount(testBed.page)).to.equal(1);
    await expectUrlParams(testBed.page, { totalGraphs: '1' });

    const graph1PO = explorePO.getGraph(0);
    const traces1 = await graph1PO.getTraceKeys();
    expect(traces1).to.include(',arch=arm,os=Android,');

    // Add second graph.
    // In this mode, we select both 'Android' and 'Ubuntu'.
    // The picker state is now: { arch: 'arm', os: ['Android', 'Ubuntu'] }
    await resetUrlChangeCount(testBed.page);
    await addGraph(testPickerPO, explorePO, { 0: ['arm'], 1: ['Android', 'Ubuntu'] }, 2);

    // Verify URL was updated correctly and only once.
    await testBed.page.waitForTimeout(500);
    expect(await getUrlChangeCount(testBed.page)).to.equal(1);
    await expectUrlParams(testBed.page, { totalGraphs: '2' });

    // Verify Top Graph (Index 0)
    // Should reflect the CURRENT picker state (Android + Ubuntu)
    const graphTopPO = explorePO.getGraph(0);
    const tracesTop = await graphTopPO.getTraceKeys();
    expect(tracesTop).to.include(',arch=arm,os=Ubuntu,');
    expect(tracesTop).to.include(',arch=arm,os=Android,');

    // Verify graph title content for the top graph
    await verifyGraphTitle(graphTopPO, 'Various');

    // Verify Bottom Graph (Index 1)
    // Should remain a snapshot of the PAST state (Only Android)
    // This proves the old graph wasn't mutated by the new plot action
    const graphBottomPO = explorePO.getGraph(1);
    const tracesBottom = await graphBottomPO.getTraceKeys();
    expect(tracesBottom).to.include(',arch=arm,os=Android,');
    expect(tracesBottom).to.not.include(',arch=arm,os=Ubuntu,');

    // Verify graph title content for the bottom graph
    await verifyGraphTitle(graphBottomPO, 'Android');

    // Check URL state after adding the second graph
    expect(new URL(await testBed.page.url()).searchParams.get('totalGraphs')).to.equal('2');
  });

  it('selects multiple values in the first field and plots all combinations', async () => {
    const explorePO = new ExploreMultiSkPO((await testBed.page.$('explore-multi-sk'))!);
    const testPickerPO = explorePO.testPicker;

    // Select 'arm' in arch field.
    await testPickerPO.waitForPickerField(0);
    const archField = await testPickerPO.getPickerField(0);
    await archField.select('arm');
    await testPickerPO.waitForSpinnerInactive();

    // Select 'x86_64' in arch field.
    await archField.select('x86_64');
    await testPickerPO.waitForSpinnerInactive();

    // Verify 'os' field is now visible and has options.
    await testPickerPO.waitForPickerField(1);
    const osField = await testPickerPO.getPickerField(1);
    expect(await osField.getLabel()).to.equal('os');

    // Select 'Android', 'Ubuntu', and 'Debian11' in os field.
    await osField.select('Android');
    await testPickerPO.waitForSpinnerInactive();
    await osField.select('Ubuntu');
    await testPickerPO.waitForSpinnerInactive();
    await osField.select('Debian11');
    await testPickerPO.waitForSpinnerInactive();

    // Click Plot.
    await testPickerPO.clickPlotButton();
    await explorePO.waitForGraphCount(1);
    await explorePO.waitForGraph(0);

    // Verify trace keys.
    const traces = await explorePO.getGraph(0).getTraceKeys();
    // Combinations that match mock data: arm/Android, arm/Ubuntu, arm/Debian11, x86_64/Debian11.
    expect(traces).to.have.lengthOf(4);
    expect(traces).to.include(',arch=arm,os=Android,');
    expect(traces).to.include(',arch=arm,os=Ubuntu,');
    expect(traces).to.include(',arch=arm,os=Debian11,');
    expect(traces).to.include(',arch=x86_64,os=Debian11,');
  });

  it('populates test picker with query from different graphs', async () => {
    const explorePO = new ExploreMultiSkPO((await testBed.page.$('explore-multi-sk'))!);
    const testPickerPO = explorePO.testPicker;

    // Plot first graph with 1 trace (arm, Android)
    await addGraph(testPickerPO, explorePO, { 0: ['arm'], 1: ['Android'] });

    // Plot second graph with 2 traces (arm, Android, Ubuntu)
    await addGraph(testPickerPO, explorePO, { 0: ['arm'], 1: ['Android', 'Ubuntu'] }, 2);

    const traces0 = await explorePO.getGraph(0).getTraceKeys();
    expect(traces0).to.have.lengthOf(2);
    const traces1 = await explorePO.getGraph(1).getTraceKeys();
    expect(traces1).to.have.lengthOf(1);

    // Click Populate Query on Graph 1
    await explorePO.getGraph(1).populateQueryButton.click();
    await testPickerPO.waitForSpinnerInactive();

    expect(await testPickerPO.getSelectedItems(0)).to.deep.equal(['arm']);
    expect(await testPickerPO.getSelectedItems(1)).to.deep.equal(['Android']);

    // Click Populate Query on Graph 0
    await explorePO.getGraph(0).populateQueryButton.click();
    await testPickerPO.waitForSpinnerInactive();

    expect(await testPickerPO.getSelectedItems(0)).to.deep.equal(['arm']);
    expect((await testPickerPO.getSelectedItems(1)).sort()).to.deep.equal(
      ['Android', 'Ubuntu'].sort()
    );
  });

  it('adds three graphs and removes them in specific order (middle, first, last)', async () => {
    const explorePO = new ExploreMultiSkPO((await testBed.page.$('explore-multi-sk'))!);
    const testPickerPO = explorePO.testPicker;

    // Graph 1: Arch=arm, OS=Android
    await addGraph(testPickerPO, explorePO, { 0: ['arm'], 1: ['Android'] });

    // Graph 2: Arch=arm, OS=Ubuntu
    await addGraph(testPickerPO, explorePO, { 0: ['arm'], 1: ['Ubuntu'] }, 2);

    // Graph 3: Arch=arm, OS=Android
    await addGraph(testPickerPO, explorePO, { 0: ['arm'], 1: ['Android'] }, 3);

    // Verify initial state.
    // Index 0 (Newest): Android (Graph 3)
    // Index 1 (Middle): Ubuntu (Graph 2)
    // Index 2 (Oldest): Android (Graph 1)

    // Verify URL has 3 graphs
    let currentUrl = new URL(await testBed.page.url());
    expect(currentUrl.searchParams.get('totalGraphs')).to.equal('3');
    expect(currentUrl.searchParams.get('shortcut')).to.not.be.null;

    const traces0 = await explorePO.getGraph(0).getTraceKeys();
    expect(traces0).to.have.lengthOf(1);
    expect(traces0[0]).to.equal(',arch=arm,os=Android,');

    const traces1 = await explorePO.getGraph(1).getTraceKeys();
    expect(traces1).to.have.lengthOf(1);
    expect(traces1[0]).to.equal(',arch=arm,os=Ubuntu,');

    const traces2 = await explorePO.getGraph(2).getTraceKeys();
    expect(traces2).to.have.lengthOf(1);
    expect(traces2[0]).to.equal(',arch=arm,os=Android,');

    // REMOVE MIDDLE (Ubuntu)
    const middleGraph = explorePO.getGraph(1);
    await startUrlTracking(testBed.page);
    await middleGraph.clickRemoveAllButton();
    await explorePO.waitForGraphCount(2);

    // Verify URL update
    await testBed.page.waitForTimeout(500);
    expect(await getUrlChangeCount(testBed.page)).to.equal(1);
    await expectUrlParams(testBed.page, { totalGraphs: '2' });

    // Now Index 0 should be Graph 3 (Android)
    // Index 1 should be Graph 1 (Android)

    const tracesPostRem1_0 = await explorePO.getGraph(0).getTraceKeys();
    expect(tracesPostRem1_0).to.have.lengthOf(1);
    expect(tracesPostRem1_0[0]).to.equal(',arch=arm,os=Android,');

    const tracesPostRem1_1 = await explorePO.getGraph(1).getTraceKeys();
    expect(tracesPostRem1_1).to.have.lengthOf(1);
    expect(tracesPostRem1_1[0]).to.equal(',arch=arm,os=Android,');

    // REMOVE FIRST
    const firstGraph = explorePO.getGraph(0);
    await resetUrlChangeCount(testBed.page);
    await firstGraph.clickRemoveAllButton();
    await explorePO.waitForGraphCount(1);

    // Verify URL update
    await testBed.page.waitForTimeout(500);
    expect(await getUrlChangeCount(testBed.page)).to.equal(1);
    await expectUrlParams(testBed.page, { totalGraphs: '1' });

    const tracesPostRem2_0 = await explorePO.getGraph(0).getTraceKeys();
    expect(tracesPostRem2_0).to.have.lengthOf(1);
    expect(tracesPostRem2_0[0]).to.equal(',arch=arm,os=Android,');

    // REMOVE LAST
    const lastGraph = explorePO.getGraph(0);
    await resetUrlChangeCount(testBed.page);
    await lastGraph.clickRemoveAllButton();
    await explorePO.waitForGraphCount(0);

    // Verify URL update
    await testBed.page.waitForTimeout(500);
    expect(await getUrlChangeCount(testBed.page)).to.equal(1);
    await expectUrlParams(testBed.page, { totalGraphs: null, shortcut: null });

    currentUrl = new URL(await testBed.page.url());
    const finalShortcut = currentUrl.searchParams.get('shortcut');
    expect(finalShortcut).to.satisfy((s: string) => s === null || s === '');
  });

  describe('with Plot Summary', () => {
    beforeEach(async () => {
      await testBed.page.goto(`${testBed.baseUrl}?manual_plot_mode=true&plotSummary=true`);
    });

    it('zooms both graphs when range selected in one', async () => {
      const explorePO = new ExploreMultiSkPO((await testBed.page.$('explore-multi-sk'))!);
      const testPickerPO = explorePO.testPicker;

      await addGraph(testPickerPO, explorePO, { 0: ['arm'], 1: ['Android'] });

      await addGraph(testPickerPO, explorePO, { 0: ['arm'], 1: ['Ubuntu'] }, 2);

      const graph0 = explorePO.getGraph(0);
      const plotSummaryPO0 = graph0.plotSummary;
      await plotSummaryPO0.waitForPlotSummaryToLoad();

      const graph1 = explorePO.getGraph(1);
      const plotSummaryPO1 = graph1.plotSummary;
      await plotSummaryPO1.waitForPlotSummaryToLoad();

      const initialRange0 = await plotSummaryPO0.getSelectedRange();
      const initialRange1 = await plotSummaryPO1.getSelectedRange();
      const width0 = initialRange0!.end - initialRange0!.begin;
      const width1 = initialRange1!.end - initialRange1!.begin;

      const initialUrl = new URL(testBed.page.url());
      const initialBegin = initialUrl.searchParams.get('begin');
      const initialEnd = initialUrl.searchParams.get('end');

      await plotSummaryPO0.resizeSelection(testBed.page, 'right', 0.75);

      const finalUrl = new URL(testBed.page.url());
      const finalBegin = finalUrl.searchParams.get('begin');
      const finalEnd = finalUrl.searchParams.get('end');

      expect(finalBegin).to.not.equal(initialBegin);
      expect(finalEnd).to.not.equal(initialEnd);
      expect(Number(finalBegin)).to.not.be.NaN;
      expect(Number(finalEnd)).to.not.be.NaN;

      const finalRange0 = await plotSummaryPO0.getSelectedRange();
      const finalRange1 = await plotSummaryPO1.getSelectedRange();
      const finalWidth0 = finalRange0!.end - finalRange0!.begin;
      const finalWidth1 = finalRange1!.end - finalRange1!.begin;

      expect(finalWidth0).to.not.equal(width0);
      expect(finalWidth1).to.not.equal(width1);
      expect(finalWidth0).to.equal(finalWidth1);
      // Use closeTo because the plot summary selection bounds don't exactly match the
      // detail graph's displayed range due to pixel-to-value conversion discrepancies
      // and automatic axis padding by Google Charts.
      expect(finalRange0?.begin).to.be.closeTo(finalRange1!.begin!, 0.001);
      expect(finalRange0?.end).to.be.closeTo(finalRange1!.end!, 0.001);

      const checkRange = async (graph: any, name: string) => {
        await poll(
          async () => {
            const range = await graph.getVisibleXAxisRange();
            if (!range) return false;

            const rangeWidth = finalRange0!.end - finalRange0!.begin;
            // Use a 5% tolerance to account for pixel-to-value conversion discrepancies
            // between the summary and main graphs, as well as automatic axis padding
            // applied by Google Charts.
            const tolerance = rangeWidth * 0.05;

            return (
              Math.abs(range.min - finalRange0!.begin) < tolerance &&
              Math.abs(range.max - finalRange0!.end) < tolerance
            );
          },
          `${name} range mismatch`,
          5000
        );
      };

      await checkRange(graph0, 'Graph 0');
      await checkRange(graph1, 'Graph 1');
    });
  });
});

describe('Explore Multi Sk with plotSummary', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    // Navigate with plotSummary=true
    await testBed.page.goto(`${testBed.baseUrl}?plotSummary=true`);
    await testBed.page.setViewport(STANDARD_LAPTOP_VIEWPORT);
  });

  it('should allow selecting a range on plot-summary-sk and zoom the graph', async () => {
    const explorePO = new ExploreMultiSkPO((await testBed.page.$('explore-multi-sk'))!);
    const testPickerPO = explorePO.testPicker;

    await addGraph(testPickerPO, explorePO, { 0: ['arm'], 1: ['Android'] });

    const simpleGraphPO = explorePO.getGraph(0);
    const plotSummaryPO = simpleGraphPO.plotSummary;
    await plotSummaryPO.waitForPlotSummaryToLoad();

    const initialUrl = new URL(testBed.page.url());
    const initialBegin = initialUrl.searchParams.get('begin');
    const initialEnd = initialUrl.searchParams.get('end');

    // Resize from the right: 'end' should change, 'begin' should stay the same.
    await startUrlTracking(testBed.page);
    await plotSummaryPO.resizeSelection(testBed.page, 'right', 0.85);

    // Verify URL update
    await testBed.page.waitForTimeout(500);
    expect(await getUrlChangeCount(testBed.page)).to.equal(1);

    let finalUrl = new URL(testBed.page.url());
    let finalBegin = finalUrl.searchParams.get('begin');
    let finalEnd = finalUrl.searchParams.get('end');

    // We allow some tolerance (e.g. 5000s) because Google Charts might snap or adjust the
    // visible range slightly even on the "anchored" side when the other side changes significantly.
    const tolerance = 5000;
    expect(Number(finalBegin)).to.be.closeTo(
      Number(initialBegin),
      tolerance,
      'Begin should stay approximately the same when resizing right'
    );
    expect(Number(finalEnd)).to.not.be.closeTo(
      Number(initialEnd),
      tolerance,
      'End should change significantly when resizing right'
    );

    expect(Number(finalBegin)).to.not.be.NaN;
    expect(Number(finalEnd)).to.not.be.NaN;

    // Resize from the left: 'begin' should change, 'end' should stay the same.
    const midBegin = finalBegin;
    const midEnd = finalEnd;

    await resetUrlChangeCount(testBed.page);
    await plotSummaryPO.resizeSelection(testBed.page, 'left', 0.15);

    await testBed.page.waitForTimeout(500);
    expect(await getUrlChangeCount(testBed.page)).to.equal(1);

    finalUrl = new URL(testBed.page.url());
    finalBegin = finalUrl.searchParams.get('begin');
    finalEnd = finalUrl.searchParams.get('end');

    expect(Number(finalBegin)).to.not.be.closeTo(
      Number(midBegin),
      tolerance,
      'Begin should change significantly when resizing left'
    );
    expect(Number(finalEnd)).to.be.closeTo(
      Number(midEnd),
      tolerance,
      'End should stay approximately the same when resizing left'
    );
  });
});

describe('Even X-Axis Spacing', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.evaluate(() => localStorage.clear());
    await testBed.page.setViewport(STANDARD_LAPTOP_VIEWPORT);
  });

  it('syncs even x-axis spacing across all graphs', async () => {
    await testBed.page.goto(`${testBed.baseUrl}?manual_plot_mode=true`);
    const explorePO = new ExploreMultiSkPO((await testBed.page.$('explore-multi-sk'))!);
    const testPickerPO = explorePO.testPicker;

    await addGraph(testPickerPO, explorePO, { 0: ['arm'], 1: ['Android'] });

    await addGraph(testPickerPO, explorePO, { 0: ['arm'], 1: ['Ubuntu'] }, 2);

    const graph0 = explorePO.getGraph(0);
    const graph1 = explorePO.getGraph(1);

    expect(await graph0.getEvenXAxisSpacing()).to.be.false;
    expect(await graph1.getEvenXAxisSpacing()).to.be.false;

    await graph0.clickEvenXAxisSpacingSwitch();

    await poll(
      async () => await graph0.getEvenXAxisSpacing(),
      'Graph 0 did not update to even spacing'
    );
    await poll(
      async () => await graph1.getEvenXAxisSpacing(),
      'Graph 1 did not update to even spacing'
    );

    await graph0.clickEvenXAxisSpacingSwitch();

    await poll(
      async () => !(await graph0.getEvenXAxisSpacing()),
      'Graph 0 did not update to normal spacing'
    );
    await poll(
      async () => !(await graph1.getEvenXAxisSpacing()),
      'Graph 1 did not update to normal spacing'
    );
  });

  it('sets even x-axis spacing from URL parameter', async () => {
    await testBed.page.goto(`${testBed.baseUrl}?evenXAxisSpacing=true&manual_plot_mode=true`);
    const explorePO = new ExploreMultiSkPO((await testBed.page.$('explore-multi-sk'))!);
    const testPickerPO = explorePO.testPicker;

    await addGraph(testPickerPO, explorePO, { 0: ['arm'], 1: ['Android'] });
    await addGraph(testPickerPO, explorePO, { 0: ['arm'], 1: ['Ubuntu'] }, 2);

    const graph0 = explorePO.getGraph(0);
    const graph1 = explorePO.getGraph(1);

    expect(await graph0.getEvenXAxisSpacing()).to.be.true;
    expect(await graph1.getEvenXAxisSpacing()).to.be.true;
  });
});

describe('Split Graph Functionality', function () {
  this.timeout(60000);
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    const queryParams = '?begin=1687855198&end=1687961973';
    await testBed.page.goto(testBed.baseUrl + queryParams);
    await testBed.page.setViewport(STANDARD_LAPTOP_VIEWPORT);
  });

  it('splits graph when split-by checkbox is checked', async () => {
    const explorePO = new ExploreMultiSkPO((await testBed.page.$('explore-multi-sk'))!);
    const testPickerPO = explorePO.testPicker;

    // Plot first (Merged) to ensure two traces appear on one chart
    await addGraph(testPickerPO, explorePO, { 0: ['arm'], 1: ['Android', 'Ubuntu'] });
    const osField = await testPickerPO.getPickerField(1);

    const mergedGraphInitial = explorePO.getGraph(0);
    const tracesMergedInitial = await mergedGraphInitial.getTraceKeys();
    expect(tracesMergedInitial).to.have.lengthOf(2);

    // Click "Split" checkbox on OS field
    const splitCheckbox = osField.splitByCheckbox;
    // Ensure it is visible (poll until not hidden)
    await waitForElementNotHidden(splitCheckbox);

    await startUrlTracking(testBed.page);
    await osField.checkSplit();

    // Verify URL update
    await testBed.page.waitForTimeout(500);
    expect(await getUrlChangeCount(testBed.page)).to.equal(1);
    await expectUrlParams(testBed.page, { splitByKeys: 'os' });

    // Wait for 3 graphs (1 hidden summary + 2 splits)
    await explorePO.waitForGraphCount(3, LONG_TIMEOUT_MS);

    // Verify contents
    // Index 0 is hidden summary, check indices 1 and 2.
    const graph1 = explorePO.getGraph(1);
    // Wait for data to load in the first split graph
    await explorePO.waitForGraph(1, GRAPH_LOAD_TIMEOUT_MS);
    const traces1 = await graph1.getTraceKeys();

    const graph2 = explorePO.getGraph(2);
    // Wait for data to load in the second split graph
    await explorePO.waitForGraph(2, GRAPH_LOAD_TIMEOUT_MS);
    const traces2 = await graph2.getTraceKeys();

    // Collect all traces from all visible graphs
    const allTraces = [...traces1, ...traces2];
    expect(allTraces).to.include(',arch=arm,os=Android,');
    expect(allTraces).to.include(',arch=arm,os=Ubuntu,');
    expect(traces1.length).to.equal(1);
    expect(traces2.length).to.equal(1);
    expect(traces1[0]).to.not.equal(traces2[0]); // Ensure they are different

    // Uncheck "Split" checkbox to merge back
    await resetUrlChangeCount(testBed.page);
    await osField.uncheckSplit();

    // Verify URL update
    await testBed.page.waitForTimeout(500);
    // Unchecking might involve updating shortcut and then clearing split keys.
    expect(await getUrlChangeCount(testBed.page)).to.be.at.least(1);
    await expectUrlParams(testBed.page, { splitByKeys: null });

    // Wait for graph count to be 1
    await explorePO.waitForGraphCount(1, LONG_TIMEOUT_MS);
    const mergedGraph = explorePO.getGraph(0);
    await explorePO.waitForGraph(0);

    // Verify merged content
    const mergedTraces = await mergedGraph.getTraceKeys();
    expect(mergedTraces).to.have.lengthOf(2);
    expect(mergedTraces).to.include(',arch=arm,os=Android,');
    expect(mergedTraces).to.include(',arch=arm,os=Ubuntu,');

    // Verify anomalies are present
    await mergedGraph.verifyAnomaliesPresent();

    // Interact with anomaly tooltip
    await mergedGraph.clickFirstAnomaly(testBed.page);
    await mergedGraph.waitForAnomalyTooltip();
  });

  it('splits graph into more than 5 graphs and loads all of them (batch loading)', async () => {
    // Navigate with plotSummary=true
    const queryParams = '?begin=1687855198&end=1687961973&plotSummary=true';
    await testBed.page.goto(testBed.baseUrl + queryParams);
    await testBed.page.setViewport(STANDARD_LAPTOP_VIEWPORT);

    const explorePO = new ExploreMultiSkPO((await testBed.page.$('explore-multi-sk'))!);
    const testPickerPO = explorePO.testPicker;

    // Select 'arm' to filter down
    await testPickerPO.waitForPickerField(0);
    const archField = await testPickerPO.getPickerField(0);
    await archField.select('arm');
    await testPickerPO.waitForSpinnerInactive();

    // Select All OS options
    await testPickerPO.waitForPickerField(1);
    const osField = await testPickerPO.getPickerField(1);
    await waitForElementNotHidden(osField.selectAllCheckbox);
    await osField.checkAll();
    await testPickerPO.waitForSpinnerInactive();

    // Check Split by OS
    await waitForElementNotHidden(osField.splitByCheckbox);
    await osField.checkSplit();

    // Click Plot
    await testPickerPO.clickPlotButton();

    // Verify that the last graph (index 6) is loaded.
    // getGraph(0) is Summary. getGraph(1)..getGraph(6) are split graphs.
    await explorePO.waitForGraph(6, GRAPH_LOAD_TIMEOUT_MS);

    // Check that the last graph has traces.
    const lastGraph = explorePO.getGraph(6);
    const traces = await lastGraph.getTraceKeys();
    expect(traces.length).to.be.greaterThan(0);
  });
});

describe('Test Picker Interactions', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    const queryParams = '?begin=1687855198&end=1687961973';
    await testBed.page.goto(testBed.baseUrl + queryParams);
    await testBed.page.setViewport(STANDARD_LAPTOP_VIEWPORT);
  });

  it('checks All then plots graph', async () => {
    const explorePO = new ExploreMultiSkPO((await testBed.page.$('explore-multi-sk'))!);
    const testPickerPO = explorePO.testPicker;

    await testPickerPO.waitForPickerField(0);
    const archField = await testPickerPO.getPickerField(0);
    await archField.select('arm');
    await testPickerPO.waitForSpinnerInactive();

    await testPickerPO.waitForPickerField(1);
    const osField = await testPickerPO.getPickerField(1);

    await waitForElementNotHidden(osField.selectAllCheckbox);
    await osField.checkAll();
    await testPickerPO.waitForSpinnerInactive();

    await testPickerPO.clickPlotButton();
    await explorePO.waitForGraphCount(1);
    await explorePO.waitForGraph(0);

    expect(await explorePO.getGraph(0).getTraceKeys()).to.have.lengthOf(6);
  });

  it('splits before plotting and plots multiple graphs', async () => {
    const explorePO = new ExploreMultiSkPO((await testBed.page.$('explore-multi-sk'))!);
    const testPickerPO = explorePO.testPicker;

    await testPickerPO.waitForPickerField(0);
    await (await testPickerPO.getPickerField(0)).select('arm');
    await testPickerPO.waitForSpinnerInactive();

    await testPickerPO.waitForPickerField(1);
    const osField = await testPickerPO.getPickerField(1);
    await osField.select('Android');
    await testPickerPO.waitForSpinnerInactive();
    await osField.select('Ubuntu');
    await testPickerPO.waitForSpinnerInactive();

    await waitForElementNotHidden(osField.splitByCheckbox);
    await osField.checkSplit();

    await testPickerPO.clickPlotButton();

    await explorePO.waitForGraphCount(3, LONG_TIMEOUT_MS);

    const traces1 = await explorePO.getGraph(1).getTraceKeys();
    const traces2 = await explorePO.getGraph(2).getTraceKeys();

    expect(traces1).to.have.lengthOf(1);
    expect(traces2).to.have.lengthOf(1);
    expect([...traces1, ...traces2]).to.include(',arch=arm,os=Android,');
    expect([...traces1, ...traces2]).to.include(',arch=arm,os=Ubuntu,');
  });
});

describe('Trace Removal State Retention', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    // Start with a specific time range to verify it doesn't shift
    const queryParams = '?begin=1687855198&end=1687961973';
    await testBed.page.goto(testBed.baseUrl + queryParams);
    await testBed.page.setViewport(STANDARD_LAPTOP_VIEWPORT);
  });

  it('maintains state while removing traces one by one (merged mode)', async () => {
    const explorePO = new ExploreMultiSkPO((await testBed.page.$('explore-multi-sk'))!);
    const testPickerPO = explorePO.testPicker;

    // Plot a single graph with 3 traces
    await addGraph(testPickerPO, explorePO, { 0: ['arm'], 1: ['Android', 'Ubuntu', 'Debian11'] });

    // Graph 0 should have 3 traces
    await explorePO.waitForGraphCount(1, LONG_TIMEOUT_MS);
    const graphPO = explorePO.getGraph(0);

    await poll(async () => (await graphPO.getTraceKeys()).length === 3, 'Waiting for 3 traces');

    const osField = await testPickerPO.getPickerField(1);

    // Save initial URL state to verify range doesn't shift
    let url = new URL(testBed.page.url());
    const initialBegin = url.searchParams.get('begin');
    const initialEnd = url.searchParams.get('end');
    expect(initialBegin).to.not.be.null;

    // Remove Android
    await osField.removeSelectedOption('Android');

    // Verify trace removed from the plot
    await poll(async () => (await graphPO.getTraceKeys()).length === 2, 'Waiting for 2 traces');
    const tracesAfterAndroid = await graphPO.getTraceKeys();
    expect(tracesAfterAndroid).to.not.include(',arch=arm,os=Android,');

    // Verify URL range remains unchanged
    url = new URL(testBed.page.url());
    expect(url.searchParams.get('begin')).to.equal(initialBegin);
    expect(url.searchParams.get('end')).to.equal(initialEnd);

    // Remove Ubuntu
    await osField.removeSelectedOption('Ubuntu');

    await poll(async () => (await graphPO.getTraceKeys()).length === 1, 'Waiting for 1 trace');
    const tracesAfterUbuntu = await graphPO.getTraceKeys();
    expect(tracesAfterUbuntu).to.not.include(',arch=arm,os=Ubuntu,');

    // TODO(eduardoyap): Add test for final trace removal down to 0 traces
  });

  it('maintains state while removing traces one by one (split mode)', async () => {
    const explorePO = new ExploreMultiSkPO((await testBed.page.$('explore-multi-sk'))!);
    const testPickerPO = explorePO.testPicker;

    // Select 3 traces
    await testPickerPO.waitForPickerField(0);
    const archField = await testPickerPO.getPickerField(0);
    await archField.select('arm');
    await testPickerPO.waitForSpinnerInactive();

    await testPickerPO.waitForPickerField(1);
    const osField = await testPickerPO.getPickerField(1);
    await osField.select('Android');
    await testPickerPO.waitForSpinnerInactive();
    await osField.select('Ubuntu');
    await testPickerPO.waitForSpinnerInactive();
    await osField.select('Debian11');
    await testPickerPO.waitForSpinnerInactive();

    // Split it
    await waitForElementNotHidden(osField.splitByCheckbox);
    await osField.checkSplit();
    await testPickerPO.clickPlotButton();

    // Wait for 4 graphs (1 hidden summary + 3 split data graphs: Android, Ubuntu, Debian11)
    await explorePO.waitForGraphCount(4, LONG_TIMEOUT_MS);

    // Save initial URL state
    let url = new URL(testBed.page.url());
    const initialBegin = url.searchParams.get('begin');
    const initialEnd = url.searchParams.get('end');
    expect(initialBegin).to.not.be.null;

    // Remove Android
    await osField.removeSelectedOption('Android');

    // Verify graph count dropped to 3 (Summary + 2 splits: Ubuntu, Debian11)
    await explorePO.waitForGraphCount(3, LONG_TIMEOUT_MS);
    // Verify URL range
    url = new URL(testBed.page.url());
    expect(url.searchParams.get('begin')).to.equal(initialBegin);
    expect(url.searchParams.get('end')).to.equal(initialEnd);

    // TODO(eduardoyap): Add tests for removing the final traces (e.g., removing Ubuntu and Debian11)
    // once the bug in ExploreMultiSk._onRemoveTrace (which incorrectly clears the summary graph query during split mode) is fixed.
  });
});
