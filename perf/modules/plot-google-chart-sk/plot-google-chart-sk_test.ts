import './index';
import { assert, expect } from 'chai';
import { PlotGoogleChartSk } from './plot-google-chart-sk';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';

// TODO(b/362831653): Add unit tests for plot-google-chart
describe('plot-google-chart-sk', () => {
  // trace samples for determineYAxisTitle unit tests
  const ms_down = 'unit=ms,improvement_direction=down';
  const ms_up = 'unit=ms,improvement_direction=up';
  const score_down = 'unit=score,improvement_direction=down';

  const newInstance = setUpElementUnderTest<PlotGoogleChartSk>('plot-google-chart-sk');
  let element: PlotGoogleChartSk;

  beforeEach(() => {
    element = newInstance(() => {});
  });

  describe('some action', () => {
    it('some result', () => {
      // eslint-disable-next-line @typescript-eslint/no-unused-expressions
      expect(element).to.not.be.null;
    });
  });

  describe('determineYAxisTitle', () => {
    it('empty', () => {
      assert.isEmpty(element.determineYAxisTitle([]));
    });

    it('unit and improvement direction same', () => {
      assert.strictEqual('ms - down', element.determineYAxisTitle([ms_down, ms_down]));
    });

    it('unit same, improvement direction different', () => {
      assert.strictEqual('ms', element.determineYAxisTitle([ms_down, ms_up]));
    });

    it('unit different, improvement direction same', () => {
      assert.strictEqual('down', element.determineYAxisTitle([ms_down, score_down]));
    });

    it('all different', () => {
      assert.isEmpty(element.determineYAxisTitle([ms_up, score_down]));
    });
  });
});
