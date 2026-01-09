import { expect } from 'chai';
import { loadCachedTestBed, takeScreenshot, TestBed } from '../../../puppeteer-tests/util';
import { ElementHandle } from 'puppeteer';
import { PlotGoogleChartSkPO } from './plot-google-chart-sk_po';
import { SidePanelSkPO } from './side-panel-sk_po';

describe('plot-google-chart-sk', () => {
  let testBed: TestBed;
  let plotGoogleChartSk: ElementHandle;
  let plotGoogleChartSkPO: PlotGoogleChartSkPO;
  let sidePanelSkPO: SidePanelSkPO;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport({ width: 800, height: 600 });
    plotGoogleChartSk = (await testBed.page.$('plot-google-chart-sk'))!;
    if (!plotGoogleChartSk) {
      throw new Error('plot-google-chart-sk not found');
    }

    // Set up sample data for the chart to be visible
    await plotGoogleChartSk.evaluate((el) => {
      // The 'data' property expects a google.visualization.DataTable.
      // We can create one in the browser context for the test.
      // This requires the 'google' global to be available on the test page.
      const sampleData = new (window as any).google.visualization.DataTable();
      sampleData.addColumn('number', 'Commit');
      sampleData.addColumn('number', 'Trace 1');
      sampleData.addRows([
        [1, 10],
        [2, 12],
        [3, 8],
        [4, 15],
      ]);
      (el as any).data = sampleData;
    });

    plotGoogleChartSkPO = new PlotGoogleChartSkPO(plotGoogleChartSk);
  });

  it('should render the chart', async () => {
    expect(plotGoogleChartSkPO).to.exist;
    expect(await plotGoogleChartSkPO.isChartVisible()).to.be.true;

    await takeScreenshot(testBed.page, 'plot-google-chart-sk', 'default-render');
  });

  it('should display chart', async () => {
    // The component should now render the chart.
    expect(await plotGoogleChartSkPO.isChartVisible()).to.be.true;

    const chartObject = await plotGoogleChartSkPO.getGoogleChartObject();
    expect(chartObject).to.not.be.null;

    await takeScreenshot(testBed.page, 'plot-google-chart-sk', 'with-data');
  });

  it('should render the demo page (smoke test)', async () => {
    expect(await testBed.page.$$('plot-google-chart-sk')).to.have.length(1);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'perf', 'plot-google-chart-sk');
    });
  });

  it('should get chart type', async () => {
    // Assuming the chart type is set as 'line' in the demo or default
    expect(await plotGoogleChartSkPO.getChartType()).to.equal('line');
  });

  it('should show reset button after zoom and hide after click', async () => {
    // Simulate a zoom action to make the reset button visible
    // This requires interacting with the chart. Since we can't directly trigger a zoom
    // via a simple method without full chart interaction, we'll simulate setting
    // the property that would normally make it visible.
    await plotGoogleChartSk.evaluate(async (el: any) => {
      el.showResetButton = true;
    });

    expect(await plotGoogleChartSkPO.isResetButtonVisible()).to.be.true;

    await plotGoogleChartSkPO.clickResetButton();

    expect(await plotGoogleChartSkPO.isResetButtonVisible()).to.be.false;
  });

  describe('side-panel-sk', () => {
    beforeEach(async () => {
      // This function runs in the browser, finds the <side-panel-sk> element,
      // and waits for it to be ready.
      const sidePanelElementHandle = await testBed.page.evaluateHandle(async () => {
        // Wait for the components to be defined.
        await Promise.all([
          customElements.whenDefined('side-panel-sk-demo'),
          customElements.whenDefined('dataframe-repository-sk'),
          customElements.whenDefined('side-panel-sk'),
        ]);

        const demo = document.querySelector('side-panel-sk-demo')!;
        await (demo as any).updateComplete;

        const repo = demo.shadowRoot!.querySelector('dataframe-repository-sk')!;
        await (repo as any).updateComplete;

        // Wait until the repository is done loading data.
        await new Promise<void>((resolve) => {
          const checkForLoading = () => {
            if (!(repo as any).loading) {
              resolve();
            } else {
              setTimeout(checkForLoading, 100);
            }
          };
          checkForLoading();
        });

        const sidePanel = repo.querySelector('side-panel-sk')!;
        await (sidePanel as any).updateComplete;

        return sidePanel;
      });

      sidePanelSkPO = new SidePanelSkPO(sidePanelElementHandle as ElementHandle<Element>);
    });

    it('should have the panel open by default', async () => {
      expect(await sidePanelSkPO.isPanelOpen()).to.be.true;
    });

    it('should toggle the panel closed and open it', async () => {
      // The panel is open by default.
      expect(await sidePanelSkPO.isPanelOpen()).to.be.true;

      // First toggle should close
      let eventPromise = testBed.page.evaluate(
        () =>
          new Promise((resolve) =>
            document.addEventListener(
              'side-panel-toggle',
              (e) => resolve((e as CustomEvent).detail),
              {
                once: true,
              }
            )
          )
      );
      await sidePanelSkPO.clickToggle();
      expect(await sidePanelSkPO.isPanelOpen()).to.be.false;
      let eventDetail = await eventPromise;
      expect(eventDetail).to.deep.equal({ open: false });

      // Second toggle should open
      eventPromise = testBed.page.evaluate(
        () =>
          new Promise((resolve) =>
            document.addEventListener(
              'side-panel-toggle',
              (e) => resolve((e as CustomEvent).detail),
              {
                once: true,
              }
            )
          )
      );
      await sidePanelSkPO.clickToggle();
      expect(await sidePanelSkPO.isPanelOpen()).to.be.true;
      eventDetail = await eventPromise;
      expect(eventDetail).to.deep.equal({ open: true });
    });
  });
});
