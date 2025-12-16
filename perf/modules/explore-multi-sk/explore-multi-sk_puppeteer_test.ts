import { expect } from 'chai';
import { loadCachedTestBed, TestBed } from '../../../puppeteer-tests/util';
import { ExploreMultiSkPO } from './explore-multi-sk_po';
import { STANDARD_LAPTOP_VIEWPORT } from '../common/puppeteer-test-util';

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
});
