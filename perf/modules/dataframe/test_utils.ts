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

// Generates an array where the values are repeated from the template.
const generateTraceFromTemplate = (template: number[], size: number) => {
  return Array(Math.ceil(size / template.length))
    .fill(template)
    .flat()
    .filter((v) => typeof v === 'number')
    .slice(0, size);
};

// Check if the given array contains at least one number.
const containsAtLeastOneNumber = (values: any[] | null) =>
  values?.filter((v) => typeof v === 'number').length || 0 > 0;

/**
 * Generate a dataframe set as followings:
 * {
 *  header: [ {
 *    offset: range.begin,
 *    timesteamp: time + timeSpan * ...(range.end - range.begin)
 *  } ...],
 *  traceset: {
 *    ",key=0": [rand()...],
 *    ",key=(tracesCount)": [rand()...],
 *  },
 * }
 *
 * There will be [range.end - range.begin] number of consecutive offsets and
 * timestamped with timeSpan intervals. The data can be filled with the given
 * array by repeating itself to fill the number of commits.
 *
 * The generated trace values can be controlled by the optional traceValues.
 * The traceValues provides a template trace values to copy from. If not given,
 * the trace values will be generated randomly, which will be different in each
 * run. This can be used to test a stable trace that produce the same chart, or
 * a specific trace values you need to validate.
 * @param range The start and end commit position (offset range)
 * @param time  The start of the timestamp
 * @param tracesCount The number of traces
 * @param timeSpans The time intervals for timestamp, multiple spans will make
 *  each offset advance in a different stamp
 * @param traceValues The trace template numbers to copy from, or random if
 *  not given. See above.
 * @returns DataFrame
 */
export const generateFullDataFrame = (
  range: range,
  time: number,
  tracesCount: number,
  timeSpans: number[],
  traceValues: (number[] | null)[] = []
): DataFrame => {
  const offsets = Array(range.end - range.begin).fill(0);
  const traces = Array(tracesCount).fill(0);

  // A helper function to generate the timestamp at index.
  // The timeSpans are accumulated one by one.
  const timeSpan = (idx: number) =>
    Array(idx)
      .fill(0)
      .reduce((pre, _, idx) => pre + timeSpans[idx % timeSpans.length], 0);
  return {
    header: offsets.map(
      (_, v) =>
        ({
          offset: range.begin + v,
          timestamp: time + timeSpan(v),
        }) as ColumnHeader
    ),
    traceset: Object.fromEntries(
      traces.map((_, v) => [
        ',key=' + v,
        containsAtLeastOneNumber(traceValues[v])
          ? generateTraceFromTemplate(traceValues[v]!, offsets.length)
          : (offsets.map(() => Math.random()) as Trace),
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
  delayInMS: number = 0,
  mock: FetchMockStatic = fetchMock
) => {
  mock.post(
    {
      url: '/_/frame/start',
      method: 'POST',
      matchPartialBody: true,
      delay: delayInMS,
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
