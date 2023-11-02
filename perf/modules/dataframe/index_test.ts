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
} from './index';
import {
  DataFrame,
  ParamSet,
  Params,
  ColumnHeader,
  TraceSet,
  ReadOnlyParamSet,
} from '../json';
import { MISSING_DATA_SENTINEL } from '../const/const';

const e = MISSING_DATA_SENTINEL;

describe('mergeColumnHeaders', () => {
  it('merges simple case', () => {
    const a: ColumnHeader[] = [
      { offset: 1, timestamp: 0 },
      { offset: 2, timestamp: 0 },
      { offset: 4, timestamp: 0 },
    ];
    const b: ColumnHeader[] = [
      { offset: 3, timestamp: 0 },
      { offset: 4, timestamp: 0 },
    ];
    const [m, aMap, bMap] = mergeColumnHeaders(a, b);
    const expected: ColumnHeader[] = [
      { offset: 1, timestamp: 0 },
      { offset: 2, timestamp: 0 },
      { offset: 3, timestamp: 0 },
      { offset: 4, timestamp: 0 },
    ];
    assert.deepEqual(m, expected);
    assert.deepEqual(aMap, { 0: 0, 1: 1, 2: 3 });
    assert.deepEqual(bMap, { 0: 2, 1: 3 });
  });

  it('merges skips', () => {
    const a: ColumnHeader[] = [
      { offset: 1, timestamp: 0 },
      { offset: 2, timestamp: 0 },
      { offset: 4, timestamp: 0 },
    ];
    const b: ColumnHeader[] = [
      { offset: 5, timestamp: 0 },
      { offset: 7, timestamp: 0 },
    ];
    const [m, aMap, bMap] = mergeColumnHeaders(a, b);
    const expected: ColumnHeader[] = [
      { offset: 1, timestamp: 0 },
      { offset: 2, timestamp: 0 },
      { offset: 4, timestamp: 0 },
      { offset: 5, timestamp: 0 },
      { offset: 7, timestamp: 0 },
    ];
    assert.deepEqual(m, expected);
    assert.deepEqual(aMap, { 0: 0, 1: 1, 2: 2 });
    assert.deepEqual(bMap, { 0: 3, 1: 4 });
  });

  it('merges empty b', () => {
    const a: ColumnHeader[] = [];
    const b: ColumnHeader[] = [
      { offset: 1, timestamp: 0 },
      { offset: 2, timestamp: 0 },
      { offset: 4, timestamp: 0 },
    ];
    const [m, aMap, bMap] = mergeColumnHeaders(a, b);
    const expected: ColumnHeader[] = [
      { offset: 1, timestamp: 0 },
      { offset: 2, timestamp: 0 },
      { offset: 4, timestamp: 0 },
    ];
    assert.deepEqual(m, expected);
    assert.deepEqual(aMap, {});
    assert.deepEqual(bMap, { 0: 0, 1: 1, 2: 2 });
  });

  it('merges empty a', () => {
    const a: ColumnHeader[] = [
      { offset: 1, timestamp: 0 },
      { offset: 2, timestamp: 0 },
      { offset: 4, timestamp: 0 },
    ];
    const b: ColumnHeader[] = [];
    const [m, aMap, bMap] = mergeColumnHeaders(a, b);
    const expected: ColumnHeader[] = [
      { offset: 1, timestamp: 0 },
      { offset: 2, timestamp: 0 },
      { offset: 4, timestamp: 0 },
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
      traceset: {},
      header: [],
      paramset: {},
      skip: 0,
    };
    buildParamSet(df);
    assert.equal(0, Object.keys(df.paramset).length);
  });

  it('builds a paramset for a non-empty DataFrame', () => {
    const df: DataFrame = {
      traceset: {
        ',arch=x86,config=565,': [1.2, 2.1],
        ',arch=x86,config=8888,': [1.3, 3.1],
        ',arch=x86,config=gpu,': [1.4, 4.1],
      },
      header: [],
      paramset: {},
      skip: 0,
    };
    buildParamSet(df);
    assert.equal(2, Object.keys(df.paramset).length);
    assert.deepEqual(df.paramset['arch'], ['x86']);
    assert.deepEqual(df.paramset['config'], ['565', '8888', 'gpu']);
  });
});

describe('join', () => {
  it('joins two empty dataframes', () => {
    const a: DataFrame = {
      traceset: {},
      header: [],
      paramset: {},
      skip: 0,
    };
    const b: DataFrame = {
      traceset: {},
      header: [],
      paramset: {},
      skip: 0,
    };
    const got = join(a, b);

    assert.deepEqual(got, b);
  });

  it('joins two non-empty dataframes', () => {
    const a: DataFrame = {
      header: [
        { offset: 1, timestamp: 0 },
        { offset: 2, timestamp: 0 },
        { offset: 4, timestamp: 0 },
      ],
      traceset: {
        ',config=8888,arch=x86,': [0.1, 0.2, 0.4],
        ',config=8888,arch=arm,': [1.1, 1.2, 1.4],
      },
      paramset: {},
      skip: 0,
    };
    const b: DataFrame = {
      header: [
        { offset: 3, timestamp: 0 },
        { offset: 4, timestamp: 0 },
      ],
      traceset: {
        ',config=565,arch=x86,': [3.3, 3.4],
        ',config=565,arch=arm,': [4.3, 4.4],
      },
      paramset: {},
      skip: 0,
    };
    buildParamSet(a);
    buildParamSet(b);

    const got = join(a, b);

    const expected: DataFrame = {
      header: [
        { offset: 1, timestamp: 0 },
        { offset: 2, timestamp: 0 },
        { offset: 3, timestamp: 0 },
        { offset: 4, timestamp: 0 },
      ],
      traceset: {
        ',config=8888,arch=x86,': [0.1, 0.2, e, 0.4],
        ',config=8888,arch=arm,': [1.1, 1.2, e, 1.4],
        ',config=565,arch=x86,': [e, e, 3.3, 3.4],
        ',config=565,arch=arm,': [e, e, 4.3, 4.4],
      },
      paramset: {},
      skip: 0,
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
      traceset: {},
      paramset: {},
      skip: 0,
    };
    const emptyBounds = timestampBounds(emptyDataFrame);
    assert.deepEqual([NaN, NaN], emptyBounds);
  });

  it('returns bounds for single-element DataFrame', () => {
    const singleElementDataFrame: DataFrame = {
      header: [{ offset: 1, timestamp: 0 }],
      traceset: {},
      paramset: {},
      skip: 0,
    };
    const singleElementBounds = timestampBounds(singleElementDataFrame);
    assert.deepEqual([0, 0], singleElementBounds);
  });

  it('returns bounds for multiple-element DataFrame', () => {
    const multipleElementDataFrame: DataFrame = {
      header: [
        { offset: 1, timestamp: 11 },
        { offset: 2, timestamp: 12 },
        { offset: 3, timestamp: 13 },
        { offset: 4, timestamp: 14 },
      ],
      traceset: {},
      paramset: {},
      skip: 0,
    };
    const multipleElementBounds = timestampBounds(multipleElementDataFrame);
    assert.deepEqual([11, 14], multipleElementBounds);
  });
});
