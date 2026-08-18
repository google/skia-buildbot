import { assert } from 'chai';
import { MISSING_DATA_SENTINEL } from '../const/const';
import {
  CommitNumber,
  DataFrame,
  ReadOnlyParamSet,
  TimestampSeconds,
  Trace,
  TraceSet,
} from '../json';
import { dataframeToCSV, removeSpecialTraces, traceSeriesToCSV, TraceSeriesLike } from './index';

const df: DataFrame = {
  header: [
    {
      offset: CommitNumber(0),
      timestamp: TimestampSeconds(1660000000),
      author: '',
      hash: 'hash_1',
      message: '',
      url: '',
    },
    {
      offset: CommitNumber(1),
      timestamp: TimestampSeconds(1660000100),
      author: '',
      hash: 'hash_2',
      message: '',
      url: '',
    },
  ],
  paramset: ReadOnlyParamSet({}),
  traceset: TraceSet({
    ',arch=x86,config=8888,': Trace([90131983.0, 90132647.0]),
    ',arch=arm,config=8888,': Trace([90135000.0, MISSING_DATA_SENTINEL]),
  }),
  skip: 0,
  traceMetadata: [],
};

describe('csv', () => {
  describe('removeSpecialTraces', () => {
    it('removes special_ prefixed traces from DataFrame', () => {
      const inputDf: DataFrame = {
        ...df,
        traceset: TraceSet({
          special_zero: Trace([0, 0]),
          special_centroid: Trace([1, 2]),
          ',arch=x86,config=8888,': Trace([90131983.0, 90132647.0]),
        }),
      };

      const result = removeSpecialTraces(inputDf);
      assert.deepEqual(Object.keys(result.traceset), [',arch=x86,config=8888,']);
      assert.deepEqual(result.traceset[',arch=x86,config=8888,'], [90131983.0, 90132647.0]);
    });

    it('returns empty traceset when DataFrame only contains special_ traces', () => {
      const onlySpecialDf: DataFrame = {
        ...df,
        traceset: TraceSet({
          special_zero: Trace([0, 0]),
          special_centroid: Trace([1, 2]),
        }),
      };

      const result = removeSpecialTraces(onlySpecialDf);
      assert.deepEqual(Object.keys(result.traceset), []);
    });

    it('removes special_ prefixed traces from TraceSeriesLike[]', () => {
      const series: TraceSeriesLike[] = [
        {
          id: 'special_zero',
          rows: [{ commit_number: 100, val: 0 }],
        },
        {
          id: 'special_centroid',
          rows: [{ commit_number: 100, val: 5 }],
        },
        {
          id: ',arch=x86,config=8888,',
          rows: [{ commit_number: 100, val: 12.5 }],
        },
      ];

      const result = removeSpecialTraces(series);
      assert.equal(result.length, 1);
      assert.equal(result[0].id, ',arch=x86,config=8888,');
    });

    it('returns empty array when TraceSeriesLike[] only contains special_ traces', () => {
      const result = removeSpecialTraces([
        { id: 'special_zero', rows: [{ commit_number: 100, val: 0 }] },
      ]);
      assert.deepEqual(result, []);
    });

    it('handles null/undefined gracefully', () => {
      assert.isNull(removeSpecialTraces(null as any));
      assert.isUndefined(removeSpecialTraces(undefined as any));
    });
  });

  describe('dataframeToCSV', () => {
    it('exports a single trace', () => {
      const singleTraceDf: DataFrame = {
        ...df,
        traceset: TraceSet({
          ',arch=x86,config=8888,': Trace([90131983.0, 90132647.0]),
        }),
      };

      const expected = `offset,hash,arch=x86&config=8888
0,hash_1,90131983
1,hash_2,90132647`;
      assert.equal(dataframeToCSV(singleTraceDf), expected);
    });

    it('exports multiple traces and empty fields', () => {
      const expected = `offset,hash,arch=x86&config=8888,arch=arm&config=8888
0,hash_1,90131983,90135000
1,hash_2,90132647,`;
      assert.equal(dataframeToCSV(df), expected);
    });

    it('exports traces after caller removes special_ traces', () => {
      const dfWithSpecial: DataFrame = {
        ...df,
        traceset: TraceSet({
          special_zero: Trace([0, 0]),
          special_centroid: Trace([1, 2]),
          ',arch=x86,config=8888,': Trace([90131983.0, 90132647.0]),
        }),
      };

      const cleanDf = removeSpecialTraces(dfWithSpecial);
      const expected = `offset,hash,arch=x86&config=8888
0,hash_1,90131983
1,hash_2,90132647`;
      assert.equal(dataframeToCSV(cleanDf), expected);
    });

    it('returns empty string for empty dataframe or traceset', () => {
      assert.equal(dataframeToCSV(null as any), '');
      assert.equal(dataframeToCSV({ ...df, traceset: TraceSet({}) }), '');
    });
  });

  describe('traceSeriesToCSV', () => {
    const series: TraceSeriesLike[] = [
      {
        id: ',arch=x86,config=8888,',
        rows: [
          { commit_number: 100, hash: 'h100', val: 12.5, createdat: 1000 },
          { commit_number: 101, hash: 'h101', val: 15.0, createdat: 2000 },
        ],
      },
      {
        id: ',arch=arm,config=8888,',
        rows: [
          { commit_number: 100, hash: 'h100', val: 20.0, createdat: 1000 },
          { commit_number: 102, hash: 'h102', val: 25.0, createdat: 3000 },
        ],
      },
      {
        id: ',arch=hidden,config=8888,',
        hidden: true,
        rows: [{ commit_number: 100, val: 99, createdat: 1000 }],
      },
    ];

    it('exports active series aligned by commit numbers', () => {
      const expected = `offset,hash,arch=x86&config=8888,arch=arm&config=8888
100,h100,12.5,20
101,h101,15,
102,h102,,25`;
      assert.equal(traceSeriesToCSV(series), expected);
    });

    it('exports series after caller removes special_ series', () => {
      const seriesWithSpecial: TraceSeriesLike[] = [
        ...series,
        {
          id: 'special_zero',
          rows: [{ commit_number: 100, val: 0 }],
        },
      ];

      const cleanSeries = removeSpecialTraces(seriesWithSpecial);
      const expected = `offset,hash,arch=x86&config=8888,arch=arm&config=8888
100,h100,12.5,20
101,h101,15,
102,h102,,25`;
      assert.equal(traceSeriesToCSV(cleanSeries), expected);
    });

    it('returns empty string for null, empty or all-hidden series', () => {
      assert.equal(traceSeriesToCSV([]), '');
      assert.equal(traceSeriesToCSV(null as any), '');
      assert.equal(traceSeriesToCSV([{ id: 'test', rows: [], hidden: true }]), '');
    });
  });
});
