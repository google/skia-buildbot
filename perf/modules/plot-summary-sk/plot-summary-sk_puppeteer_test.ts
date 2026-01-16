import { expect } from 'chai';

import { loadCachedTestBed, takeScreenshot, TestBed } from '../../../puppeteer-tests/util';
import { LitElement } from 'lit';
import { PlotSummarySkSelectionEventDetails } from './plot-summary-sk';
import { PlotSummarySkPO } from './plot-summary-sk_po';

describe('plot-summary-sk', () => {
  let testBed: TestBed;

  before(async () => {
    testBed = await loadCachedTestBed();
  });

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    // Show the four summary bars. Don't show the events as they might be flaky.
    await testBed.page.setViewport({ width: 400, height: 1000 });
    return testBed.page.waitForFunction(
      () => document.querySelector('#events')?.textContent === 'ready',
      { timeout: 5000 }
    );
  });

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
  ].forEach((plot) =>
    describe(`the element ${plot.id}`, () => {
      let plotSummarySkPO: PlotSummarySkPO;

      beforeEach(async () => {
        const element = await testBed.page.$(plot.id);
        if (!element) {
          throw new Error(`Element ${plot.id} not found`);
        }
        const boundingBox = await element.boundingBox();
        plotSummarySkPO = new PlotSummarySkPO(element);
        plotSummarySkPO.boundingBox = boundingBox;
      });

      // We move the selection box that always ends up with those two
      // timestamps. They are roughly around Oct 1st, 2023.
      // See demo.ts how the dataframe is generated.
      const start = plot.start;
      const end = plot.end;

      const tolerance = plot.tolerance;

      it('draw from left to right', async () => {
        await waitPlotSummary(plot.id);
        await plotSummarySkPO.selectRange(testBed.page, 0.24, 0.552);
        const json = await getEventText();
        const detail = JSON.parse(json!) as PlotSummarySkSelectionEventDetails;

        expect(detail.value.begin).to.be.approximately(start, tolerance);
        expect(detail.value.end).to.be.approximately(end, tolerance);
      });

      it('draw from right to left', async () => {
        await waitPlotSummary(plot.id);
        await plotSummarySkPO.selectRange(testBed.page, 0.552, 0.24);
        const json = await getEventText();
        const detail = JSON.parse(json!) as PlotSummarySkSelectionEventDetails;

        expect(detail.value.begin).to.be.approximately(start, tolerance);
        expect(detail.value.end).to.be.approximately(end, tolerance);
      });

      it('draw and move the selection', async () => {
        await waitPlotSummary(plot.id);
        await plotSummarySkPO.selectRange(testBed.page, 0.604, 0.292);
        await plotSummarySkPO.selectRange(testBed.page, 0.5, 0.448);

        const json = await getEventText();
        const detail = JSON.parse(json!) as PlotSummarySkSelectionEventDetails;

        expect(detail.value.begin).to.be.approximately(start, tolerance);
        expect(detail.value.end).to.be.approximately(end, tolerance);
      });
    })
  );

  describe('load buttons', () => {
    let plotSummarySkPO: PlotSummarySkPO;

    beforeEach(async () => {
      // Use #plot1 for these tests, assuming buttons are present.
      const element = await testBed.page.$('#plot1');
      if (!element) {
        throw new Error('Element #plot1 not found');
      }
      plotSummarySkPO = new PlotSummarySkPO(element);

      // Enable controls for this test
      await testBed.page.evaluate(() => {
        const plot = document.querySelector('#plot1') as any;
        if (plot) {
          plot.hasControl = true;
          // Mock dfRepo to ensure buttons render
          if (!plot.dfRepo) {
            plot.dfRepo = { extendRange: async () => {}, commitRange: { begin: 0, end: 0 } };
          }
        }
      });
      await waitPlotSummary('#plot1');

      // Check if buttons are rendered
      const leftButtonExists = await plotSummarySkPO.leftLoadButton
        .isEmpty()
        .then((empty) => !empty);
      const rightButtonExists = await plotSummarySkPO.rightLoadButton
        .isEmpty()
        .then((empty) => !empty);

      expect(leftButtonExists).to.be.true;
      expect(rightButtonExists).to.be.true;
    });

    it('can click the left load button', async () => {
      await plotSummarySkPO.clickLeftLoad();
      expect(true).to.be.true; // Indicate test passed if no error
    });

    it('can click the right load button', async () => {
      await plotSummarySkPO.clickRightLoad();
      expect(true).to.be.true; // Indicate test passed if no error
    });
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      await takeScreenshot(testBed.page, 'perf', 'plot-summary-default-state');
    });
  });
});
