import { assert } from 'chai';

import { DataFrameRepository } from './dataframe_context';
import './dataframe_context';

import { ColumnHeader, ReadOnlyParamSet } from '../json';
import fetchMock from 'fetch-mock';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import {
  generateAnomalyMap,
  generateFullDataFrame,
  mockFrameStart,
} from './test_utils';

const now = 1726081856; // an arbitrary UNIX time;
const timeSpan = 89; // an arbitrary prime number for time span between commits .

const sorted = (a: (ColumnHeader | null)[]) => {
  return a.every((v, idx, arr) => {
    return idx === 0 || v!.offset > arr[idx - 1]!.offset;
  });
};

describe('dataframe-repository', () => {
  const newEl = setUpElementUnderTest<DataFrameRepository>(
    'dataframe-repository-sk'
  );

  const paramset = ReadOnlyParamSet({
    benchmark: ['Compile'],
    bot: ['MacM1'],
    ref_mode: ['head'],
  });

  const df = generateFullDataFrame({ begin: 90, end: 120 }, now, 3, [timeSpan]);
  const anomaly = generateAnomalyMap(df, [
    { commit: 5, bugId: 555 },
    { commit: 15, bugId: 1515 },
    { commit: 25, bugId: 2525 },
  ]);
  afterEach(() => {
    fetchMock.reset();
  });

  it('initialize w/ no data', () => {
    const dfRepo = newEl();
    assert.deepEqual(dfRepo.commitRange, { begin: 0, end: 0 });
    assert.deepEqual(dfRepo.timeRange, { begin: 0, end: 0 });
    assert.isTrue(dfRepo.isEmpty);
    assert.isEmpty(dfRepo.header);
    assert.isEmpty(dfRepo.traces);
  });

  it('initialize w/ data', async () => {
    mockFrameStart(df, paramset, anomaly);

    const dfRepo = newEl();
    assert.equal(
      await dfRepo.resetTraces(
        {
          begin: now + 1,
          end: now + timeSpan * 10,
        },
        paramset
      ),
      10
    );
    // The trace key generated from generateFullDataFrame.
    const traceKey = ',key=0';
    assert.isTrue(sorted(dfRepo.header));
    assert.sameOrderedMembers(
      df.traceset[traceKey].slice(1, 11),
      dfRepo.traces[traceKey]
    );
    assert.equal(dfRepo.anomaly[traceKey]![95].bug_id, 555);
  });

  it('init data and extend range', async () => {
    mockFrameStart(df, paramset, anomaly);

    const dfRepo = newEl();
    assert.equal(
      await dfRepo.resetTraces(
        {
          begin: now,
          end: now + timeSpan * 10 - 1,
        },
        paramset
      ),
      10
    );

    // The trace key generated from generateFullDataFrame.
    const traceKey = ',key=0';
    assert.isUndefined(dfRepo.anomaly[traceKey]![105]);

    assert.equal(await dfRepo.extendRange(timeSpan * 10), 10);
    assert.isTrue(sorted(dfRepo.header));
    assert.lengthOf(dfRepo.header, 20);
    assert.sameOrderedMembers(
      df.traceset[traceKey].slice(0, 20),
      dfRepo.traces[traceKey]
    );
    assert.equal(dfRepo.anomaly[traceKey]![105].bug_id, 1515);
  });

  it('init data and extend range both ways', async () => {
    const df = generateFullDataFrame({ begin: 100, end: 201 }, now, 1, [
      timeSpan,
    ]);
    mockFrameStart(df, paramset);

    const dfRepo = newEl();
    assert.equal(
      await dfRepo.resetTraces(
        {
          begin: now + timeSpan * 40,
          end: now + timeSpan * 60 - 1,
        },
        paramset
      ),
      20,
      'init 20 traces'
    );

    assert.equal(
      await dfRepo.extendRange(-timeSpan * 20),
      20,
      'extend backward first 20'
    );
    assert.equal(
      await dfRepo.extendRange(timeSpan * 20),
      20,
      'extend forward first 20'
    );
    assert.equal(
      await dfRepo.extendRange(-timeSpan * 20),
      20,
      'extend backward second 20'
    );
    assert.equal(
      await dfRepo.extendRange(timeSpan * 20),
      20,
      'extend forward second 20'
    );
    assert.isTrue(sorted(dfRepo.header));
    assert.lengthOf(dfRepo.header, 100);
    assert.sameOrderedMembers(
      df.traceset[',key=0'].slice(0, 100),
      dfRepo.traces[',key=0']
    );
  });

  it('init data and reset repo', async () => {
    mockFrameStart(df, paramset);

    const dfRepo = newEl();
    assert.equal(
      await dfRepo.resetTraces(
        {
          begin: now + 1,
          end: now + timeSpan * 10,
        },
        paramset
      ),
      10
    );
    assert.isTrue(sorted(dfRepo.header));
    assert.sameOrderedMembers(
      df.traceset[',key=0'].slice(1, 11),
      dfRepo.traces[',key=0']
    );

    assert.equal(
      await dfRepo.resetTraces(
        {
          begin: now + timeSpan * 10 + 1,
          end: now + timeSpan * 25,
        },
        paramset
      ),
      15
    );
    assert.isTrue(sorted(dfRepo.header));
    assert.lengthOf(dfRepo.header, 15);
    assert.sameOrderedMembers(
      df.traceset[',key=0'].slice(11, 26),
      dfRepo.traces[',key=0']
    );
  });
});
