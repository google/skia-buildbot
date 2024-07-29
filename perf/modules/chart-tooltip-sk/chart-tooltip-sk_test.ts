import './index';
import { assert } from 'chai';
import { ChartTooltipSk } from './chart-tooltip-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { Anomaly, CommitNumber } from '../json';

describe('chart-tooltip-sk', () => {
  const newInstance = setUpElementUnderTest<ChartTooltipSk>('chart-tooltip-sk');

  let element: ChartTooltipSk;
  beforeEach(() => {
    // element = newInstance((el: ChartTooltipSk) => {
    //   // Place here any code that must run after the element is instantiated but
    //   // before it is attached to the DOM (e.g. property setter calls,
    //   // document-level event listeners, etc.).
    // });
    element = newInstance();
  });

  const test_name =
    'ChromiumPerf/win-11-perf/webrtc/cpuTimeMetric_duration_std/multiple_peerconnections';
  const y_value = 100;
  const commit_position = CommitNumber(12345);

  const dummyAnomaly = (bugId: number): Anomaly => ({
    id: 1,
    test_path: '',
    bug_id: bugId,
    start_revision: 1234,
    end_revision: 1239,
    is_improvement: false,
    recovered: true,
    state: '',
    statistic: '',
    units: '',
    degrees_of_freedom: 0,
    median_before_anomaly: 75.209091,
    median_after_anomaly: 100.5023,
    p_value: 0,
    segment_size_after: 0,
    segment_size_before: 0,
    std_dev_before_anomaly: 0,
    t_statistic: 0,
  });

  describe('set fields', () => {
    it('anomalies should be set', () => {
      element.load(
        test_name,
        y_value,
        commit_position,
        dummyAnomaly(12345),
        null,
        false
      );
      assert.equal(element.test_name, test_name);
      assert.equal(element.y_value, y_value);
      assert.equal(element.commit_position, commit_position);
      assert.isNotNull(element.anomaly);
    });
  });
});
