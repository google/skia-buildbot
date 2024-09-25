import fetchMock, { FetchMockStatic } from 'fetch-mock';
import {
  DataFrame,
  ReadOnlyParamSet,
  ColumnHeader,
  Trace,
  TraceSet,
  FrameRequest,
} from '../json';
import { findSubDataframe, range } from './index';
import { fromParamSet } from '../../../infra-sk/modules/query';

/**
 * Generate a dataframe set as followings:
 * {
 *  header: [ {
 *    offset: range.begin,
 *    timesteamp: time + timeSpan * ...(range.end - range.begin)
 *  } ...],
 *  traceset: {
 *    ",key=0": [rand()...],
 *    ",key=(range.end-range.begin)": [rand()...],
 *  },
 * }
 *
 * There will be [range.end - range.begin] number of consecutive offsets and
 * timestamped with timeSpan intervals.
 *
 * @param range The start and end commit position (offset range)
 * @param time  The start of the timestamp
 * @param tracesCount The number of traces
 * @param timeSpan The time interval for timestamp
 * @returns DataFrame
 */
export const generateFullDataFrame = (
  range: range,
  time: number,
  tracesCount: number,
  timeSpan: number
): DataFrame => {
  const offsets = Array(range.end - range.begin).fill(0);
  const traces = Array(tracesCount).fill(0);
  return {
    header: offsets.map(
      (_, v) =>
        ({
          offset: range.begin + v,
          timestamp: time + v * timeSpan,
        }) as ColumnHeader
    ),
    traceset: Object.fromEntries(
      traces.map((_, v) => [
        ',key=' + v,
        offsets.map(() => Math.random()) as Trace,
      ])
    ) as TraceSet,
    skip: 0,
    paramset: ReadOnlyParamSet({}),
  };
};

/**
 * Generate a new sub DataFrame from another DataFrame.
 *
 * @param dataframe The full dataframe
 * @param range The index range of the dataframe
 * @returns
 *  The new copy of DataFrame containing the subrange from the full Dataframe.
 */
export const generateSubDataframe = (
  dataframe: DataFrame,
  range: range
): DataFrame => {
  return {
    header: dataframe.header!.slice(range.begin, range.end),
    traceset: Object.fromEntries(
      Object.keys(dataframe.traceset).map((k) => [
        k,
        dataframe.traceset[k].slice(range.begin, range.end),
      ])
    ) as TraceSet,
    skip: 0,
    paramset: ReadOnlyParamSet({}),
  };
};

export const mockFrameStart = (
  df: DataFrame,
  paramset: ReadOnlyParamSet,
  mock: FetchMockStatic = fetchMock
) => {
  mock.post(
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
      const subrange = findSubDataframe(df.header!, {
        begin: body.begin,
        end: body.end,
      });

      return {
        status: 'Finished',
        messages: [{ key: 'Loading', value: 'Finished' }],
        results: {
          dataframe: generateSubDataframe(df, subrange),
        },
      };
    }
  );
  return mock;
};
