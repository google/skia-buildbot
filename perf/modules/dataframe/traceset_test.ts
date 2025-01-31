import { assert } from 'chai';
import { findTraceByLabel, getLegend, getLegendKeysTitle, getTitle } from './traceset';
import { generateFullDataFrame } from './test_utils';
import { convertFromDataframe } from '../common/plot-builder';
import { load } from '@google-web-components/google-chart/loader';
import { PlotGoogleChartSk } from '../plot-google-chart-sk/plot-google-chart-sk';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';

const now = 1726081856; // an arbitrary UNIX time;
const timeSpan = 89; // an arbitrary prime number for time span between commits .

describe('getTitle', () => {
  it('happy path', async () => {
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
    // Load Google Chart API for DataTable.
    setUpElementUnderTest<PlotGoogleChartSk>('plot-google-chart-sk');
    await load();
    const dt = google.visualization.arrayToDataTable(convertFromDataframe(df, 'both')!);

    const title = getTitle(dt);
    assert.deepEqual(title, {
      benchmark: 'JetStream2',
      bot: 'MacM1',
      test: 'Total',
    });
  });

  it('only one key returns full title', async () => {
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
    // Load Google Chart API for DataTable.
    setUpElementUnderTest<PlotGoogleChartSk>('plot-google-chart-sk');
    await load();
    const dt = google.visualization.arrayToDataTable(convertFromDataframe(df, 'both')!);

    const title = getTitle(dt);
    assert.deepEqual(title, {
      benchmark: 'JetStream2',
      bot: 'MacM1',
      ref_mode: 'head',
      subtest: 'Average',
      test: 'Total',
      v8_mode: 'pgo',
    });
  });

  it('missing key returns common keys', async () => {
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
    // Load Google Chart API for DataTable.
    setUpElementUnderTest<PlotGoogleChartSk>('plot-google-chart-sk');
    await load();
    const dt = google.visualization.arrayToDataTable(convertFromDataframe(df, 'both')!);

    const title = getTitle(dt);
    assert.deepEqual(title, {
      benchmark: 'JetStream2',
      ref_mode: 'ref',
    });
  });

  it('function keys return common keys', async () => {
    const keys = [
      'norm(,benchmark=JetStream2,bot=M1,ref_mode=head,subtest=Average,test=Total,v8_mode=pgo,)',
      'norm(,benchmark=JetStream2,bot=M1,ref_mode=ref,subtest=Average,test=Total,v8_mode=default,)',
      'norm(,benchmark=JetStream2,bot=M1,ref_mode=ref,subtest=Normal,test=Total,v8_mode=default,)',
    ];
    const df = generateFullDataFrame(
      { begin: 90, end: 120 },
      now,
      keys.length,
      [timeSpan],
      [null],
      keys
    );
    // Load Google Chart API for DataTable.
    setUpElementUnderTest<PlotGoogleChartSk>('plot-google-chart-sk');
    await load();
    const dt = google.visualization.arrayToDataTable(convertFromDataframe(df, 'both')!);

    const title = getTitle(dt);
    assert.deepEqual(title, {
      benchmark: 'JetStream2',
      bot: 'M1',
      test: 'Total',
    });
  });

  it('empty string returns no title', async () => {
    const keys = ['', ',benchmark=JetStream2,ref_mode=ref,'];
    const df = generateFullDataFrame(
      { begin: 90, end: 120 },
      now,
      keys.length,
      [timeSpan],
      [null],
      keys
    );
    // Load Google Chart API for DataTable.
    setUpElementUnderTest<PlotGoogleChartSk>('plot-google-chart-sk');
    await load();
    const dt = google.visualization.arrayToDataTable(convertFromDataframe(df, 'both')!);

    const title = getTitle(dt);
    assert.deepEqual(title, {});
  });
});

describe('getLegend', () => {
  it('happy path', async () => {
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
    // Load Google Chart API for DataTable.
    setUpElementUnderTest<PlotGoogleChartSk>('plot-google-chart-sk');
    await load();
    const dt = google.visualization.arrayToDataTable(convertFromDataframe(df, 'both')!);

    const legend = getLegend(dt);
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

  it('function keys return legend with function text removed', async () => {
    const keys = [
      'norm(,benchmark=JetStream2,bot=M1,ref_mode=head,subtest=Average,test=Total,v8_mode=pgo,)',
      'norm(,benchmark=JetStream2,bot=M1,ref_mode=ref,subtest=Average,test=Total,v8_mode=default,)',
      'norm(,benchmark=JetStream2,bot=M1,ref_mode=ref,subtest=Normal,test=Total,v8_mode=default,)',
    ];
    const df = generateFullDataFrame(
      { begin: 90, end: 120 },
      now,
      keys.length,
      [timeSpan],
      [null],
      keys
    );
    // Load Google Chart API for DataTable.
    setUpElementUnderTest<PlotGoogleChartSk>('plot-google-chart-sk');
    await load();
    const dt = google.visualization.arrayToDataTable(convertFromDataframe(df, 'both')!);

    const legend = getLegend(dt);
    console.log(legend);
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

  it('only one key returns empty legend', async () => {
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
    // Load Google Chart API for DataTable.
    setUpElementUnderTest<PlotGoogleChartSk>('plot-google-chart-sk');
    await load();
    const dt = google.visualization.arrayToDataTable(convertFromDataframe(df, 'both')!);

    const legend = getLegend(dt);
    assert.equal(legend.length, keys.length);
    assert.deepEqual(legend, [{}]);
  });

  it('missing keys return none', async () => {
    const keys = [
      ',benchmark=JetStream2,bot=MacM1,ref_mode=ref,',
      ',benchmark=JetStream2,ref_mode=ref,',
      ',benchmark=JetStream2,ref_mode=avg,subtest=jetstream2',
    ];
    const df = generateFullDataFrame(
      { begin: 90, end: 120 },
      now,
      keys.length,
      [timeSpan],
      [null],
      keys
    );
    // Load Google Chart API for DataTable.
    setUpElementUnderTest<PlotGoogleChartSk>('plot-google-chart-sk');
    await load();
    const dt = google.visualization.arrayToDataTable(convertFromDataframe(df, 'both')!);

    const legend = getLegend(dt);
    assert.equal(legend.length, keys.length);
    assert.deepEqual(legend, [
      {
        bot: 'MacM1',
        ref_mode: 'ref',
        subtest: '-',
      },
      {
        bot: '-',
        ref_mode: 'ref',
        subtest: '-',
      },
      {
        bot: '-',
        ref_mode: 'avg',
        subtest: 'jetstream2',
      },
    ]);
  });

  it('blank keys return none', async () => {
    const keys = [
      ',benchmark=JetStream2,bot=,ref_mode=ref,',
      ',benchmark=JetStream2,bot=MacM1,ref_mode=avg,',
      ',benchmark=JetStream2,bot=win-10-perf,ref_mode=',
    ];
    const df = generateFullDataFrame(
      { begin: 90, end: 120 },
      now,
      keys.length,
      [timeSpan],
      [null],
      keys
    );
    // Load Google Chart API for DataTable.
    setUpElementUnderTest<PlotGoogleChartSk>('plot-google-chart-sk');
    await load();
    const dt = google.visualization.arrayToDataTable(convertFromDataframe(df, 'both')!);

    const legend = getLegend(dt);
    assert.equal(legend.length, keys.length);
    assert.deepEqual(legend, [
      {
        bot: '-',
        ref_mode: 'ref',
      },
      {
        bot: 'MacM1',
        ref_mode: 'avg',
      },
      {
        bot: 'win-10-perf',
        ref_mode: '-',
      },
    ]);
  });

  it('empty string causes legend to return everything', async () => {
    const keys = ['', ',benchmark=JetStream2,ref_mode=ref,', ',benchmark=JetStream2,ref_mode=avg,'];
    const df = generateFullDataFrame(
      { begin: 90, end: 120 },
      now,
      keys.length,
      [timeSpan],
      [null],
      keys
    );
    // Load Google Chart API for DataTable.
    setUpElementUnderTest<PlotGoogleChartSk>('plot-google-chart-sk');
    await load();
    const dt = google.visualization.arrayToDataTable(convertFromDataframe(df, 'both')!);

    const legend = getLegend(dt);
    assert.equal(legend.length, keys.length);
    assert.deepEqual(legend, [
      {
        benchmark: '-',
        ref_mode: '-',
      },
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

describe('getLegendKeysTitle', () => {
  it('found and return subtest_1 value', async () => {
    const labels = [
      ',benchmark=JetStream2,bot=MacM1,ref_mode=head,subtest_1=Average,test=Total,v8_mode=pgo,',
      ',benchmark=JetStream2,bot=MacM1,ref_mode=head,subtest_1=Sum,test=Total,v8_mode=pgo,',
      ',benchmark=JetStream2,bot=MacM1,ref_mode=head,subtest_1=Min,test=Total,v8_mode=pgo,',
    ];

    const df = generateFullDataFrame(
      { begin: 90, end: 120 },
      now,
      labels.length,
      [timeSpan],
      [null],
      labels
    );
    // Load Google Chart API for DataTable.
    setUpElementUnderTest<PlotGoogleChartSk>('plot-google-chart-sk');
    await load();
    const dt = google.visualization.arrayToDataTable(convertFromDataframe(df, 'both')!);
    const legend = getLegend(dt);
    const actualValue = getLegendKeysTitle(legend[0]);

    assert.deepEqual(actualValue, 'subtest_1');
  });
});

describe('find and return matched label from Google chart', () => {
  it('found and return matched label value', async () => {
    const label = [
      ',benchmark=JetStream2,bot=MacM1,ref_mode=head,subtest_1=Average,test=Total,v8_mode=pgo,',
      ',benchmark=JetStream2,bot=MacM1,ref_mode=head,subtest_1=Sum,test=Total,v8_mode=pgo,',
      ',benchmark=JetStream2,bot=MacM1,ref_mode=head,subtest_1=Min,test=Total,v8_mode=pgo,',
    ];
    const df = generateFullDataFrame(
      { begin: 90, end: 120 },
      now,
      label.length,
      [timeSpan],
      [null],
      label
    );
    // Load Google Chart API for DataTable.
    setUpElementUnderTest<PlotGoogleChartSk>('plot-google-chart-sk');
    await load();
    const dt = google.visualization.arrayToDataTable(convertFromDataframe(df, 'both')!);

    const actualValue = findTraceByLabel(dt, label[0]);
    console.log(actualValue);
    assert.deepEqual(actualValue, label[0]);
  });

  it('return null if there is no matched label ', async () => {
    const label = [
      ',benchmark=JetStream2,bot=MacM1,ref_mode=head,subtest_1=Average,test=Total,v8_mode=pgo,',
      ',benchmark=JetStream2,bot=MacM1,ref_mode=head,subtest_1=Max,test=Total,v8_mode=pgo,',
      ',benchmark=JetStream2,bot=MacM1,ref_mode=head,subtest_1=Min,test=Total,v8_mode=pgo,',
    ];
    const df = generateFullDataFrame(
      { begin: 90, end: 120 },
      now,
      label.length,
      [timeSpan],
      [null],
      label
    );
    // Load Google Chart API for DataTable.
    setUpElementUnderTest<PlotGoogleChartSk>('plot-google-chart-sk');
    await load();
    const dt = google.visualization.arrayToDataTable(convertFromDataframe(df, 'both')!);

    const actualValue = findTraceByLabel(
      dt,
      ',benchmark=JetStream2,bot=MacM1,ref_mode=head,subtest_1=fencedframe,test=Total,v8_mode=pgo,'
    );
    console.log(actualValue);
    assert.deepEqual(actualValue, null);
  });
});
