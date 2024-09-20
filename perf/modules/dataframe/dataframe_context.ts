import { range } from './index';
import { fromParamSet } from '../../../infra-sk/modules/query';
import { ColumnHeader, ShiftRequest, ShiftResponse } from '../json';
import {
  FrameRequest,
  FrameResponse,
  Trace,
  TraceSet,
  ReadOnlyParamSet,
} from '../json';
import { startRequest, messageByName } from '../progress/progress';

// Shift the range by offset.
// Note, shitf [0, 10] by 10 will give [1, 11].
const deltaRange = (range: range, offset: number) => {
  const { begin, end } = range;
  if (offset > 0) {
    return { begin: end + 1, end: end + offset + 1 };
  } else {
    return { begin: begin + offset - 1, end: begin - 1 };
  }
};

const sliceRange = ({ begin, end }: range, chuckSize: number) => {
  if (chuckSize <= 0) {
    return [{ begin, end }];
  }

  const span = end - begin;
  const slices = Math.ceil(span / chuckSize);
  if (slices === 1) {
    return [{ begin, end }];
  }

  return [{ begin, end: begin + chuckSize }].concat(
    Array.from({ length: slices - 1 }, (_, k): range => {
      return {
        begin: begin + (k - 1) * chuckSize + 1,
        end: begin + k * chuckSize,
      };
    })
  );
};

/**
 * DataFrameRepository manages a local cache of the traces.
 *
 * The class loads the traces with the initial paramset and extends using the
 * same paramset. The paramset can only be reset by reloading the entire
 * traces, that is, paramset being immutable.
 *
 * Note: this is still WIP.
 *
 * TODO(haowoo):
 *  1. Errors are not handled, the responses errors will propogage.
 *  2. The ranges should also be merged.
 *  3. Upgrade to LitElement and "provides" dataframe.
 */
export class DataFrameRepository {
  // baseUrl is used to set locally and test against sandbox services.
  private static baseUrl = '';

  private static shiftUrl = DataFrameRepository.baseUrl + '/_/shift/';

  private static frameStartUrl = DataFrameRepository.baseUrl + '/_/frame/start';

  private _paramset = ReadOnlyParamSet({});

  private _traceset = TraceSet({});

  private _header: (ColumnHeader | null)[] = [];

  // When the requests are more than this size, we slice into separate requests
  // so they run concurrently to improve data fetching performance.
  private chunkSize = 30 * 24 * 60 * 60; // 1 month in seconds.

  get paramset() {
    return this._paramset;
  }

  get traces() {
    return this._traceset;
  }

  get header() {
    return this._header;
  }

  get queryString() {
    return fromParamSet(this.paramset);
  }

  get isEmpty(): boolean {
    const header = this.header;
    return !header || header!.length === 0;
  }

  getRange(key: 'offset' | 'timestamp'): range {
    if (this.isEmpty) {
      return { begin: 0, end: 0 };
    }
    const header = this.header!;
    const len = header!.length;
    if (len > 1) {
      return { begin: header[0]![key], end: header[len - 1]![key] };
    } else {
      return { begin: header[0]![key], end: header[0]![key] };
    }
  }

  get commitRange(): range {
    return this.getRange('offset');
  }

  get timeRange(): range {
    return this.getRange('timestamp');
  }

  private addTraceset(header: (ColumnHeader | null)[], traceset: TraceSet) {
    if (header.length <= 0) {
      return;
    }

    // TODO(haowoo): this now is simply to prepend or append the given data,
    //  however, we should also try to replace duplicate if there is time
    //  overlap, or insert into middle of the dataset. This is not needed at
    //  the moment.
    const isAfter = header[0]!.timestamp >= this.timeRange.end;
    if (isAfter) {
      this._header = (this.header || []).concat(header);
    } else {
      this._header = header.concat(this.header);
    }

    let traceHead = TraceSet({}),
      traceTail = this.traces;
    if (isAfter) {
      [traceHead, traceTail] = [traceTail, traceHead];
    }
    Object.keys(this.traces).forEach((key) => {
      this._traceset[key] = (traceHead[key] || [])
        .concat(...traceset[key])
        .concat(traceTail[key] || []) as Trace;
    });
  }

  // Track the current ongoing request.
  // Initiazlied as resolved because there are no requests.
  private _requestComplete = Promise.resolve(0);

  /**
   * Returns a Promise that resolves when the ongoing request completes.
   */
  get requestComplete(): Promise<number> {
    return this._requestComplete;
  }

  private generateFrameRequest({ begin, end }: range) {
    return {
      begin: begin,
      end: end,
      request_type: 0,
      queries: [this.queryString],
      tz: Intl.DateTimeFormat().resolvedOptions().timeZone,
    } as FrameRequest;
  }

  private async requestNewRange(range: range) {
    const req = this.generateFrameRequest(range);
    const resp = await startRequest(
      DataFrameRepository.frameStartUrl,
      req,
      1000
    );
    if (resp.status !== 'Finished') {
      return Promise.reject(resp.messages);
    }
    const msg = messageByName(resp.messages, 'Message');
    if (msg) {
      return Promise.reject(msg);
    }
    return resp.results as FrameResponse;
  }

  /**
   * Reset and load the dataframe with the ParamSet.
   *
   * @param range The requested timestamp range in seconds.
   * @param param The paramset to fetch the dataframe.
   * @returns The promise that resolves to the length of traces when the
   *  request completes.
   */
  async resetTraces(range: range, param: ReadOnlyParamSet) {
    let resolver = (_: number) => {};
    const curRequest = this._requestComplete;
    this._requestComplete = new Promise((resolve) => {
      resolver = resolve;
    });
    await curRequest;

    this._paramset = param;
    const resp = await this.requestNewRange(range);
    this._traceset = resp.dataframe?.traceset || TraceSet({});
    this._header = resp.dataframe?.header || [];

    const totalTraces = resp.dataframe?.header?.length || 0;
    resolver(totalTraces);
    return totalTraces;
  }

  /**
   * Prepend or append additional traces to the dataframe using the same
   * ParamSet.
   *
   * @param offsetInSeconds The offset extending, positive means forward,
   *  negative means backward.
   * @returns The promise that resolves to the length of additional traces when
   *  the request completes.
   */
  async extendRange(offsetInSeconds: number) {
    let resolver = (_: number) => {};
    const curRequest = this._requestComplete;
    this._requestComplete = new Promise((resolve) => {
      resolver = resolve;
    });
    await curRequest;
    const range = deltaRange(this.timeRange, offsetInSeconds);
    const ranges = sliceRange(range, this.chunkSize);
    const allResponses: Promise<FrameResponse>[] = [];
    for (let slice = 0; slice < ranges.length; slice++) {
      allResponses.push(this.requestNewRange(ranges[slice]));
    }

    // Fetch and sort the frame responses so they can appended consecutively.
    const sortedResponses = (await Promise.all(allResponses)).sort(
      (a: FrameResponse, b: FrameResponse) => {
        return (
          a.dataframe!.header![0]!.offset - b.dataframe!.header![0]!.offset
        );
      }
    );
    const totalTraces = sortedResponses
      .map((resp) => resp.dataframe!.header?.length || 0)
      .reduce((count, cur) => count + cur, 0);

    const header = ([] as (ColumnHeader | null)[]).concat(
      ...sortedResponses.map((resp) => resp.dataframe!.header)
    );

    const traceset = TraceSet({});
    Object.keys(this.traces).forEach((key) => {
      traceset[key] = (traceset[key] || []).concat(
        ...sortedResponses.map((resp) => resp.dataframe!.traceset[key])
      ) as Trace;
    });
    this.addTraceset(header, traceset);

    resolver(totalTraces);
    return totalTraces;
  }

  protected async shift(commitRange: range, offset: number = -200) {
    const req = deltaRange(commitRange, offset) as ShiftRequest;
    const resp = await fetch(DataFrameRepository.shiftUrl, {
      method: 'POST',
      body: JSON.stringify(req),
      headers: {
        'Content-Type': 'application/json',
      },
    });
    if (!resp.ok) {
      return Promise.reject(resp.statusText);
    }
    return (await resp.json()) as ShiftResponse;
  }
}
