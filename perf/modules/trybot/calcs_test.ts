import { assert } from 'chai';
import { byParams } from './calcs';

describe('trybot', () => {
  describe('byParams', () => {
    it('returns empty list on empty input', () => {
      assert.deepEqual([], byParams({
        header: [],
        results: [],
        paramset: {},
      }));
    });

    it('returns returns correct average for two traces', () => {
      const res = byParams({
        header: [],
        results: [
          {
            params: {
              model: 'GCE',
              test: '1',
            },
            median: 0, // median, lower, upper, and values are ignored by byParams.
            lower: 0,
            upper: 0,
            stddevRatio: 2.0,
            values: [],
          },
          {
            params: {
              model: 'Nexus5x',
              test: '1',
            },
            median: 0, // median, lower, upper, and values are ignored by byParams.
            lower: 0,
            upper: 0,
            stddevRatio: 1.0,
            values: [],
          },
        ],
        paramset: {
          model: ['GCE', 'Nexus5x'],
          test: ['1'],
        },
      });

      // We expect that along the test=1 axes to average the two stddevRatio values.
      assert.deepEqual(res, [{
        keyValue: 'model=GCE',
        aveStdDevRatio: 2.0,
        n: 1,
        high: 1,
        low: 0,
      }, {
        keyValue: 'test=1',
        aveStdDevRatio: 1.5,
        n: 2,
        high: 2,
        low: 0,
      }, {
        keyValue: 'model=Nexus5x',
        aveStdDevRatio: 1.0,
        n: 1,
        high: 1,
        low: 0,
      }]);
    });

    it('sorts the results by descending aveStdDevRatio', () => {
      const res = byParams({
        header: [],
        results: [
          {
            params: {
              test: '2',
            },
            median: 0, // median, lower, upper, and values are ignored by byParams.
            lower: 0,
            upper: 0,
            stddevRatio: 2.0,
            values: [],
          },
          {
            params: {
              test: '1',
            },
            median: 0, // median, lower, upper, and values are ignored by byParams.
            lower: 0,
            upper: 0,
            stddevRatio: 1.0,
            values: [],
          },
          {
            params: {
              test: '3',
            },
            median: 0, // median, lower, upper, and values are ignored by byParams.
            lower: 0,
            upper: 0,
            stddevRatio: 3.0,
            values: [],
          },

        ],
        paramset: {
          test: ['1', '2', '3'],
        },
      });

      assert.deepEqual(
        res.map((r) => r.aveStdDevRatio),
        [3.0, 2.0, 1.0],
      );

      assert.deepEqual(
        res.map((r) => r.keyValue),
        ['test=3', 'test=2', 'test=1'],
      );
    });
  });
});
