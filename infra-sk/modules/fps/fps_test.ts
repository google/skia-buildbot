import { assert } from 'chai';
import { FPS, numMeasurements } from './fps';

const frameDuration = 1000 / 60; // ms

describe('FPS', () => {
  it('returns 0 before any calls to raf().', () => {
    const f = new FPS();
    assert.equal(f.fps, 0);
  });

  it('calculates fps correctly.', () => {
    const f = new FPS();
    const timestamps: number[] = [];
    for (let i = 0; i < numMeasurements; i++) {
      timestamps.push(i * frameDuration); // Perfectly spaced 60 FPS ms timestamps.
    }
    // eslint-disable-next-line dot-notation
    f['timestamps'] = timestamps; // Bypass the privateness of timestamps.
    f.raf();
    assert.equal(f.fps.toFixed(1), '60.0');
  });
});
