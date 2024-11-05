import { expect } from 'chai';
import { loadCachedTestBed, takeScreenshot, TestBed } from '../../../puppeteer-tests/util';
import { LitElement } from 'lit';
import { PlotSummarySkSelectionEventDetails } from './plot-summary-sk';

describe('plot-summary-sk', () => {
  let testBed: TestBed;

  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    // only show three summary bar, don't show the events as they might be flaky.
    await testBed.page.setViewport({ width: 400, height: 600 });
    return testBed.page.waitForFunction(
      () => document.querySelector('#events')?.textContent === 'ready',
      { timeout: 5000 }
    );
  });

  // y position for three plots.
  const plots = [120, 230, 370, 490];
  const select = async (x: number, offset: number, plot: number) => {
    await testBed.page.mouse.move(x, plots[plot]);
    await testBed.page.mouse.down();
    await testBed.page.mouse.move(x + offset, plots[plot]);
    await testBed.page.mouse.up();
  };

  const waitPlotSummary = (sel: string) =>
    testBed.page.$eval(sel, (plot) => (plot as LitElement).updateComplete);

  const getEventText = () => testBed.page.$eval('#events', (node) => node.textContent);

  // The timestamp tolerance from UI selection, this is 120 second in
  // the span of 3 days, the tolerance here should be negligible.
  // The offset/commit tolerance should be more rigid as there is only a few numbers.
  [
    { id: '#plot1', start: 1696177743, end: 1696255571, tolerance: 120 },
    { id: '#plot2', start: 104.539, end: 110.492, tolerance: 1e-2 },
    { id: '#plot3', start: 1696142266, end: 1696173566, tolerance: 120 },
    { id: '#plot4', start: 123.651, end: 154.669, tolerance: 1e-2 },
  ].forEach((plot, idx) =>
    describe(`the element ${plot.id}`, () => {
      // We move the selection box that always ends up with those two
      // timestamps. They are roughly around Oct 1st, 2023.
      // See demo.ts how the dataframe is generated.
      const start = plot.start,
        end = plot.end;

      const tolerance = plot.tolerance;

      it('draw from left to right', async () => {
        await waitPlotSummary(plot.id);
        await select(100, 120, idx);
        const json = await getEventText();
        const detail = JSON.parse(json!) as PlotSummarySkSelectionEventDetails;

        expect(detail.value.begin).to.be.approximately(start, tolerance);
        expect(detail.value.end).to.be.approximately(end, tolerance);
      });

      it('draw from right to left', async () => {
        await waitPlotSummary(plot.id);
        await select(220, -120, idx);
        const json = await getEventText();
        const detail = JSON.parse(json!) as PlotSummarySkSelectionEventDetails;

        expect(detail.value.begin).to.be.approximately(start, tolerance);
        expect(detail.value.end).to.be.approximately(end, tolerance);
      });

      it('draw and move the selection', async () => {
        await waitPlotSummary(plot.id);
        await select(240, -120, idx);
        await select(200, -20, idx);
        const json = await getEventText();
        const detail = JSON.parse(json!) as PlotSummarySkSelectionEventDetails;

        expect(detail.value.begin).to.be.approximately(start, tolerance);
        expect(detail.value.end).to.be.approximately(end, tolerance);
      });
    })
  );

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'perf', 'plot-summary-default-state');
    });
    it('makes a selection on the graph', async () => {
      await select(100, 200, 0);
      await select(230, -40, 1);
      await select(250, 50, 2);
      await select(100, 120, 3);

      await takeScreenshot(testBed.page, 'perf', 'plot-summary-select');
    });
  });
});
