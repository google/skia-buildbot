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
import { load } from '@google-web-components/google-chart/loader';
import { createContext, provide } from '@lit/context';
import { LitElement } from 'lit';
import { customElement } from 'lit/decorators.js';

import { mergeAnomaly, removeAnomaly, range } from './index';
import { fromParamSet } from '../../../infra-sk/modules/query';
import {
  AnomalyMap,
  ColumnHeader,
  GetUserIssuesForTraceKeysRequest,
  ShiftRequest,
  TraceMetadata,
} from '../json';
import { DataFrame, FrameRequest, FrameResponse, Trace, TraceSet, ReadOnlyParamSet } from '../json';
import { convertFromDataframe } from '../common/plot-builder';
import { formatSpecialFunctions } from '../paramtools';
import { MISSING_DATA_SENTINEL } from '../const/const';
import { DataService, DataServiceError } from '../data-service/data-service';

// Holds issue data for a single data point.
// x and y corresponds to the location of the data point on the chart
export interface IssueDetail {
  bugId: number;
  x: number;
  y: number;
}

// A map of user reported issues generally constructed from UserIssueResponse
export type UserIssueMap = { [key: string]: { [key: number]: IssueDetail } } | null;

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

export type DataTable = google.visualization.DataTable | null;

// This context provides the dataframe when it is ready to use from the data
// store, typically a remote server or a local mock.
export const dataframeContext = createContext<DataFrame>(Symbol('dataframe-context'));

export const dataTableContext = createContext<DataTable>(Symbol('datatable-context'));

export const traceColorMapContext = createContext<Map<string, string>>(
  Symbol('trace-color-map-context')
);

export const dataframeAnomalyContext = createContext<AnomalyMap>(
  Symbol('dataframe-anomaly-context')
);

// This context tracks the bugaizer user issues associated
// with the dataframe points
export const dataframeUserIssueContext = createContext<UserIssueMap>(
  Symbol('dataframe-user-issue-context')
);

// This context indicates whether there is an ongoing dataframe request.
export const dataframeLoadingContext = createContext<boolean>(Symbol('dataframe-loading-context'));

// This context prodides the data repository to query the data.
export const dataframeRepoContext = createContext<DataFrameRepository>(
  Symbol('dataframe-repo-context')
);

const emptyResolver = (_1: number) => {};

const MAX_DATAPOINTS = 5000;

@customElement('dataframe-repository-sk')
export class DataFrameRepository extends LitElement {
  // The promise that resolves when the Google Chart API is loaded.
  private static loadPromise = load();

  private _paramset = ReadOnlyParamSet({});

  // Most of logic to create the request is still in explore-simple-sk, we make
  // it simple to refactor that takes whatever it has and simply extending its
  // range.
  private _baseRequest?: FrameRequest;

  private _traceset = TraceSet({});

  private _traceMetadata: TraceMetadata[] | null = [];

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
    traceMetadata: [],
  };

  @provide({ context: dataTableContext })
  data: DataTable = null;

  @provide({ context: dataframeLoadingContext })
  loading = false;

  @provide({ context: dataframeRepoContext })
  dfRepo = this;

  @provide({ context: dataframeAnomalyContext })
  anomaly: AnomalyMap = null;

  @provide({ context: dataframeUserIssueContext })
  userIssues: UserIssueMap = null;

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

  private addTraceInfo(
    header: (ColumnHeader | null)[],
    traceset: TraceSet,
    traceMetadata: TraceMetadata[] | null
  ) {
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

    if (this._header.length > MAX_DATAPOINTS) {
      const excess = this._header.length - MAX_DATAPOINTS;
      if (isAfter) {
        this._header.splice(0, excess);
        Object.keys(this._traceset).forEach((key) => {
          (this._traceset[key] as number[]).splice(0, excess);
        });
      } else {
        this._header.splice(MAX_DATAPOINTS);
        Object.keys(this._traceset).forEach((key) => {
          (this._traceset[key] as number[]).splice(MAX_DATAPOINTS);
        });
      }
    }

    if (traceMetadata !== null) {
      if (this._traceMetadata === null) {
        this._traceMetadata = [];
      }
      traceMetadata.forEach((traceData) => {
        let exists = false;
        for (let i = 0; i < this._traceMetadata!.length; i++) {
          if (this._traceMetadata![i].traceid === traceData.traceid) {
            exists = true;
            Object.keys(traceData.commitLinks!).forEach((commitIdKey) => {
              const commitId = parseInt(commitIdKey);
              if (!(commitId in this._traceMetadata![i].commitLinks!)) {
                // This commit is not in the existing metadata, so let's add it.
                this._traceMetadata![i].commitLinks![commitId] = traceData.commitLinks![commitId];
              }
            });
          }
          if (!exists) {
            // This means the trace is not present in existing metadata,
            // so let's add the entire thing.
            this._traceMetadata!.push(traceData);
          }
        }
      });
    }
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
    if (req.begin === req.end) {
      return {} as FrameResponse;
    }

    try {
      const resp = await DataService.getInstance().sendFrameRequest(req, {
        onMessage: (msg) => {
          throw new Error(msg);
        },
        pollingIntervalMs: 1000,
      });
      return resp;
    } catch (e: any) {
      if (e instanceof DataServiceError) {
        console.log(
          'request range (',
          new Date(range.begin * 1000),
          new Date(range.end * 1000),
          ') failed with msg:',
          e.message
        );
        return {} as FrameResponse;
      }

      if (e.message) {
        return Promise.reject(e.message);
      }
      console.log('fetch frame response failed.', e);
      return {} as FrameResponse;
    }
  }

  private async setDataFrame(df: DataFrame) {
    this.dataframe = df;

    await DataFrameRepository.loadPromise;

    // We could possibly merge with new data w/o recreating the entire table.
    this.data = google.visualization.arrayToDataTable(convertFromDataframe(df, 'both')!);
  }

  /**
   * Get the Anomaly.
   *
   * @param trace The trace name, typically this is the test name.
   * @param commit The commit position
   * @returns The Anomaly if it exits at given position, null otherwise.
   */
  getAnomaly(trace: string, commit: number) {
    if (!this.anomaly) {
      return null;
    }

    const traceAnomalies = this.anomaly[trace];
    return (traceAnomalies && traceAnomalies![commit]) ?? null;
  }

  getAllAnomalies(): AnomalyMap {
    return this.anomaly;
  }

  /**
   * Update anomaly map.
   *
   * @param anomalies The list of anomaly to be merged.
   * @param id List of original revisions to be compared against.
   */
  updateAnomalies(anomalies: AnomalyMap, id: string) {
    this.anomaly = removeAnomaly(this.anomaly, id);
    this.anomaly = mergeAnomaly(this.anomaly, anomalies);
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
    request: FrameRequest,
    replaceAnomalies: boolean = false
  ) {
    this._baseRequest = request;
    this._baseRequest.request_type = 0; // change to timestamp-based query.

    let newAnomalyMap: AnomalyMap;
    if (replaceAnomalies) {
      newAnomalyMap = mergeAnomaly(null, anomalies);
    } else {
      newAnomalyMap = mergeAnomaly(this.anomaly, anomalies);
    }
    this._header = dataframe.header || [];
    this._traceset = dataframe.traceset;
    this._traceMetadata = dataframe.traceMetadata;

    await this.setDataFrame(dataframe);
    this.anomaly = newAnomalyMap;
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
      resolver = resolve;
    });

    await curRequest;

    this.loading = true;
    this._paramset = param;
    const resp = await this.requestNewRange(range);
    this._traceset = resp.dataframe?.traceset || TraceSet({});
    this._header = resp.dataframe?.header || [];
    this._traceMetadata = resp.dataframe?.traceMetadata || [];

    const totalTraces = resp.dataframe?.header?.length || 0;
    await this.setDataFrame({
      traceset: this._traceset,
      header: this._header,
      paramset: this._paramset,
      skip: 0,
      traceMetadata: this._traceMetadata,
    });

    this.anomaly = mergeAnomaly(this.anomaly, resp.anomalymap);
    this.loading = false;
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
    if (offsetInSeconds === this.chunkSize) {
      return;
    }
    let totalTraces = 0;
    let resolver = emptyResolver;
    const curRequest = this._requestComplete;
    this._requestComplete = new Promise((resolve) => {
      resolver = resolve;
    });
    try {
      await curRequest;
      this.loading = true;
      const range = deltaRange(this.timeRange, offsetInSeconds);
      const ranges = sliceRange(range, this.chunkSize);
      const allResponses: Promise<FrameResponse>[] = [];
      for (let slice = 0; slice < ranges.length; slice++) {
        allResponses.push(this.requestNewRange(ranges[slice]));
      }

      // Fetch and sort the frame responses so they can appended consecutively.
      const sortedResponses = (await Promise.allSettled(allResponses))
        .filter(
          (result): result is PromiseFulfilledResult<FrameResponse> => result.status === 'fulfilled'
        )
        .map((result) => result.value)
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
      totalTraces = sortedResponses
        .map((resp) => resp.dataframe!.header?.length || 0)
        .reduce((count, cur) => count + cur, 0);

      const header = ([] as (ColumnHeader | null)[]).concat(
        ...sortedResponses.map((resp) => resp.dataframe!.header)
      );

      const traceset = TraceSet({});
      Object.keys(this.traces).forEach((key) => {
        traceset[key] = (traceset[key] || []).concat(
          ...sortedResponses.map((resp) => {
            if (resp.dataframe && resp.dataframe.traceset[key]) {
              return resp.dataframe.traceset[key];
            } else {
              // If one of the traces we're trying to expand is not in the response,
              // this will cause the traceset to not have the same length as the
              // header, shifting all datapoints. Instead, we need to pad the trace
              // with MISSING values so that they're in sync with the header.
              const numNulls = resp.dataframe?.header?.length ?? 0;
              return Array(numNulls).fill(MISSING_DATA_SENTINEL);
            }
          })
        ) as Trace;
      });

      const traceMetadata: TraceMetadata[] = [];
      sortedResponses.map((resp) => {
        if (resp.dataframe && resp.dataframe.traceMetadata) {
          traceMetadata.push(...resp.dataframe.traceMetadata);
        }
      });
      this.addTraceInfo(header, traceset, traceMetadata);

      const anomaly = sortedResponses.reduce((pre, cur) => mergeAnomaly(pre, cur.anomalymap), {});
      await this.setDataFrame({
        traceset: this._traceset,
        header: this._header,
        paramset: this._paramset,
        skip: 0,
        traceMetadata: this._traceMetadata,
      });

      this.anomaly = mergeAnomaly(this.anomaly, anomaly);
    } finally {
      this.loading = false;
      // This is needed to ensure that the resolver is called even if the
      // request fails, otherwise the caller can get stuck.
      resolver(totalTraces);
      return totalTraces;
    }
  }

  protected async shift(commitRange: range, offset: number = -200) {
    const req = deltaRange(commitRange, offset) as ShiftRequest;
    return await DataService.getInstance().shift(req);
  }

  // Makes an API call to fetch the comments in the given commit position range.
  // Modifies the response to match with the UserIssueMap interface.
  async getUserIssues(traceKeys: string[], begin: number, end: number): Promise<UserIssueMap> {
    // If trace key has special functions like norm(,a=A,b=B,), remove
    // those functions from the traceKey. This is done to make sure
    // the user issue is reflected for the normalized, etc transformations of
    // the graph.
    const modifiedTraceKeys = traceKeys
      .map((k) => formatSpecialFunctions(k))
      .map((trace) => {
        return ',' + trace + ',';
      });

    const req: GetUserIssuesForTraceKeysRequest = {
      trace_keys: modifiedTraceKeys,
      begin_commit_position: begin,
      end_commit_position: end,
    };
    const resp = await DataService.getInstance().getUserIssues(req);

    const output: UserIssueMap = {};
    if (resp.UserIssues) {
      resp.UserIssues.forEach((issue) => {
        const traceKey = issue.TraceKey;
        const commitPos = issue.CommitPosition;
        const issueId = issue.IssueId;
        if (!(traceKey in output)) {
          output[traceKey] = {};
        }
        output[traceKey][commitPos] = { bugId: issueId, x: -1, y: -1 };
      });
    }

    this.userIssues = output;
    return output;
  }

  // Updates the userIssues property if a new buganizer issue is created
  // or an existing issue is removed.
  updateUserIssue(traceKey: string, commitPosition: number, bugId: number) {
    let issues = this.userIssues;
    this.userIssues = null;

    // If trace key has special functions like norm(,a=A,b=B,), remove
    // those functions from the traceKey. This is done to make sure
    // the user issue is reflected for the normalized, etc transformations of
    // the graph.
    const modifiedTraceKey_preTempFix = formatSpecialFunctions(traceKey);
    // TODO(b/469649488) remove leading / trailing commas from the DB
    const modifiedTraceKey = ',' + modifiedTraceKey_preTempFix + ',';

    const updatedIssue = { bugId: bugId, x: -1, y: -1 };
    if (issues === null) {
      issues = {};
      issues[modifiedTraceKey] = {};
      issues[modifiedTraceKey][commitPosition] = updatedIssue;
      this.userIssues = issues;
      return;
    }

    if (Object.keys(issues).includes(modifiedTraceKey)) {
      issues[modifiedTraceKey][commitPosition] = updatedIssue;
      this.userIssues = issues;
    } else {
      issues[modifiedTraceKey] = {};
      issues[modifiedTraceKey][commitPosition] = updatedIssue;
      this.userIssues = issues;
    }
  }

  /**
   * Clears all the data in the repository.
   */
  clear() {
    this._traceset = TraceSet({});
    this._header = [];
    this._traceMetadata = [];
    this.dataframe = {
      traceset: TraceSet({}),
      header: [],
      paramset: ReadOnlyParamSet({}),
      skip: 0,
      traceMetadata: [],
    };
    this.data = null;
    this.anomaly = null;
    this.userIssues = null;
  }

  /**
   * Clears just the anomaly map.
   */
  clearAnomalyMap() {
    this.anomaly = null;
  }
}
