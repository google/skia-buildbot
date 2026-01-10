import { assert } from 'chai';
import { getTraceColor, convertFromDataframe } from './plot-builder';
import { TraceSet, ColumnHeader, Trace } from '../json';
import { MISSING_DATA_SENTINEL } from '../const/const';

describe('plot-builder', () => {
  describe('getTraceColor', () => {
    it('returns consistent color for same string', () => {
      assert.equal(getTraceColor('foo'), getTraceColor('foo'));
    });
    it('returns different colors for different strings', () => {
      assert.notEqual(getTraceColor('foo'), getTraceColor('bar'));
    });
  });

  describe('convertFromDataframe', () => {
    it('returns null for empty header', () => {
      assert.isNull(convertFromDataframe({ traceset: TraceSet({}), header: [] }));
    });

    it('converts dataframe correctly', () => {
      const traceset: TraceSet = TraceSet({
        trace1: Trace([1, 2]),
        trace2: Trace([3, MISSING_DATA_SENTINEL]),
      });
      const header: ColumnHeader[] = [
        { offset: 100, timestamp: 1000 },
        { offset: 101, timestamp: 2000 },
      ] as any;

      const result = convertFromDataframe({ traceset, header }, 'commit');
      assert.isNotNull(result);
      // Row 0: Header
      // Row 1: Data point 1
      // Row 2: Data point 2
      assert.equal(result!.length, 3);

      // Header check
      // [ {role: domain...}, 'trace1', 'trace2' ]
      assert.equal(result![0][1], 'trace1');
      assert.equal(result![0][2], 'trace2');

      // Data check
      // Row 1: [100, 1, 3]
      assert.equal(result![1][0], 100);
      assert.equal(result![1][1], 1);
      assert.equal(result![1][2], 3);

      // Row 2: [101, 2, null] (missing data sentinel -> null)
      assert.equal(result![2][0], 101);
      assert.equal(result![2][1], 2);
      assert.isNull(result![2][2]);
    });
  });
});
