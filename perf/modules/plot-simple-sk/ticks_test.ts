import { assert } from 'chai';
import { ticks } from './ticks';

describe('ticks()', () => {
  it('handles months', () => {
    const ts = [
      new Date(2014, 6, 1, 0, 0, 0, 0),
      new Date(2014, 7, 1, 0, 0, 0, 0),
      new Date(2014, 7, 2, 0, 0, 0, 0),
      new Date(2014, 9, 1, 0, 0, 0, 0),
    ];

    assert.deepEqual(ticks(ts), [
      { x: 0, text: 'Jul' },
      { x: 1, text: 'Aug' },
      { x: 3, text: 'Oct' },
    ]);
  });

  it('handles day of month', () => {
    const ts = [
      new Date(2014, 6, 1, 0, 0, 0, 0),
      new Date(2014, 6, 3, 0, 0, 0, 0),
      new Date(2014, 6, 5, 0, 0, 0, 0),
      new Date(2014, 6, 7, 0, 0, 0, 0),
      new Date(2014, 6, 9, 0, 0, 0, 0),
    ];

    assert.deepEqual(ticks(ts), [
      { x: 0, text: 'Jul 1' },
      { x: 1, text: 'Jul 3' },
      { x: 2, text: 'Jul 5' },
      { x: 3, text: 'Jul 7' },
      { x: 4, text: 'Jul 9' },
    ]);
  });

  it('handles weekdays', () => {
    const ts = [
      new Date(2014, 6, 1, 6, 0, 0, 0),
      new Date(2014, 6, 2, 5, 0, 0, 0),
      new Date(2014, 6, 3, 4, 0, 0, 0),
      new Date(2014, 6, 5, 3, 0, 0, 0),
      new Date(2014, 6, 7, 2, 0, 0, 0),
    ];

    assert.deepEqual(ticks(ts), [
      { x: 0, text: 'Tue, 06' },
      { x: 1, text: 'Wed, 05' },
      { x: 2, text: 'Thu, 04' },
      { x: 3, text: 'Sat, 03' },
      { x: 4, text: 'Mon, 02' },
    ]);
  });

  it('handles hours', () => {
    const ts = [
      new Date(2014, 6, 1, 10, 0, 0, 0),
      new Date(2014, 6, 1, 11, 0, 0, 0),
      new Date(2014, 6, 1, 12, 0, 0, 0),
      new Date(2014, 6, 1, 13, 0, 0, 0),
      new Date(2014, 6, 1, 15, 0, 0, 0),
    ];

    assert.deepEqual(ticks(ts), [
      { x: 0, text: '10' },
      { x: 1, text: '11' },
      { x: 2, text: '12' },
      { x: 3, text: '13' },
      { x: 4, text: '15' },
    ]);
  });

  it('handles minutes and decimation', () => {
    const ts = [
      new Date(2014, 6, 1, 1, 1, 0, 0),
      new Date(2014, 6, 1, 1, 2, 0, 0),
      new Date(2014, 6, 1, 1, 3, 0, 0),
      new Date(2014, 6, 1, 1, 4, 0, 0),
      new Date(2014, 6, 1, 1, 5, 0, 0),
      new Date(2014, 6, 1, 1, 6, 0, 0),
      new Date(2014, 6, 1, 1, 7, 0, 0),
      new Date(2014, 6, 1, 1, 8, 0, 0),
      new Date(2014, 6, 1, 1, 9, 0, 0),
      new Date(2014, 6, 1, 1, 10, 0, 0),
      new Date(2014, 6, 1, 1, 11, 0, 0),
      new Date(2014, 6, 1, 1, 12, 0, 0),
    ];

    // Also tests decimation.
    assert.deepEqual(ticks(ts), [
      { x: 1, text: '01:02' },
      { x: 3, text: '01:04' },
      { x: 5, text: '01:06' },
      { x: 7, text: '01:08' },
      { x: 9, text: '01:10' },
      { x: 11, text: '01:12' },
    ]);
  });

  it('handles seconds', () => {
    const ts = [
      new Date(2014, 6, 1, 1, 1, 10, 0),
      new Date(2014, 6, 1, 1, 1, 11, 0),
      new Date(2014, 6, 1, 1, 1, 12, 0),
      new Date(2014, 6, 1, 1, 1, 13, 0),
    ];

    // Also tests decimation.
    assert.deepEqual(ticks(ts), [
      { x: 0, text: '01:01:10' },
      { x: 1, text: '01:01:11' },
      { x: 2, text: '01:01:12' },
      { x: 3, text: '01:01:13' },
    ]);
  });
});
