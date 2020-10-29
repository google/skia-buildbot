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

    it('returns returns correct values for two traces', () => {
      const res = byParams({
        header: [],
        results: [
          {
            params: {
              model: 'GCE',
              test: 'AndroidCodec_01_original.jpg_SampleSize2_640_480',
            },
            median: 20.16768,
            lower: 0.08480634,
            upper: 1.0624574,
            stddevRatio: -0.7101869,
            values: [
              21.441837,
              21.57052,
              20.130117,
              20.099943,
              21.651741,
              20.16893,
              20.223429,
              20.195557,
              20.137909,
              20.258265,
              21.580639,
              20.166431,
              20.109184,
              20.038948,
              20.077394,
              20.1049,
              20.267982,
              21.69963,
              20.093105,
              20.025578,
              20.107452,
            ],
          },
          {
            params: {
              model: 'Nexus5x',
              test: 'AndroidCodec_01_original.jpg_SampleSize2_640_480',
            },
            median: 20.209984,
            lower: 0.12529692,
            upper: 1.3107316,
            stddevRatio: -1.1634808,
            values: [
              19.947046,
              20.16007,
              21.49804,
              20.165445,
              20.090742,
              20.091187,
              21.74085,
              20.189804,
              20.19587,
              20.183008,
              20.2241,
              21.7264,
              21.715054,
              20.28873,
              20.0661,
              20.0784,
              21.831709,
              21.597977,
              20.227877,
              21.736917,
              20.064203,
            ],
          },
        ],
        paramset: {
          model: ['GCE', 'Nexus5x'],
          test: ['AndroidCodec_01_original.jpg_SampleSize2_640_480'],
        },
      });

      assert.deepEqual(
        res.map((r) => r.aveStdDevRatio),
        [-0.7101869, -0.93683385, -1.1634808],
      );

      assert.deepEqual(
        res.map((r) => r.keyValue),
        ['model=GCE', 'test=AndroidCodec_01_original.jpg_SampleSize2_640_480', 'model=Nexus5x'],
      );
    });
  });
});
