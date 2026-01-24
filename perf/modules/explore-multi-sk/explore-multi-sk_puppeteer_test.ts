import { expect } from 'chai';
import { loadCachedTestBed, TestBed } from '../../../puppeteer-tests/util';
import { ExploreMultiSkPO } from './explore-multi-sk_po';
import {
  STANDARD_LAPTOP_VIEWPORT,
  poll,
  waitForElementNotHidden,
} from '../common/puppeteer-test-util';

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

    // Wait for the test picker to populate.
    // Order based on include_params: ['arch', 'os']
    // 1. Arch
    await testPickerPO.waitForPickerField(0);
    const archField = await testPickerPO.getPickerField(0);
    await archField.select('arm');
    await testPickerPO.waitForSpinnerInactive();

    // 2. OS
    await testPickerPO.waitForPickerField(1);
    const osField = await testPickerPO.getPickerField(1);
    await osField.select('Android');
    await testPickerPO.waitForSpinnerInactive();
    await osField.select('Ubuntu');
    await testPickerPO.waitForSpinnerInactive();

    // Click Plot
    await testPickerPO.clickPlotButton();
    await explorePO.waitForGraph();

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

    // Remove Android trace
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
    await testBed.page.mouse.move(coords.x, coords.y);

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

    // 1. Arch
    await testPickerPO.waitForPickerField(0);
    const archField = await testPickerPO.getPickerField(0);
    await archField.select('arm');
    await testPickerPO.waitForSpinnerInactive();

    // 2. OS
    await testPickerPO.waitForPickerField(1);
    const osField = await testPickerPO.getPickerField(1);
    await osField.select('Android');
    await testPickerPO.waitForSpinnerInactive();

    // Click Plot
    await testPickerPO.clickPlotButton();
    await explorePO.waitForGraph();

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
    await exploreSimplePO.xAxisSwitch.applyFnToDOMNode((el) => (el as HTMLElement).click());

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

    // ==========================================
    // 1. ADD FIRST GRAPH (Selection: Android)
    // ==========================================
    await testPickerPO.waitForPickerField(0);
    const archField = await testPickerPO.getPickerField(0);
    await archField.select('arm');
    await testPickerPO.waitForSpinnerInactive();

    await testPickerPO.waitForPickerField(1);
    const osField = await testPickerPO.getPickerField(1);
    await osField.select('Android');
    await testPickerPO.waitForSpinnerInactive();

    await testPickerPO.clickPlotButton();
    await explorePO.waitForGraph(0);

    // Verify First Graph (Index 0)
    expect(await explorePO.getGraphCount()).to.equal(1);
    const graph1PO = explorePO.getGraph(0);
    const traces1 = await graph1PO.getTraceKeys();
    expect(traces1).to.include(',arch=arm,os=Android,');

    let currentUrl = new URL(await testBed.page.url());
    expect(currentUrl.searchParams.get('totalGraphs')).to.equal('1');

    // ==========================================
    // 2. ADD SECOND GRAPH (Selection: Android + Ubuntu)
    // ==========================================
    // We select 'Ubuntu'. In this mode, 'Android' remains selected in the picker.
    // The picker state is now: { arch: 'arm', os: ['Android', 'Ubuntu'] }
    await osField.select('Ubuntu');
    await testPickerPO.waitForSpinnerInactive();

    await testPickerPO.clickPlotButton();

    await explorePO.waitForGraphCount(2);
    await explorePO.waitForGraph(0);

    // --- VERIFY TOP GRAPH (Index 0) ---
    // Should reflect the CURRENT picker state (Android + Ubuntu)
    const graphTopPO = explorePO.getGraph(0);
    const tracesTop = await graphTopPO.getTraceKeys();
    expect(tracesTop).to.include(',arch=arm,os=Ubuntu,');
    expect(tracesTop).to.include(',arch=arm,os=Android,');

    // --- VERIFY BOTTOM GRAPH (Index 1) ---
    // Should remain a snapshot of the PAST state (Only Android)
    // This proves the old graph wasn't mutated by the new plot action
    const graphBottomPO = explorePO.getGraph(1);
    const tracesBottom = await graphBottomPO.getTraceKeys();
    expect(tracesBottom).to.include(',arch=arm,os=Android,');
    expect(tracesBottom).to.not.include(',arch=arm,os=Ubuntu,');

    // Check URL state after adding the second graph
    currentUrl = new URL(await testBed.page.url());
    expect(currentUrl.searchParams.get('totalGraphs')).to.equal('2');
  });

  it('adds three graphs and removes them in specific order (middle, first, last)', async () => {
    const explorePO = new ExploreMultiSkPO((await testBed.page.$('explore-multi-sk'))!);
    const testPickerPO = explorePO.testPicker;

    // SETUP: CREATE 3 DISTINCT GRAPHS
    // Graph 1: Arch=arm, OS=Android
    await testPickerPO.waitForPickerField(0);
    const archField = await testPickerPO.getPickerField(0);
    await archField.select('arm');
    await testPickerPO.waitForSpinnerInactive();

    await testPickerPO.waitForPickerField(1);
    const osField = await testPickerPO.getPickerField(1);
    await osField.select('Android');
    await testPickerPO.waitForSpinnerInactive();

    await testPickerPO.clickPlotButton();
    await explorePO.waitForGraph(0);

    // Graph 2: Arch=arm, OS=Ubuntu
    await osField.clear();
    await osField.select('Ubuntu');
    await testPickerPO.waitForSpinnerInactive();
    await testPickerPO.clickPlotButton();
    await explorePO.waitForGraphCount(2);
    await explorePO.waitForGraph(0);

    // Graph 3: Arch=x86, OS=Ubuntu
    await osField.clear();
    await osField.select('Android');
    await testPickerPO.clickPlotButton();
    await explorePO.waitForGraphCount(3);
    await explorePO.waitForGraph(0);

    // VERIFY INITIAL STATE
    // Index 0 (Newest): Android
    // Index 1 (Middle): Ubuntu
    // Index 2 (Oldest): Android (Graph 1)

    // Verify URL has 3 graphs
    let currentUrl = new URL(await testBed.page.url());
    expect(currentUrl.searchParams.get('totalGraphs')).to.equal('3');
    expect(currentUrl.searchParams.get('shortcut')).to.not.be.null;

    // Verify Traces match expected position
    const traces0 = await explorePO.getGraph(0).getTraceKeys();
    expect(traces0).to.have.lengthOf(1);
    expect(traces0[0]).to.equal(',arch=arm,os=Android,');

    const traces1 = await explorePO.getGraph(1).getTraceKeys();
    expect(traces1).to.have.lengthOf(1);
    expect(traces1[0]).to.equal(',arch=arm,os=Ubuntu,');

    const traces2 = await explorePO.getGraph(2).getTraceKeys();
    expect(traces2).to.have.lengthOf(1);
    expect(traces2[0]).to.equal(',arch=arm,os=Android,');

    // REMOVE MIDDLE
    const middleGraph = explorePO.getGraph(1);
    await middleGraph.clickRemoveAllButton();
    await explorePO.waitForGraphCount(2);

    const tracesPostRem1_0 = await explorePO.getGraph(0).getTraceKeys();
    expect(tracesPostRem1_0).to.have.lengthOf(1);
    expect(tracesPostRem1_0[0]).to.equal(',arch=arm,os=Android,');

    const tracesPostRem1_1 = await explorePO.getGraph(1).getTraceKeys();
    expect(tracesPostRem1_1).to.have.lengthOf(1);
    expect(tracesPostRem1_1[0]).to.equal(',arch=arm,os=Android,');

    currentUrl = new URL(await testBed.page.url());
    expect(currentUrl.searchParams.get('totalGraphs')).to.equal('2');

    // REMOVE FIRST
    const firstGraph = explorePO.getGraph(0);
    await firstGraph.clickRemoveAllButton();

    await explorePO.waitForGraphCount(1);

    const tracesPostRem2_0 = await explorePO.getGraph(0).getTraceKeys();
    expect(tracesPostRem2_0).to.have.lengthOf(1);
    expect(tracesPostRem2_0[0]).to.equal(',arch=arm,os=Android,');

    currentUrl = new URL(await testBed.page.url());
    expect(currentUrl.searchParams.get('totalGraphs')).to.equal('1');

    // REMOVE LAST
    const lastGraph = explorePO.getGraph(0);
    await lastGraph.clickRemoveAllButton();

    await explorePO.waitForGraphCount(0);

    currentUrl = new URL(await testBed.page.url());
    expect(currentUrl.searchParams.get('totalGraphs')).to.be.null;

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

      await testPickerPO.waitForPickerField(0);
      const archField = await testPickerPO.getPickerField(0);
      await archField.select('arm');
      await testPickerPO.waitForSpinnerInactive();

      await testPickerPO.waitForPickerField(1);
      const osField = await testPickerPO.getPickerField(1);
      await osField.select('Android');
      await testPickerPO.waitForSpinnerInactive();

      await testPickerPO.clickPlotButton();
      await explorePO.waitForGraph(0);
      const graph0 = explorePO.getGraph(0);
      const plotSummaryPO0 = graph0.plotSummary;
      await plotSummaryPO0.waitForPlotSummaryToLoad();

      await osField.clear();
      await osField.select('Ubuntu');
      await testPickerPO.waitForSpinnerInactive();
      await testPickerPO.clickPlotButton();
      await explorePO.waitForGraphCount(2);
      await explorePO.waitForGraph(0);
      const graph1 = explorePO.getGraph(0);
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

    await testPickerPO.waitForPickerField(0);
    const archField = await testPickerPO.getPickerField(0);
    await archField.select('arm');
    await testPickerPO.waitForSpinnerInactive();

    await testPickerPO.waitForPickerField(1);
    const osField = await testPickerPO.getPickerField(1);
    await osField.select('Android');
    await testPickerPO.waitForSpinnerInactive();

    await testPickerPO.clickPlotButton();
    await explorePO.waitForGraph(0);

    const simpleGraphPO = explorePO.getGraph(0);
    const plotSummaryPO = simpleGraphPO.plotSummary;
    await plotSummaryPO.waitForPlotSummaryToLoad();

    const initialUrl = new URL(testBed.page.url());
    const initialBegin = initialUrl.searchParams.get('begin');
    const initialEnd = initialUrl.searchParams.get('end');

    await plotSummaryPO.resizeSelection(testBed.page, 'right', 0.75);

    const finalUrl = new URL(testBed.page.url());
    const finalBegin = finalUrl.searchParams.get('begin');
    const finalEnd = finalUrl.searchParams.get('end');

    expect(finalBegin).to.not.equal(initialBegin);
    expect(finalEnd).to.not.equal(initialEnd);

    expect(Number(finalBegin)).to.not.be.NaN;
    expect(Number(finalEnd)).to.not.be.NaN;
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

    // 1. Select Arch = arm
    await testPickerPO.waitForPickerField(0);
    const archField = await testPickerPO.getPickerField(0);
    await archField.select('arm');
    await testPickerPO.waitForSpinnerInactive();

    // 2. Select OS = Android, Ubuntu
    await testPickerPO.waitForPickerField(1);
    const osField = await testPickerPO.getPickerField(1);
    await osField.select('Android');
    await testPickerPO.waitForSpinnerInactive();
    await osField.select('Ubuntu');
    await testPickerPO.waitForSpinnerInactive();

    // 3. Plot first (Merged) to ensure two traces appear on one chart
    await testPickerPO.clickPlotButton();
    await explorePO.waitForGraph(0);
    const mergedGraphInitial = explorePO.getGraph(0);
    const tracesMergedInitial = await mergedGraphInitial.getTraceKeys();
    expect(tracesMergedInitial).to.have.lengthOf(2);

    // 4. Click "Split" checkbox on OS field
    const splitCheckbox = osField.splitByCheckbox;
    // Ensure it is visible (poll until not hidden)
    await waitForElementNotHidden(splitCheckbox);

    await osField.checkSplit();

    // 5. Wait for 2 graphs
    await explorePO.waitForGraphCount(2);

    // 6. Verify contents
    const graph0 = explorePO.getGraph(0);
    // Wait for data to load in the first graph
    await explorePO.waitForGraph(0);
    const traces0 = await graph0.getTraceKeys();

    const graph1 = explorePO.getGraph(1);
    // Wait for data to load in the second graph
    await explorePO.waitForGraph(1);
    const traces1 = await graph1.getTraceKeys();

    // Collect all traces from all graphs
    const allTraces = [...traces0, ...traces1];
    expect(allTraces).to.include(',arch=arm,os=Android,');
    expect(allTraces).to.include(',arch=arm,os=Ubuntu,');
    expect(traces0.length).to.equal(1);
    expect(traces1.length).to.equal(1);
    expect(traces0[0]).to.not.equal(traces1[0]); // Ensure they are different

    // 7. Uncheck "Split" checkbox to merge back
    await osField.uncheckSplit();

    // 8. Wait for graph count to be 1
    await explorePO.waitForGraphCount(1);
    const mergedGraph = explorePO.getGraph(0);
    await explorePO.waitForGraph(0);

    // 9. Verify merged content
    const mergedTraces = await mergedGraph.getTraceKeys();
    expect(mergedTraces).to.have.lengthOf(2);
    expect(mergedTraces).to.include(',arch=arm,os=Android,');
    expect(mergedTraces).to.include(',arch=arm,os=Ubuntu,');

    // 10. Verify anomalies are present
    await mergedGraph.verifyAnomaliesPresent();

    // 11. Interact with anomaly tooltip
    await mergedGraph.clickFirstAnomaly(testBed.page);
    await mergedGraph.waitForAnomalyTooltip();
  });
});
