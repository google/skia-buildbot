import { expect } from 'chai';
import { loadCachedTestBed, TestBed } from '../../../puppeteer-tests/util';
import { ExploreMultiSkPO } from './explore-multi-sk_po';
import { STANDARD_LAPTOP_VIEWPORT, poll } from '../common/puppeteer-test-util';

describe('Anomalies and Traces', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
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
