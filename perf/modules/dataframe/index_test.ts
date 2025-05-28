/**
 * TODO(seanmccullough): refactor these tests and the tests for //perf/go/dataframe/
 * so they share the same json data for test case input and exepected output.
 */
import { assert } from 'chai';
import {
  buildParamSet,
  mergeColumnHeaders,
  join,
  timestampBounds,
  findSubDataframe,
  mergeAnomaly,
  findAnomalyInRange,
  range,
  generateSubDataframe,
} from './index';
import {
  DataFrame,
  ColumnHeader,
  TraceSet,
  ReadOnlyParamSet,
  CommitNumber,
  Trace,
  TimestampSeconds,
} from '../json';
import { MISSING_DATA_SENTINEL } from '../const/const';
import { generateAnomalyMap, generateFullDataFrame } from './test_utils';

const e = MISSING_DATA_SENTINEL;

describe('generate subrange from dataframe', () => {
  it('slice range', () => {
    const df = generateFullDataFrame(range(0, 10), 1, 3, [2]);
    const sub = generateSubDataframe(df, range(0, 3));
    assert.deepEqual(sub.header, df.header?.slice(0, 3));
    assert.deepEqual(sub.traceset[',key=1'], df.traceset[',key=1'].slice(0, 3));
    assert.deepEqual(sub.traceset[',key=2'], df.traceset[',key=2'].slice(0, 3));
    assert.deepEqual(sub.traceset[',key=0'], df.traceset[',key=0'].slice(0, 3));
  });
});

describe('find subrange from dataframe header', () => {
  it('find subrange inside the header', () => {
    const header = generateFullDataFrame(range(0, 10), 1, 1, [2]).header!;
    assert.deepEqual(findSubDataframe(header, range(1, 2), 'offset'), range(1, 3));
    assert.deepEqual(findSubDataframe(header, range(1, 6), 'timestamp'), range(0, 3));
    assert.deepEqual(findSubDataframe(header, range(3, 7)), range(1, 4));
  });

  it('find subrange outside the header', () => {
    const header = generateFullDataFrame(range(0, 10), 1, 1, [2]).header!;
    assert.deepEqual(findSubDataframe(header, range(-1, 2), 'offset'), range(0, 3));
    assert.deepEqual(findSubDataframe(header, range(9, 11), 'offset'), range(9, 10));
    assert.deepEqual(findSubDataframe(header, range(-1, 6)), range(0, 3));
    assert.deepEqual(findSubDataframe(header, range(19, 22)), range(9, 10));
  });

  it('find subrange not in the header', () => {
    const header = generateFullDataFrame(range(0, 10), 1, 1, [2]).header!;
    assert.deepEqual(findSubDataframe(header, range(-10, -1), 'offset'), range(0, 0));
    assert.deepEqual(findSubDataframe(header, range(100, 101), 'offset'), range(10, 10));
    assert.deepEqual(findSubDataframe(header, range(-10, -1)), range(0, 0));
    assert.deepEqual(findSubDataframe(header, range(100, 120)), range(10, 10));
  });
});

describe('merge anomaly', () => {
  const df = generateFullDataFrame({ begin: 0, end: 20 }, 1, 5, [1, 3, 6, 4, 2]);
  const anomaly = generateAnomalyMap(df, [
    { trace: 1, commit: 4, bugId: 4001 },
    { trace: 2, commit: 4, bugId: 4002 },
    { trace: 2, commit: 7, bugId: 7002 },
  ]);
  const updated = generateAnomalyMap(df, [
    { trace: 1, commit: 4, bugId: 4101 },
    { trace: 2, commit: 7, bugId: 7102 },
  ]);

  it('merge empty (always return non-null)', () => {
    assert.isNotNull(mergeAnomaly(null));
    assert.isEmpty(mergeAnomaly(null));
    assert.isNotNull(mergeAnomaly(null, {}, null));
  });

  it('merge empty w/ non-empty', () => {
    const anomaly1 = mergeAnomaly(null, findAnomalyInRange(anomaly, { begin: 5, end: 10 }));
    assert.isUndefined(anomaly1[',key=1']);
    assert.equal(anomaly1[',key=2']![7].bug_id, 7002);
  });

  it('merge non-empty w/ non-empty', () => {
    const anomaly1 = findAnomalyInRange(anomaly, { begin: 0, end: 5 })!;
    assert.equal(anomaly1[',key=1']![4].bug_id, 4001);
    assert.equal(anomaly1[',key=2']![4].bug_id, 4002);
    assert.isUndefined(anomaly1[',key=2']![7]);

    const anomaly2 = mergeAnomaly(anomaly1, findAnomalyInRange(anomaly, { begin: 5, end: 10 }));
    assert.equal(anomaly2[',key=1']![4].bug_id, 4001);
    assert.equal(anomaly2[',key=2']![7].bug_id, 7002);
  });

  it('merge w/ updated entries', () => {
    const anomaly1 = findAnomalyInRange(anomaly, { begin: 0, end: 5 })!;
    assert.equal(anomaly1[',key=1']![4].bug_id, 4001);

    const anomaly2 = mergeAnomaly(anomaly1, findAnomalyInRange(updated, { begin: 0, end: 10 })!);
    assert.equal(anomaly2[',key=1']![4].bug_id, 4101);
    assert.equal(anomaly2[',key=2']![7].bug_id, 7102);
  });

  it('merge w/ new traces', () => {
    const anomaly1 = findAnomalyInRange(anomaly, { begin: 5, end: 10 })!;
    assert.equal(anomaly1[',key=2']![7].bug_id, 7002);
    assert.isUndefined(anomaly1[',key=1']);

    const anomaly2 = mergeAnomaly(anomaly1, findAnomalyInRange(anomaly, { begin: 0, end: 5 })!);
    assert.equal(anomaly2[',key=1']![4].bug_id, 4001);
    assert.equal(anomaly2[',key=2']![4].bug_id, 4002);
  });

  it('merge w/ new and updated traces', () => {
    const anomaly1 = findAnomalyInRange(anomaly, { begin: 5, end: 10 })!;
    assert.equal(anomaly1[',key=2']![7].bug_id, 7002);
    assert.isUndefined(anomaly1[',key=1']);

    const anomaly2 = mergeAnomaly(anomaly1, findAnomalyInRange(updated, { begin: 0, end: 10 })!);
    assert.equal(anomaly2[',key=1']![4].bug_id, 4101);
    assert.equal(anomaly2[',key=2']![7].bug_id, 7102);
  });
});

describe('mergeColumnHeaders', () => {
  it('merges simple case', () => {
    const a: ColumnHeader[] = [
      {
        offset: CommitNumber(1),
        timestamp: TimestampSeconds(0),
        author: '',
        hash: '',
        message: '',
        url: '',
      },
      {
        offset: CommitNumber(2),
        timestamp: TimestampSeconds(0),
        author: '',
        hash: '',
        message: '',
        url: '',
      },
      {
        offset: CommitNumber(4),
        timestamp: TimestampSeconds(0),
        author: '',
        hash: '',
        message: '',
        url: '',
      },
    ];
    const b: ColumnHeader[] = [
      {
        offset: CommitNumber(3),
        timestamp: TimestampSeconds(0),
        author: '',
        hash: '',
        message: '',
        url: '',
      },
      {
        offset: CommitNumber(4),
        timestamp: TimestampSeconds(0),
        author: '',
        hash: '',
        message: '',
        url: '',
      },
    ];
    const [m, aMap, bMap] = mergeColumnHeaders(a, b);
    const expected: ColumnHeader[] = [
      {
        offset: CommitNumber(1),
        timestamp: TimestampSeconds(0),
        author: '',
        hash: '',
        message: '',
        url: '',
      },
      {
        offset: CommitNumber(2),
        timestamp: TimestampSeconds(0),
        author: '',
        hash: '',
        message: '',
        url: '',
      },
      {
        offset: CommitNumber(3),
        timestamp: TimestampSeconds(0),
        author: '',
        hash: '',
        message: '',
        url: '',
      },
      {
        offset: CommitNumber(4),
        timestamp: TimestampSeconds(0),
        author: '',
        hash: '',
        message: '',
        url: '',
      },
    ];
    assert.deepEqual(m, expected);
    assert.deepEqual(aMap, { 0: 0, 1: 1, 2: 3 });
    assert.deepEqual(bMap, { 0: 2, 1: 3 });
  });

  it('merges skips', () => {
    const a: ColumnHeader[] = [
      {
        offset: CommitNumber(1),
        timestamp: TimestampSeconds(0),
        author: '',
        hash: '',
        message: '',
        url: '',
      },
      {
        offset: CommitNumber(2),
        timestamp: TimestampSeconds(0),
        author: '',
        hash: '',
        message: '',
        url: '',
      },
      {
        offset: CommitNumber(4),
        timestamp: TimestampSeconds(0),
        author: '',
        hash: '',
        message: '',
        url: '',
      },
    ];
    const b: ColumnHeader[] = [
      {
        offset: CommitNumber(5),
        timestamp: TimestampSeconds(0),
        author: '',
        hash: '',
        message: '',
        url: '',
      },
      {
        offset: CommitNumber(7),
        timestamp: TimestampSeconds(0),
        author: '',
        hash: '',
        message: '',
        url: '',
      },
    ];
    const [m, aMap, bMap] = mergeColumnHeaders(a, b);
    const expected: ColumnHeader[] = [
      {
        offset: CommitNumber(1),
        timestamp: TimestampSeconds(0),
        author: '',
        hash: '',
        message: '',
        url: '',
      },
      {
        offset: CommitNumber(2),
        timestamp: TimestampSeconds(0),
        author: '',
        hash: '',
        message: '',
        url: '',
      },
      {
        offset: CommitNumber(4),
        timestamp: TimestampSeconds(0),
        author: '',
        hash: '',
        message: '',
        url: '',
      },
      {
        offset: CommitNumber(5),
        timestamp: TimestampSeconds(0),
        author: '',
        hash: '',
        message: '',
        url: '',
      },
      {
        offset: CommitNumber(7),
        timestamp: TimestampSeconds(0),
        author: '',
        hash: '',
        message: '',
        url: '',
      },
    ];
    assert.deepEqual(m, expected);
    assert.deepEqual(aMap, { 0: 0, 1: 1, 2: 2 });
    assert.deepEqual(bMap, { 0: 3, 1: 4 });
  });

  it('merges empty b', () => {
    const a: ColumnHeader[] = [];
    const b: ColumnHeader[] = [
      {
        offset: CommitNumber(1),
        timestamp: TimestampSeconds(0),
        author: '',
        hash: '',
        message: '',
        url: '',
      },
      {
        offset: CommitNumber(2),
        timestamp: TimestampSeconds(0),
        author: '',
        hash: '',
        message: '',
        url: '',
      },
      {
        offset: CommitNumber(4),
        timestamp: TimestampSeconds(0),
        author: '',
        hash: '',
        message: '',
        url: '',
      },
    ];
    const [m, aMap, bMap] = mergeColumnHeaders(a, b);
    const expected: ColumnHeader[] = [
      {
        offset: CommitNumber(1),
        timestamp: TimestampSeconds(0),
        author: '',
        hash: '',
        message: '',
        url: '',
      },
      {
        offset: CommitNumber(2),
        timestamp: TimestampSeconds(0),
        author: '',
        hash: '',
        message: '',
        url: '',
      },
      {
        offset: CommitNumber(4),
        timestamp: TimestampSeconds(0),
        author: '',
        hash: '',
        message: '',
        url: '',
      },
    ];
    assert.deepEqual(m, expected);
    assert.deepEqual(aMap, {});
    assert.deepEqual(bMap, { 0: 0, 1: 1, 2: 2 });
  });

  it('merges empty a', () => {
    const a: ColumnHeader[] = [
      {
        offset: CommitNumber(1),
        timestamp: TimestampSeconds(0),
        author: '',
        hash: '',
        message: '',
        url: '',
      },
      {
        offset: CommitNumber(2),
        timestamp: TimestampSeconds(0),
        author: '',
        hash: '',
        message: '',
        url: '',
      },
      {
        offset: CommitNumber(4),
        timestamp: TimestampSeconds(0),
        author: '',
        hash: '',
        message: '',
        url: '',
      },
    ];
    const b: ColumnHeader[] = [];
    const [m, aMap, bMap] = mergeColumnHeaders(a, b);
    const expected: ColumnHeader[] = [
      {
        offset: CommitNumber(1),
        timestamp: TimestampSeconds(0),
        author: '',
        hash: '',
        message: '',
        url: '',
      },
      {
        offset: CommitNumber(2),
        timestamp: TimestampSeconds(0),
        author: '',
        hash: '',
        message: '',
        url: '',
      },
      {
        offset: CommitNumber(4),
        timestamp: TimestampSeconds(0),
        author: '',
        hash: '',
        message: '',
        url: '',
      },
    ];
    assert.deepEqual(m, expected);
    assert.deepEqual(aMap, { 0: 0, 1: 1, 2: 2 });
    assert.deepEqual(bMap, {});
  });

  it('merges empty a and b', () => {
    const a: ColumnHeader[] = [];
    const b: ColumnHeader[] = [];
    const [m, aMap, bMap] = mergeColumnHeaders(a, b);
    const expected: ColumnHeader[] = [];
    assert.deepEqual(m, expected);
    assert.deepEqual(aMap, {});
    assert.deepEqual(bMap, {});
  });
});

describe('buildParamSet', () => {
  it('builds a paramset for an empty DataFrame', () => {
    const df: DataFrame = {
      traceset: TraceSet({}),
      header: [],
      paramset: ReadOnlyParamSet({}),
      skip: 0,
      traceMetadata: [],
    };
    buildParamSet(df);
    assert.equal(0, Object.keys(df.paramset).length);
  });

  it('builds a paramset for a non-empty DataFrame', () => {
    const df: DataFrame = {
      traceset: TraceSet({
        ',arch=x86,config=565,': Trace([1.2, 2.1]),
        ',arch=x86,config=8888,': Trace([1.3, 3.1]),
        ',arch=x86,config=gpu,': Trace([1.4, 4.1]),
      }),
      header: [],
      paramset: ReadOnlyParamSet({}),
      skip: 0,
      traceMetadata: [],
    };
    buildParamSet(df);
    assert.equal(2, Object.keys(df.paramset).length);
    assert.deepEqual(df.paramset.arch, ['x86']);
    assert.deepEqual(df.paramset.config, ['565', '8888', 'gpu']);
  });
});

describe('join', () => {
  it('joins two empty dataframes', () => {
    const a: DataFrame = {
      traceset: TraceSet({}),
      header: [],
      paramset: ReadOnlyParamSet({}),
      skip: 0,
      traceMetadata: [],
    };
    const b: DataFrame = {
      traceset: TraceSet({}),
      header: [],
      paramset: ReadOnlyParamSet({}),
      skip: 0,
      traceMetadata: [],
    };
    const got = join(a, b);

    assert.deepEqual(got, b);
  });

  it('joins two non-empty dataframes', () => {
    const a: DataFrame = {
      header: [
        {
          offset: CommitNumber(1),
          timestamp: TimestampSeconds(0),
          author: '',
          hash: '',
          message: '',
          url: '',
        },
        {
          offset: CommitNumber(2),
          timestamp: TimestampSeconds(0),
          author: '',
          hash: '',
          message: '',
          url: '',
        },
        {
          offset: CommitNumber(4),
          timestamp: TimestampSeconds(0),
          author: '',
          hash: '',
          message: '',
          url: '',
        },
      ],
      traceset: TraceSet({
        ',config=8888,arch=x86,': Trace([0.1, 0.2, 0.4]),
        ',config=8888,arch=arm,': Trace([1.1, 1.2, 1.4]),
      }),
      paramset: ReadOnlyParamSet({}),
      skip: 0,
      traceMetadata: [],
    };
    const b: DataFrame = {
      header: [
        {
          offset: CommitNumber(3),
          timestamp: TimestampSeconds(0),
          author: '',
          hash: '',
          message: '',
          url: '',
        },
        {
          offset: CommitNumber(4),
          timestamp: TimestampSeconds(0),
          author: '',
          hash: '',
          message: '',
          url: '',
        },
      ],
      traceset: TraceSet({
        ',config=565,arch=x86,': Trace([3.3, 3.4]),
        ',config=565,arch=arm,': Trace([4.3, 4.4]),
      }),
      paramset: ReadOnlyParamSet({}),
      skip: 0,
      traceMetadata: [],
    };
    buildParamSet(a);
    buildParamSet(b);

    const got = join(a, b);

    const expected: DataFrame = {
      header: [
        {
          offset: CommitNumber(1),
          timestamp: TimestampSeconds(0),
          author: '',
          hash: '',
          message: '',
          url: '',
        },
        {
          offset: CommitNumber(2),
          timestamp: TimestampSeconds(0),
          author: '',
          hash: '',
          message: '',
          url: '',
        },
        {
          offset: CommitNumber(3),
          timestamp: TimestampSeconds(0),
          author: '',
          hash: '',
          message: '',
          url: '',
        },
        {
          offset: CommitNumber(4),
          timestamp: TimestampSeconds(0),
          author: '',
          hash: '',
          message: '',
          url: '',
        },
      ],
      traceset: TraceSet({
        ',config=8888,arch=x86,': Trace([0.1, 0.2, e, 0.4]),
        ',config=8888,arch=arm,': Trace([1.1, 1.2, e, 1.4]),
        ',config=565,arch=x86,': Trace([e, e, 3.3, 3.4]),
        ',config=565,arch=arm,': Trace([e, e, 4.3, 4.4]),
      }),
      paramset: ReadOnlyParamSet({}),
      skip: 0,
      traceMetadata: [],
    };
    buildParamSet(expected);

    assert.deepEqual(got.header, expected.header);
    assert.deepEqual(got.traceset, expected.traceset);
    assert.deepEqual(got.paramset, expected.paramset);
    assert.deepEqual(got, expected);
  });
});

describe('timestampBounds', () => {
  it('returns NaNs for null DataFrame', () => {
    const nullBounds = timestampBounds(null);
    assert.deepEqual([NaN, NaN], nullBounds);
  });

  it('returns NaNs for empty DataFrame', () => {
    const emptyDataFrame: DataFrame = {
      header: [],
      traceset: TraceSet({}),
      paramset: ReadOnlyParamSet({}),
      skip: 0,
      traceMetadata: [],
    };
    const emptyBounds = timestampBounds(emptyDataFrame);
    assert.deepEqual([NaN, NaN], emptyBounds);
  });

  it('returns bounds for single-element DataFrame', () => {
    const singleElementDataFrame: DataFrame = {
      header: [
        {
          offset: CommitNumber(1),
          timestamp: TimestampSeconds(0),
          author: '',
          hash: '',
          message: '',
          url: '',
        },
      ],
      traceset: TraceSet({}),
      paramset: ReadOnlyParamSet({}),
      skip: 0,
      traceMetadata: [],
    };
    const singleElementBounds = timestampBounds(singleElementDataFrame);
    assert.deepEqual([0, 0], singleElementBounds);
  });

  it('returns bounds for multiple-element DataFrame', () => {
    const multipleElementDataFrame: DataFrame = {
      header: [
        {
          offset: CommitNumber(1),
          timestamp: TimestampSeconds(11),
          author: '',
          hash: '',
          message: '',
          url: '',
        },
        {
          offset: CommitNumber(2),
          timestamp: TimestampSeconds(12),
          author: '',
          hash: '',
          message: '',
          url: '',
        },
        {
          offset: CommitNumber(3),
          timestamp: TimestampSeconds(13),
          author: '',
          hash: '',
          message: '',
          url: '',
        },
        {
          offset: CommitNumber(4),
          timestamp: TimestampSeconds(14),
          author: '',
          hash: '',
          message: '',
          url: '',
        },
      ],
      traceset: TraceSet({}),
      paramset: ReadOnlyParamSet({}),
      skip: 0,
      traceMetadata: [],
    };
    const multipleElementBounds = timestampBounds(multipleElementDataFrame);
    assert.deepEqual([11, 14], multipleElementBounds);
  });
});
