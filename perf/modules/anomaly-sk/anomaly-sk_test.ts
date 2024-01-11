import { assert } from 'chai';
import {
  Anomaly,
  AnomalyMap,
  ColumnHeader,
  CommitNumber,
  DataFrame,
  ReadOnlyParamSet,
  TimestampSeconds,
  Trace,
  TraceSet,
} from '../json';
import { AnomalyData } from '../plot-simple-sk/plot-simple-sk';
import { getAnomalyDataMap } from './anomaly-sk';

const dummyAnomaly = (): Anomaly => ({
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
});

describe('getAnomalyDataMap', () => {
  const header: ColumnHeader[] = [
    {
      offset: CommitNumber(99),
      timestamp: TimestampSeconds(0),
    },
    {
      offset: CommitNumber(100),
      timestamp: TimestampSeconds(0),
    },
    {
      offset: CommitNumber(101),
      timestamp: TimestampSeconds(0),
    },
  ];
  const traceset = TraceSet({
    traceA: Trace([5, 5, 15]),
    traceB: Trace([1, 1, 4]),
  });
  const dataframe: DataFrame = {
    traceset: traceset,
    header: header,
    skip: 0,
    paramset: ReadOnlyParamSet({}),
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
  it('maps anomaly to the next commit if exact match not available', () => {
    const columnHeader: ColumnHeader = {
      offset: CommitNumber(103),
      timestamp: TimestampSeconds(0),
    };
    dataframe.header?.push(columnHeader);
    dataframe.traceset.traceA.push(200);
    // Add anomaly that does not have a commit in the header.
    const anomalymap = { traceA: { 102: anomalyA } };
    const dataMap = getAnomalyDataMap(
      dataframe.traceset,
      dataframe.header!,
      anomalymap
    );
    const expectedAnomalyMap: { [key: string]: AnomalyData[] } = {
      traceA: [
        {
          x: 3,
          y: 200,
          anomaly: anomalyA,
        },
      ],
    };
    assert.deepEqual(dataMap, expectedAnomalyMap);
  });
});
