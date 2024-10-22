/**
 * @module modules/dataframe/dataframe_context
 * @description <dataframe-repository-sk/>
 *
 * DataFrameRepository manages a local cache of the traces.
 *
 * The class loads the traces with the initial paramset and extends using the
 * same paramset. The paramset can only be reset by reloading the entire
 * traces, that is, paramset being immutable.
 *
 * @provide
 * dataframeContext: the DataFrame storing the current available data.
 *
 * @provide
 * dataframeLoadingContext: whether there is a request in flight.
 *
 * @provide
 * dataframeRepoContext: the repo object itself to manage data.
 *
 * @example
 * In the html page, you attach the <dataframe-repository-sk> as a parent node,
 * then in the child node implementation, you can request the context by:
 * @consume({context:dataframeContext})
 * dataframe: DataFrame
 *
 * this.dataframe will be assigned whenever there is a change.
 */
import { createContext, provide } from '@lit/context';
import { LitElement } from 'lit';
import { customElement } from 'lit/decorators.js';

import { mergeAnomaly, range } from './index';
import { fromParamSet } from '../../../infra-sk/modules/query';
import { AnomalyMap, ColumnHeader, ShiftRequest, ShiftResponse } from '../json';
import { DataFrame, FrameRequest, FrameResponse, Trace, TraceSet, ReadOnlyParamSet } from '../json';
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

// Slice a range into smaller ranges with each size being chunkSize.
// The range is inclusive on both ends, the lengh is (end - begin) + 1.
// e.g. sliceRange({begin: 0, end:20}, 10) yields:
//  {begin: 0, end: 9}
//  {begin: 10, end: 19}
//  {begin: 20, end: 20}
// And each subrange is also inclusive on both ends.
export const sliceRange = ({ begin, end }: range, chunkSize: number) => {
  if (chunkSize <= 0) {
    return [{ begin, end }];
  }

  // The range is inclusive on both ends
  const span = end - begin + 1;
  const slices = Math.ceil(span / chunkSize);

  // Shortcut as this will be most of cases.
  if (slices === 1) {
    return [{ begin, end }];
  }

  // The first (n - 1) slices and then concat the last slice as the last one
  // usually doesn't have the same length.
  return Array.from({ length: slices - 1 }, (_, k): range => {
    return {
      begin: begin + k * chunkSize,
      end: begin + (k + 1) * chunkSize - 1,
    };
  }).concat({ begin: begin + (slices - 1) * chunkSize, end: end });
};

// This context provides the dataframe when it is ready to use from the data
// store, typically a remote server or a local mock.
export const dataframeContext = createContext<DataFrame>(Symbol('dataframe-context'));

export const dataframeAnomalyContext = createContext<AnomalyMap>(
  Symbol('dataframe-anomaly-context')
);

// This context indicates whether there is an ongoing dataframe request.
export const dataframeLoadingContext = createContext<boolean>(Symbol('dataframe-loading-context'));

// This context prodides the data repository to query the data.
export const dataframeRepoContext = createContext<DataFrameRepository>(
  Symbol('dataframe-repo-context')
);

const emptyResolver = (_1: number, _2: DataFrame, _3: AnomalyMap) => {};

@customElement('dataframe-repository-sk')
export class DataFrameRepository extends LitElement {
  private static shiftUrl = '/_/shift/';

  private static frameStartUrl = '/_/frame/start';

  private _paramset = ReadOnlyParamSet({});

  // Most of logic to create the request is still in explore-simple-sk, we make
  // it simple to refactor that takes whatever it has and simply extending its
  // range.
  private _baseRequest?: FrameRequest;

  private _traceset = TraceSet({});

  private _header: (ColumnHeader | null)[] = [];

  // When the requests are more than this size, we slice into separate requests
  // so they run concurrently to improve data fetching performance.
  private chunkSize = 30 * 24 * 60 * 60; // 1 month in seconds.

  @provide({ context: dataframeContext })
  dataframe: DataFrame = {
    traceset: TraceSet({}),
    header: [],
    paramset: ReadOnlyParamSet({}),
    skip: 0,
  };

  @provide({ context: dataframeLoadingContext })
  loading = false;

  @provide({ context: dataframeRepoContext })
  dfRepo = this;

  @provide({ context: dataframeAnomalyContext })
  anomaly: AnomalyMap = null;

  // This element doesn't render anything and all the children should be
  // attached to itself directly.
  // Note, <slot></slot> is not needed if there is no shadow root.
  protected createRenderRoot(): HTMLElement | DocumentFragment {
    return this;
  }

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
    if (!this._baseRequest) {
      this._baseRequest = this.generateFrameRequest(range);
    }
    const req = structuredClone(this._baseRequest);
    req.begin = range.begin;
    req.end = range.end;
    const resp = await startRequest(DataFrameRepository.frameStartUrl, req, 1000).catch(() => {
      return null;
    });
    if (resp === null) {
      // We silently fails when the server returns an error for now.
      console.log('fetch frame response failed.');
      return {} as FrameResponse;
    }
    if (resp?.status !== 'Finished') {
      // This usually happens when there is no commits for the  given date,
      // We emit an empty range so the UI still functions just w/o any data,
      // The caller may handle this empty return gracefully on its own.
      console.log(
        'request range (',
        new Date(range.begin * 1000),
        new Date(range.end * 1000),
        ') failed with msg:',
        resp?.messages
      );
      return {} as FrameResponse;
    }
    const msg = messageByName(resp.messages, 'Message');
    if (msg) {
      return Promise.reject(msg);
    }
    return resp.results as FrameResponse;
  }

  /**
   * Reset the dataframe and its corresponding FrameRequest.
   *
   * This can be used to refill the data if it is from a different source, and this
   * can take over and extend the time frame.
   *
   * @param dataframe The full dataframe from the request.
   * @param request The completed FrameRequet that was sent for the dataframe.
   */
  async resetWithDataframeAndRequest(
    dataframe: DataFrame,
    anomalies: AnomalyMap,
    request: FrameRequest
  ) {
    this._baseRequest = request;
    this._baseRequest.request_type = 0; // change to timestamp-based query.

    this.dataframe = dataframe;
    this.anomaly = mergeAnomaly(this.anomaly, anomalies);
    this._header = dataframe.header || [];
    this._traceset = dataframe.traceset;
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
    let resolver = emptyResolver;
    const curRequest = this._requestComplete;
    this._requestComplete = new Promise((resolve) => {
      resolver = (n, df, anomaly) => {
        this.dataframe = df;
        this.anomaly = mergeAnomaly(this.anomaly, anomaly);
        this.loading = false;
        resolve(n);
      };
    });

    await curRequest;

    this.loading = true;
    this._paramset = param;
    const resp = await this.requestNewRange(range);
    this._traceset = resp.dataframe?.traceset || TraceSet({});
    this._header = resp.dataframe?.header || [];

    const totalTraces = resp.dataframe?.header?.length || 0;
    resolver(
      totalTraces,
      {
        traceset: this._traceset,
        header: this._header,
        paramset: this._paramset,
        skip: 0,
      },
      resp.anomalymap
    );
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
    let resolver = emptyResolver;
    const curRequest = this._requestComplete;
    this._requestComplete = new Promise((resolve) => {
      resolver = (n, df, anomaly) => {
        this.loading = false;
        this.dataframe = df;
        this.anomaly = mergeAnomaly(this.anomaly, anomaly);
        resolve(n);
      };
    });
    await curRequest;
    this.loading = true;
    const range = deltaRange(this.timeRange, offsetInSeconds);
    const ranges = sliceRange(range, this.chunkSize);
    const allResponses: Promise<FrameResponse>[] = [];
    for (let slice = 0; slice < ranges.length; slice++) {
      allResponses.push(this.requestNewRange(ranges[slice]));
    }

    // Fetch and sort the frame responses so they can appended consecutively.
    const sortedResponses = (await Promise.all(allResponses))
      .filter(
        // Filter responses with valid dataframes with actual traces.
        (fr) =>
          fr.dataframe &&
          (fr.dataframe.header?.length || 0) > 0 &&
          Object.keys(fr.dataframe.traceset).length > 0
      )
      .sort(
        (a: FrameResponse, b: FrameResponse) =>
          a.dataframe!.header![0]!.offset - b.dataframe!.header![0]!.offset
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

    const anomaly = sortedResponses.reduce((pre, cur) => mergeAnomaly(pre, cur.anomalymap), {});
    resolver(
      totalTraces,
      {
        traceset: this._traceset,
        header: this._header,
        paramset: this._paramset,
        skip: 0,
      },
      anomaly
    );
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
