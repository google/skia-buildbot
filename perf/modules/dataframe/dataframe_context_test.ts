import { assert } from 'chai';

import { DataFrameRepository } from './dataframe_context';
import { range } from './index';
import './dataframe_context';

import { ColumnHeader, ReadOnlyParamSet } from '../json';
import fetchMock from 'fetch-mock';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import {
  generateAnomalyMap,
  generateFullDataFrame,
  mockFrameStart,
  mockUserIssues,
} from './test_utils';

const now = 1726081856; // an arbitrary UNIX time;
const timeSpan = 89; // an arbitrary prime number for time span between commits .

const sorted = (a: (ColumnHeader | null)[]) => {
  return a.every((v, idx, arr) => {
    return idx === 0 || v!.offset > arr[idx - 1]!.offset;
  });
};

describe('dataframe-repository', () => {
  const newEl = setUpElementUnderTest<DataFrameRepository>('dataframe-repository-sk');

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
    assert.equal(await dfRepo.resetTraces(range(now + 1, now + timeSpan * 10), paramset), 10);

    // The trace key generated from generateFullDataFrame.
    const traceKey = ',key=0';
    assert.isTrue(sorted(dfRepo.header));
    assert.sameOrderedMembers(df.traceset[traceKey].slice(1, 11), dfRepo.traces[traceKey]);
    const anomalies = dfRepo.anomaly![traceKey];
    assert.equal(anomalies![95].bug_id, 555);
  });

  it('init data and extend range', async () => {
    mockFrameStart(df, paramset, anomaly);

    const dfRepo = newEl();
    assert.equal(await dfRepo.resetTraces(range(now, now + timeSpan * 10 - 1), paramset), 10);

    // The trace key generated from generateFullDataFrame.
    const traceKey = ',key=0';
    assert.isUndefined(dfRepo.anomaly![traceKey]![105]);

    assert.equal(await dfRepo.extendRange(timeSpan * 10), 10);
    assert.isTrue(sorted(dfRepo.header));
    assert.lengthOf(dfRepo.header, 20);
    assert.sameOrderedMembers(df.traceset[traceKey].slice(0, 20), dfRepo.traces[traceKey]);
    const anomalies = dfRepo.anomaly![traceKey];
    assert.equal(anomalies![105].bug_id, 1515);
  });

  it('init data and extend range both ways', async () => {
    const df = generateFullDataFrame(range(100, 201), now, 1, [timeSpan]);
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

    assert.equal(await dfRepo.extendRange(-timeSpan * 20), 20, 'extend backward first 20');
    assert.equal(await dfRepo.extendRange(timeSpan * 20), 20, 'extend forward first 20');
    assert.equal(await dfRepo.extendRange(-timeSpan * 20), 20, 'extend backward second 20');
    assert.equal(await dfRepo.extendRange(timeSpan * 20), 20, 'extend forward second 20');
    assert.isTrue(sorted(dfRepo.header));
    assert.lengthOf(dfRepo.header, 100);
    assert.sameOrderedMembers(df.traceset[',key=0'].slice(0, 100), dfRepo.traces[',key=0']);
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
    assert.sameOrderedMembers(df.traceset[',key=0'].slice(1, 11), dfRepo.traces[',key=0']);

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
    assert.sameOrderedMembers(df.traceset[',key=0'].slice(11, 26), dfRepo.traces[',key=0']);
  });

  it('extend range w/ chunks', async () => {
    const df = generateFullDataFrame(range(100, 150), now, 3, [timeSpan]);
    mockFrameStart(df, paramset);
    const start = range(now + timeSpan * 20, now + timeSpan * 30 - 1);
    const key1 = ',key=0',
      key2 = ',key=1';

    // Stress test different slice chunks.
    [10, timeSpan, timeSpan - 1, timeSpan * 3].every((chunkSize) =>
      it(`chunk size ${chunkSize}`, async () => {
        const dfRepo = newEl((el) => (el['chunkSize'] = chunkSize));
        assert.equal(await dfRepo.resetTraces(start, paramset), 10);
        assert.isTrue(sorted(dfRepo.header));
        assert.sameOrderedMembers(df.traceset[key1].slice(0, 10), dfRepo.traces[key1]);

        assert.equal(await dfRepo.extendRange(timeSpan * 20), 20);
        assert.isTrue(sorted(dfRepo.header));
        assert.lengthOf(dfRepo.header, 30);

        assert.equal(await dfRepo.extendRange(-timeSpan * 20), 20);
        assert.isTrue(sorted(dfRepo.header));
        assert.lengthOf(dfRepo.header, 30);

        assert.sameOrderedMembers(df.traceset[key1], dfRepo.traces[key1]);
        assert.sameOrderedMembers(df.traceset[key2], dfRepo.traces[key2]);
      })
    );
  });

  it('gets user issues', async () => {
    mockUserIssues(false);

    const dfRepo = newEl();
    await dfRepo.getUserIssues([',a=1,', ',b=1,', ',c=1,'], 100, 200);

    const expected = {
      ',a=1,': { 1: { bugId: 2345, x: -1, y: -1 } },
      ',b=1,': { 3: { bugId: 3456, x: -1, y: -1 } },
      ',c=1,': { 8: { bugId: 4567, x: -1, y: -1 } },
    };
    assert.deepEqual(dfRepo.userIssues, expected);
  });

  it('fails to get user issues', async () => {
    mockUserIssues(true);
    const dfRepo = newEl();
    try {
      await dfRepo.getUserIssues([',a=1,', ',b=1,', ',c=1,'], 100, 200);
    } catch (err: any) {
      assert.equal(err.message, 'Bad network response: Internal Server Error');
    }
  });

  it('updates user issues', async () => {
    const df = generateFullDataFrame(range(100, 201), now, 1, [timeSpan]);
    mockFrameStart(df, paramset);

    const dfRepo = newEl();
    const obj1 = { 1: { bugId: 1234, x: 2, y: 3 } };
    const obj2 = { 3: { bugId: 3453, x: 8, y: 20 }, 8: { bugId: 5345, x: 29, y: 45 } };
    const obj3 = { 5: { bugId: 5675, x: 10, y: 30 } };
    dfRepo.userIssues = {
      ',key=0,': obj1,
      ',key=1,': obj2,
      ',key=2,': obj3,
    };

    const expected = {
      ',key=0,': obj1,
      ',key=1,': { 3: { bugId: 345, x: -1, y: -1 }, 8: { bugId: 5345, x: 29, y: 45 } },
      ',key=2,': obj3,
    };

    await dfRepo.updateUserIssue('key=1', 3, 345);

    assert.deepEqual(dfRepo.userIssues, expected);
  });

  it('updates user issues new trace', async () => {
    const df = generateFullDataFrame(range(100, 201), now, 1, [timeSpan]);
    mockFrameStart(df, paramset);

    const dfRepo = newEl();
    const obj1 = { 1: { bugId: 1234, x: 2, y: 3 } };
    const obj2 = { 3: { bugId: 3453, x: 8, y: 20 }, 8: { bugId: 5345, x: 29, y: 45 } };
    const obj3 = { 5: { bugId: 5675, x: 10, y: 30 } };
    dfRepo.userIssues = {
      ',key=0,': obj1,
      ',key=1,': obj2,
      ',key=2,': obj3,
    };

    const expected = {
      ',key=0,': obj1,
      ',key=1,': obj2,
      ',key=2,': obj3,
      ',key=3,': { 6: { bugId: 6767, x: -1, y: -1 } },
    };

    await dfRepo.updateUserIssue(',key=3,', 6, 6767);

    assert.deepEqual(dfRepo.userIssues, expected);
  });

  it('updates user issues no existing issues', async () => {
    const df = generateFullDataFrame(range(100, 201), now, 1, [timeSpan]);
    mockFrameStart(df, paramset);

    const dfRepo = newEl();

    const expected = {
      ',key=k,': { 6: { bugId: 6767, x: -1, y: -1 } },
    };

    await dfRepo.updateUserIssue('key=k', 6, 6767);

    assert.deepEqual(dfRepo.userIssues, expected);
  });

  it('getUserIssues ensures traceKeys have leading/trailing commas', async () => {
    // Mock the backend response with traceKeys with leading/trailing commas
    // and ensure the frontend keeps them.
    fetchMock.post('glob:/_/user_issues/', {
      body: JSON.stringify({
        UserIssues: [
          { UserId: 'test', TraceKey: ',a=1,', CommitPosition: 1, IssueId: 2345 },
          { UserId: 'test', TraceKey: ',b=1,', CommitPosition: 3, IssueId: 3456 },
        ],
      }),
      status: 200,
    });

    const dfRepo = newEl();
    await dfRepo.getUserIssues([',a=1,', ',b=1,'], 100, 200);

    const expected = {
      ',a=1,': { 1: { bugId: 2345, x: -1, y: -1 } },
      ',b=1,': { 3: { bugId: 3456, x: -1, y: -1 } },
    };
    assert.deepEqual(dfRepo.userIssues, expected);
  });

  it('updateUserIssue ensures traceKey has leading/trailing commas', async () => {
    const dfRepo = newEl();
    dfRepo.userIssues = {}; // Start with an empty userIssues map

    // Call updateUserIssue with a traceKey *without* leading/trailing commas
    await dfRepo.updateUserIssue('newKey=1', 10, 9999);

    const expected = {
      ',newKey=1,': { 10: { bugId: 9999, x: -1, y: -1 } },
    };
    assert.deepEqual(dfRepo.userIssues, expected);
  });

  it('clears the repository', async () => {
    mockFrameStart(df, paramset, anomaly);

    const dfRepo = newEl();
    await dfRepo.resetTraces(range(now + 1, now + timeSpan * 10), paramset);

    dfRepo.clear();

    assert.deepEqual(dfRepo.commitRange, { begin: 0, end: 0 });
    assert.deepEqual(dfRepo.timeRange, { begin: 0, end: 0 });
    assert.isTrue(dfRepo.isEmpty);
    assert.isEmpty(dfRepo.header);
    assert.isEmpty(dfRepo.traces);
    assert.isNull(dfRepo.anomaly);
    assert.isNull(dfRepo.userIssues);
  });

  it('truncation preserves newest data when extending past', async () => {
    // MAX_DATAPOINTS is 5000.
    // 1. Initial request: Load 100 commits (the "anchor").
    // 2. Extend past: Mock the response to return 6000 commits.
    // 3. Verify: We should have exactly 5000 commits, and they should
    //    include the original 100 commits from the initial request.

    const initialDf = generateFullDataFrame(range(7000, 7100), now + 7000 * timeSpan, 1, [
      timeSpan,
    ]);
    mockFrameStart(initialDf, paramset);

    const dfRepo = newEl();
    await dfRepo.resetTraces(
      {
        begin: now + timeSpan * 7000,
        end: now + timeSpan * 7099,
      },
      paramset
    );

    assert.lengthOf(dfRepo.header, 100);
    const anchorLastCommit = dfRepo.header[99]!.offset; // 7099

    // Reset mock to return a massive 6000-point dataframe from the past
    fetchMock.reset();
    // We fetch 6000 points before the anchor.
    // Anchor starts at 7000. So we fetch 1000 to 6999.
    const massivePastDf = generateFullDataFrame(range(1000, 7000), now + 1000 * timeSpan, 1, [
      timeSpan,
    ]);
    mockFrameStart(massivePastDf, paramset);

    // This extendRange call will trigger the truncation logic
    await dfRepo.extendRange(-timeSpan * 6000);

    // Array should be capped at 5000 points.
    assert.lengthOf(dfRepo.header, 5000);

    // Verify the anchor was preserved! It should be at the end of the array.
    const newFirstCommit = dfRepo.header[0]!.offset;
    const newLastCommit = dfRepo.header[4999]!.offset;

    // We had 100 (anchor) + 6000 (past) = 6100 total.
    // We keep 5000 newest.
    // 6100 - 5000 = 1100 oldest dropped.
    // Original range was 1000 to 7099.
    // 1000 + 1100 = 2100.
    // So new range should be 2100 to 7099.

    assert.equal(newLastCommit, anchorLastCommit, 'The newest anchor commit was not preserved!');
    assert.equal(newFirstCommit, 2100, 'The incorrect amount of oldest data was dropped!');

    // Verify traceset length
    assert.lengthOf(dfRepo.traces[',key=0'], 5000);
  });

  it('truncation preserves newest data when extending future', async () => {
    // MAX_DATAPOINTS is 5000.
    // 1. Initial request: Load 100 commits (the "anchor").
    // 2. Extend future: Mock the response to return 6000 commits.
    // 3. Verify: We should have exactly 5000 commits, and they should
    //    drop the original 100 commits from the initial request because they are now the "oldest".

    const initialDf = generateFullDataFrame(range(1000, 1100), now + 1000 * timeSpan, 1, [
      timeSpan,
    ]);
    mockFrameStart(initialDf, paramset);

    const dfRepo = newEl();
    await dfRepo.resetTraces(
      {
        begin: now + timeSpan * 1000,
        end: now + timeSpan * 1099,
      },
      paramset
    );

    assert.lengthOf(dfRepo.header, 100);

    // Reset mock to return a massive 6000-point dataframe from the future
    fetchMock.reset();
    const massiveFutureDf = generateFullDataFrame(range(1100, 7100), now + 1100 * timeSpan, 1, [
      timeSpan,
    ]);
    mockFrameStart(massiveFutureDf, paramset);

    // This extendRange call will trigger the truncation logic
    await dfRepo.extendRange(timeSpan * 6000);

    // Array should be capped at 5000 points.
    assert.lengthOf(dfRepo.header, 5000);

    // Verify the newest data is kept!
    const newFirstCommit = dfRepo.header[0]!.offset;
    const newLastCommit = dfRepo.header[4999]!.offset;

    // Total: 100 (anchor) + 6000 (future) = 6100.
    // Keep 5000 newest. Drop 1100 oldest.
    // 1000 + 1100 = 2100.
    // Range 2100 to 7099.

    assert.equal(newLastCommit, 7099, 'The newest commit was not preserved!');
    assert.equal(newFirstCommit, 2100, 'The incorrect amount of oldest data was dropped!');

    // Verify traceset length
    assert.lengthOf(dfRepo.traces[',key=0'], 5000);
  });
});
