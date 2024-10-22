import { assert } from 'chai';
import { getLegend, getTitle } from './traceset';
import { generateFullDataFrame } from './test_utils';

const now = 1726081856; // an arbitrary UNIX time;
const timeSpan = 89; // an arbitrary prime number for time span between commits .

describe('getTitle', () => {
  it('happy path', () => {
    const keys = [
      ',benchmark=JetStream2,bot=MacM1,ref_mode=head,subtest=Average,test=Total,v8_mode=pgo,',
      ',benchmark=JetStream2,bot=MacM1,ref_mode=ref,subtest=Average,test=Total,v8_mode=default,',
      ',benchmark=JetStream2,bot=MacM1,ref_mode=ref,subtest=Normal,test=Total,v8_mode=default,',
    ];
    const df = generateFullDataFrame(
      { begin: 90, end: 120 },
      now,
      keys.length,
      [timeSpan],
      [null],
      keys
    );

    const title = getTitle(df);
    assert.deepEqual(title, {
      benchmark: 'JetStream2',
      bot: 'MacM1',
      test: 'Total',
    });
  });

  it('only one key returns full title', () => {
    const keys = [
      ',benchmark=JetStream2,bot=MacM1,ref_mode=head,subtest=Average,test=Total,v8_mode=pgo,',
    ];
    const df = generateFullDataFrame(
      { begin: 90, end: 120 },
      now,
      keys.length,
      [timeSpan],
      [null],
      keys
    );

    const title = getTitle(df);
    assert.deepEqual(title, {
      benchmark: 'JetStream2',
      bot: 'MacM1',
      ref_mode: 'head',
      subtest: 'Average',
      test: 'Total',
      v8_mode: 'pgo',
    });
  });

  it('missing key returns common keys', () => {
    const keys = [
      ',benchmark=JetStream2,bot=MacM1,ref_mode=ref,',
      ',benchmark=JetStream2,ref_mode=ref,',
      ',benchmark=JetStream2,ref_mode=ref,',
    ];
    const df = generateFullDataFrame(
      { begin: 90, end: 120 },
      now,
      keys.length,
      [timeSpan],
      [null],
      keys
    );
    const title = getTitle(df);
    assert.deepEqual(title, {
      benchmark: 'JetStream2',
      ref_mode: 'ref',
    });
  });

  it('empty string returns no title', () => {
    const keys = ['', ',benchmark=JetStream2,ref_mode=ref,'];
    const df = generateFullDataFrame(
      { begin: 90, end: 120 },
      now,
      keys.length,
      [timeSpan],
      [null],
      keys
    );
    const title = getTitle(df);
    assert.deepEqual(title, {});
  });
});

describe('getLegend', () => {
  it('happy path', () => {
    const keys = [
      ',benchmark=JetStream2,bot=MacM1,ref_mode=head,subtest=Average,test=Total,v8_mode=pgo,',
      ',benchmark=JetStream2,bot=MacM1,ref_mode=ref,subtest=Average,test=Total,v8_mode=default,',
      ',benchmark=JetStream2,bot=MacM1,ref_mode=ref,subtest=Normal,test=Total,v8_mode=default,',
    ];
    const df = generateFullDataFrame(
      { begin: 90, end: 120 },
      now,
      keys.length,
      [timeSpan],
      [null],
      keys
    );

    const legend = getLegend(df);
    assert.equal(legend.length, keys.length);
    assert.deepEqual(legend, [
      {
        ref_mode: 'head',
        subtest: 'Average',
        v8_mode: 'pgo',
      },
      {
        ref_mode: 'ref',
        subtest: 'Average',
        v8_mode: 'default',
      },
      {
        ref_mode: 'ref',
        subtest: 'Normal',
        v8_mode: 'default',
      },
    ]);
  });

  it('only one key returns empty legend', () => {
    const keys = [
      ',benchmark=JetStream2,bot=MacM1,ref_mode=head,subtest=Average,test=Total,v8_mode=pgo,',
    ];
    const df = generateFullDataFrame(
      { begin: 90, end: 120 },
      now,
      keys.length,
      [timeSpan],
      [null],
      keys
    );

    const legend = getLegend(df);
    assert.equal(legend.length, keys.length);
    assert.deepEqual(legend, [{}]);
  });

  it('missing key returns unique keys', () => {
    const keys = [
      ',benchmark=JetStream2,bot=MacM1,ref_mode=ref,',
      ',benchmark=JetStream2,ref_mode=ref,',
      ',benchmark=JetStream2,ref_mode=avg,',
    ];
    const df = generateFullDataFrame(
      { begin: 90, end: 120 },
      now,
      keys.length,
      [timeSpan],
      [null],
      keys
    );

    const legend = getLegend(df);
    assert.equal(legend.length, keys.length);
    assert.deepEqual(legend, [
      {
        bot: 'MacM1',
        ref_mode: 'ref',
      },
      {
        ref_mode: 'ref',
      },
      {
        ref_mode: 'avg',
      },
    ]);
  });

  it('empty string causes legend to return everything', () => {
    const keys = ['', ',benchmark=JetStream2,ref_mode=ref,', ',benchmark=JetStream2,ref_mode=avg,'];
    const df = generateFullDataFrame(
      { begin: 90, end: 120 },
      now,
      keys.length,
      [timeSpan],
      [null],
      keys
    );

    const legend = getLegend(df);
    assert.equal(legend.length, keys.length);
    assert.deepEqual(legend, [
      {},
      {
        benchmark: 'JetStream2',
        ref_mode: 'ref',
      },
      {
        benchmark: 'JetStream2',
        ref_mode: 'avg',
      },
    ]);
  });
});
