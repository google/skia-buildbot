import { assert } from 'chai';
import {
  Anomaly,
  AnomalyMap,
  ColumnHeader,
  DataFrame,
  TraceSet,
} from '../json';
import { AnomalyData } from '../plot-simple-sk/plot-simple-sk';
import { getAnomalyDataMap } from './anomaly-sk';

const dummyAnomaly = (): Anomaly => {
  return {
    id: 0,
    test_path: '',
    bug_id: -1,
    start_revision: 0,
    end_revision: 3,
    is_improvement: false,
    recovered: true,
    state: '',
    statistic: '',
    units: '',
    degrees_of_freedom: 0,
    median_before_anomaly: 0,
    median_after_anomaly: 0,
    p_value: 0,
    segment_size_after: 0,
    segment_size_before: 0,
    std_dev_before_anomaly: 0,
    t_statistic: 0,
  };
};

describe('getAnomalyDataMap', () => {
  const header: ColumnHeader[] = [
    {
      offset: 99,
      timestamp: 0,
    },
    {
      offset: 100,
      timestamp: 0,
    },
    {
      offset: 101,
      timestamp: 0,
    },
  ];
  const traceset: TraceSet = {
    traceA: [5, 5, 15],
    traceB: [1, 1, 4],
  };
  const dataframe: DataFrame = {
    traceset: traceset,
    header: header,
    skip: 0,
    paramset: {},
  };
  const anomalyA: Anomaly = dummyAnomaly();
  const anomalyB: Anomaly = dummyAnomaly();
  const anomalymap: AnomalyMap = {
    traceA: { 101: anomalyA },
    traceB: { 101: anomalyB },
  };
  const expectedAnomalyDataMap: { [key: string]: AnomalyData[] } = {
    traceA: [
      {
        x: 2,
        y: 15,
        anomaly: anomalyA,
      },
    ],
    traceB: [
      {
        x: 2,
        y: 4,
        anomaly: anomalyB,
      },
    ],
  };
  it('returns two traces with one anomaly each', () => {
    const anomalyDataMap = getAnomalyDataMap(
      dataframe.traceset,
      dataframe.header!,
      anomalymap
    );
    assert.deepEqual(anomalyDataMap, expectedAnomalyDataMap);
  });
});
