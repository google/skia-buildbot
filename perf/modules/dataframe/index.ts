// Contains DataFrame merge logic, similar to //perf/go/dataframe/dataframe.go

import {
  DataFrame,
  ParamSet,
  ColumnHeader,
  TraceSet,
  Trace,
  AnomalyMap,
  CommitNumberAnomalyMap,
  ReadOnlyParamSet,
} from '../json';
import { addParamSet, addParamsToParamSet, fromKey, toReadOnlyParamSet } from '../paramtools';
import { MISSING_DATA_SENTINEL } from '../const/const';

export type { ColumnHeader };

// Simple type denoting the begin and end of the range.
export type range = { begin: number; end: number };

// Helper function to create range shortcut.
export const range = (begin: number, end: number) => ({ begin, end }) as range;

/**
 * Find the subrange in the header.
 * @param header The subfield of DataFrame, containing the time stamps.
 * @param range The range to search for within the given header.
 * @param domain Whether timestamp or offset.
 * @returns The range [inclusive start, exclusive end) matching the given range
 *  in the given header.
 *  [0, 0) is returned when the range is before the header and,
 *  [length, length) is returned when the range is beyond the header.
 *
 * @example
 *  header = [0,10,20,30,40], [10,20] ==> [1, 3) because header[3] is the first
 *  element that is larger than 20. And it contains two elements.
 *  header = [0,10,20,30,40], [10,10] ==> [1, 2) because header[2] is the first
 *  element that is larger than 10. And it contains one element.
 */
export const findSubDataframe = (
  header: (ColumnHeader | null)[],
  range: range,
  domain: 'timestamp' | 'offset' | 'commit' | 'date' = 'timestamp'
): range => {
  const prop = domain === 'date' || domain === 'timestamp' ? 'timestamp' : 'offset';
  const begin = header.findIndex((v) => {
    return range.begin <= v![prop];
  });
  const end = header.findIndex((v) => {
    return range.end < v![prop];
  });
  return {
    begin: begin < 0 ? header.length : begin,
    end: end < 0 ? header.length : end,
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
export const generateSubDataframe = (dataframe: DataFrame, range: range): DataFrame => {
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
    // Maybe we can trim this to only the commits in the range.
    traceMetadata: dataframe.traceMetadata,
  };
};

/**
 * Merge two AnomalyMap and return a new AnomalyMap.
 *
 * This function always returns a non-nil AnomalyMap.
 * @param anomaly1 The first anomaly.
 * @param anomalies The list of anomaly to be merged.
 * @returns The new AnomalyMap.
 */
export const mergeAnomaly = (anomaly1: AnomalyMap, ...anomalies: AnomalyMap[]) => {
  const anomaly = structuredClone(anomaly1 || {});
  if (anomalies.length <= 0) {
    return structuredClone(anomaly1 || {});
  }

  /**
   * {
   *  ',trace=key1': {
   *    commit_position1: Anomaly 1,
   *    commit_position2: Anomaly 2,
   *    ...
   *  },
   *  `,trace=key2': {...}
   * }
   */
  anomalies.forEach((anomaly2) => {
    for (const trace in anomaly2) {
      // Merge the anomaly from anomaly2 for the trace.
      // First we use the existing merged anomaly as the base, and then add
      // or update the anomaly data at each commit.
      const commitAnomaly = anomaly[trace] || {};
      const commitAnomaly2 = anomaly2[trace];

      // In each trace, the anomaly is in the sparsed array, so it's more
      // efficient to iterater using keys.
      for (const commit in commitAnomaly2) {
        const commitNum = Number(commit);

        // The anomaly at commitNum will either be overridden and added to the
        // anomaly for the current trace.
        commitAnomaly[commitNum] = commitAnomaly2![commitNum];
      }

      // Override with the updated anomaly for the trace.
      anomaly[trace] = commitAnomaly;
    }
  });
  return anomaly;
};

/**
 * Removes reference to existing anomalies that have been moved/nudged.
 *  First remove the existing anomaly that was moved, then merge the new
 *  location into the anomaly map. This should match what is on the backend.
 *
 * @param anomalies The list of anomaly to check against.
 * @param id Anomaly ID to filter out.
 * @returns The new AnomalyMap.
 */
export const removeAnomaly = (anomalies: AnomalyMap, id: string): AnomalyMap => {
  const anomaly: AnomalyMap = {};
  for (const trace in anomalies) {
    const commitAnomaly: CommitNumberAnomalyMap = {};
    const traceAnomaly = anomalies![trace];
    for (const commit in traceAnomaly) {
      const commitNum = Number(commit);
      // Check that the nudged commit number is found, then skip adding back in.
      if (traceAnomaly[commitNum].id !== id) {
        commitAnomaly[commitNum] = traceAnomaly[commitNum];
      }
    }
    if (Object.keys(commitAnomaly).length > 0) {
      anomaly[trace] = commitAnomaly;
    }
  }
  return anomaly;
};

export const findAnomalyInRange = (allAnomaly: AnomalyMap, range: range): AnomalyMap => {
  const anomaly: AnomalyMap = {};
  for (const trace in allAnomaly) {
    const commitAnomaly: CommitNumberAnomalyMap = {};
    const traceAnomaly = allAnomaly![trace];
    for (const commit in traceAnomaly) {
      const commitNum = Number(commit);
      if (commitNum >= range.begin && commitNum <= range.end) {
        commitAnomaly[commitNum] = traceAnomaly[commitNum];
      }
    }

    if (Object.keys(commitAnomaly).length > 0) {
      anomaly[trace] = commitAnomaly;
    }
  }
  return anomaly;
};

/** mergeColumnHeaders creates a merged header from the two given headers.
 *
 * I.e. {1,4,5} + {3,4} => {1,3,4,5}
 */
export function mergeColumnHeaders(
  a: (ColumnHeader | null)[] | null,
  b: (ColumnHeader | null)[] | null
): [(ColumnHeader | null)[] | null, { [key: number]: number }, { [key: number]: number }] {
  if (a === null || a.length === 0) {
    return [b, simpleMap(0), simpleMap(b!.length)];
  }
  if (b === null || b.length === 0) {
    return [a, simpleMap(a!.length), simpleMap(0)];
  }
  const aMap: { [key: number]: number } = {};
  const bMap: { [key: number]: number } = {};
  const numA = a.length;
  const numB = b.length;
  let pA = 0;
  let pB = 0;
  const ret: (ColumnHeader | null)[] = [];
  for (; true; ) {
    if (pA === numA && pB === numB) {
      break;
    }
    if (pA === numA) {
      // Copy in the rest of B.
      for (let i = 0; i < numB; i++) {
        bMap[i] = ret.length;
        ret.push(b[i]);
      }
      break;
    }
    if (pB === numB) {
      // Copy in the rest of A.
      for (let i = pA; i < numA; i++) {
        aMap[i] = ret.length;
        ret.push(a[i]);
      }
      break;
    }

    if (a[pA]!.offset < b[pB]!.offset) {
      aMap[pA] = ret.length;
      ret.push(a[pA]);
      pA += 1;
    } else if (a[pA]!.offset > b[pB]!.offset) {
      bMap[pB] = ret.length;
      ret.push(b[pB]);
      pB += 1;
    } else {
      aMap[pA] = ret.length;
      bMap[pB] = ret.length;
      ret.push(a[pA]);
      pA += 1;
      pB += 1;
    }
  }
  return [ret, aMap, bMap];
}

/** join creates a new DataFrame that is the union of 'a' and 'b'.
 *
 * Will handle the case of a and b having data for different sets of commits,
 * i.e. a.Header doesn't have to equal b.Header.
 */
export function join(a: DataFrame, b: DataFrame): DataFrame {
  if (a === null) {
    return b;
  }
  const [header, aMap, bMap] = mergeColumnHeaders(a.header, b.header);
  const ret: DataFrame = {
    header: header,
    traceset: {} as TraceSet,
    traceMetadata: null,
  } as DataFrame;
  if (a.header!.length === 0) {
    a.header = b.header;
  }
  ret.skip = b.skip;

  const ps = ParamSet({});

  addParamSet(ps, a.paramset);
  addParamSet(ps, b.paramset);

  normalize(ps);
  ret.paramset = toReadOnlyParamSet(ps);

  const traceLen = ret.header!.length;

  // Pre-allocate the arrays in the traceset with the correct length.
  for (const key of Object.keys(a.traceset)) {
    if (!ret.traceset[key]) {
      ret.traceset[key] = new Array(traceLen).fill(MISSING_DATA_SENTINEL) as Trace;
    }
  }
  for (const key of Object.keys(b.traceset)) {
    if (!ret.traceset[key]) {
      ret.traceset[key] = new Array(traceLen).fill(MISSING_DATA_SENTINEL) as Trace;
    }
  }

  for (const [key, sourceTrace] of Object.entries(a.traceset)) {
    const destTrace = ret.traceset[key];
    (sourceTrace as number[]).forEach((sourceValue, sourceOffset) => {
      destTrace[aMap[sourceOffset]] = sourceValue;
    });
  }

  for (const [key, sourceTrace] of Object.entries(b.traceset)) {
    const destTrace = ret.traceset[key];
    (sourceTrace as number[]).forEach((sourceValue, sourceOffset) => {
      destTrace[bMap[sourceOffset]] = sourceValue;
    });
  }

  ret.traceMetadata = [...(a.traceMetadata || []), ...(b.traceMetadata || [])];

  return ret;
}

/** buildParamSet rebuilds d.paramset from the keys of d.traceset. */
export function buildParamSet(d: DataFrame): void {
  const paramSet = ParamSet({});
  for (const key of Object.keys(d.traceset)) {
    const params = fromKey(key);
    addParamsToParamSet(paramSet, params);
  }
  normalize(paramSet);
  d.paramset = toReadOnlyParamSet(paramSet);
}

/** timestampBounds returns the timestamps for the first and last header values.
 * If df is null, or its header value is null or of length 0, this will return
 * [NaN, NaN].
 */
export function timestampBounds(df: DataFrame | null): [number, number] {
  if (df === null || df.header === null || df.header.length === 0) {
    return [NaN, NaN];
  }
  const ret: [number, number] = [NaN, NaN];
  ret[0] = df.header![0]!.timestamp;
  ret[1] = df.header![df.header!.length - 1]!.timestamp;
  return ret;
}

function normalize(ps: ParamSet): void {
  for (const [_, v] of Object.entries(ps)) {
    (v as string[]).sort();
  }
}

function simpleMap(n: number): { [key: number]: number } {
  const ret: { [key: number]: number } = {};

  for (let i = 0; i < n; i += 1) {
    ret[i] = i;
  }
  return ret;
}
