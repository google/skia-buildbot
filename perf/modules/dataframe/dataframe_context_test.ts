import { assert } from 'chai';

import { DataFrameRepository } from './dataframe_context';
import './dataframe_context';
import { fromParamSet } from '../../../infra-sk/modules/query';

import { ColumnHeader, ReadOnlyParamSet, FrameRequest } from '../json';
import fetchMock from 'fetch-mock';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { findSubDataframe, range } from './index';

const now = 1726081856; // an arbitrary UNIX time;
const timeSpan = 89; // an arbitrary prime number for time span between commits .

interface dataframe {
  traceset: { [key: string]: number[] };
  header: { offset: number; timestamp: number }[];
}

const generateFullDataFrame = (
  range: range,
  time: number,
  tracesCount: number
) => {
  const offsets = Array(range.end - range.begin).fill(0);
  const traces = Array(tracesCount).fill(0);
  return {
    traceset: Object.fromEntries(
      traces.map((_, v) => [',key=' + v, offsets.map(() => Math.random())])
    ),
    header: offsets.map((_, v) => ({
      offset: range.begin + v,
      timestamp: time + v * timeSpan,
    })),
  } as dataframe;
};

const generateDataframe = (dataframe: dataframe, range: range) => {
  return {
    header: dataframe.header.slice(range.begin, range.end),
    traceset: Object.fromEntries(
      Object.keys(dataframe.traceset).map((k) => [
        k,
        dataframe.traceset[k].slice(range.begin, range.end),
      ])
    ),
  };
};

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

  function fetchMockFrameStart(commitRange: range, startTime: number) {
    const dataframe = generateFullDataFrame(commitRange, startTime, 1);
    fetchMock.post(
      {
        url: '/_/frame/start',
        method: 'POST',
        matchPartialBody: true,
        body: {
          queries: [fromParamSet(paramset)],
        },
      },
      (_, req) => {
        const body: FrameRequest = JSON.parse(req.body!.toString());
        const subrange = findSubDataframe(dataframe.header, {
          begin: body.begin,
          end: body.end,
        });

        return {
          status: 'Finished',
          messages: [{ key: 'Loading', value: 'Finished' }],
          results: {
            dataframe: generateDataframe(dataframe, subrange),
          },
        };
      }
    );
    return dataframe;
  }

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
    const df = fetchMockFrameStart({ begin: 90, end: 120 }, now);

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
  });

  it('init data and extend range', async () => {
    const df = fetchMockFrameStart({ begin: 90, end: 120 }, now);

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

    assert.equal(await dfRepo.extendRange(timeSpan * 10), 10);
    assert.isTrue(sorted(dfRepo.header));
    assert.lengthOf(dfRepo.header, 20);
    assert.sameOrderedMembers(
      df.traceset[',key=0'].slice(0, 20),
      dfRepo.traces[',key=0']
    );
  });

  it('init data and extend range both ways', async () => {
    const df = fetchMockFrameStart({ begin: 100, end: 201 }, now);

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
    const df = fetchMockFrameStart({ begin: 90, end: 120 }, now);

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
