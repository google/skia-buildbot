import { expect } from 'chai';
import {
  loadCachedTestBed,
  takeScreenshot,
  TestBed,
} from '../../../puppeteer-tests/util';
import { LitElement } from 'lit';

describe('plot-summary-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    // only show three summary bar, don't show the events as they might be flaky.
    await testBed.page.setViewport({ width: 400, height: 450 });
  });

  // y position for three plots.
  const plots = [120, 230, 340];
  const select = async (x: number, offset: number, plot: number) => {
    await testBed.page.mouse.move(x, plots[plot]);
    await testBed.page.mouse.down();
    await testBed.page.mouse.move(x + offset, plots[plot]);
    await testBed.page.mouse.up();
  };

  const waitPlotSummary = (sel: string) =>
    testBed.page.$eval(sel, (plot) => (plot as LitElement).updateComplete);

  const getEventJSON = () =>
    testBed.page.$eval('#events', (node) => node.textContent);

  ['#plot1'].forEach((plot, idx) =>
    describe(`the element ${plot}`, () => {
      // We move the selection box that always ends up with those two
      // timestamps. They are roughly around Oct 1st, 2023.
      // See demo.ts how the dataframe is generated.
      const start = 1696180344,
        end = 1696255100;

      // The timestamp tolerance from UI selection, this is 120 second in
      // the span of 3 days, the tolerance here should be negligible.
      const tolerance = 120;

      it('draw from left to right', async () => {
        await waitPlotSummary(plot);
        await select(100, 120, idx);
        const json = await getEventJSON();
        const detail = JSON.parse(json!);
        expect(detail.valueStart).to.be.approximately(start, tolerance);
        expect(detail.valueEnd).to.be.approximately(end, tolerance);
        expect(detail.domain).to.be.equal('date');
      });

      it('draw from right to left', async () => {
        await waitPlotSummary(plot);
        await select(220, -120, idx);
        const json = await getEventJSON();
        const detail = JSON.parse(json!);
        expect(detail.valueStart).to.be.approximately(start, tolerance);
        expect(detail.valueEnd).to.be.approximately(end, tolerance);
      });

      it('draw and move the selection', async () => {
        await waitPlotSummary(plot);
        await select(240, -120, idx);
        await select(200, -20, idx);
        const json = await getEventJSON();
        const detail = JSON.parse(json!);
        expect(detail.valueStart).to.be.approximately(start, tolerance);
        expect(detail.valueEnd).to.be.approximately(end, tolerance);
      });
    })
  );

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await waitPlotSummary('#plot1');
      await takeScreenshot(testBed.page, 'perf', 'plot-summary-default-state');
    });
    it('makes a selection on the graph', async () => {
      await waitPlotSummary('#plot1');
      await select(100, 200, 0);
      await select(230, -40, 1);
      await select(250, 50, 2);
      await takeScreenshot(testBed.page, 'perf', 'plot-summary-select');
    });
  });
});
