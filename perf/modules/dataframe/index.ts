// Contains DataFrame merge logic, similar to //perf/go/dataframe/dataframe.go

import {
  DataFrame,
  ParamSet,
  Params,
  ColumnHeader,
  TraceSet,
  ReadOnlyParamSet,
  Trace,
} from '../json';
import { addParamSet, addParamsToParamSet, fromKey } from '../paramtools';
import { MISSING_DATA_SENTINEL } from '../const/const';

/** mergeColumnHeaders creates a merged header from the two given headers.
 *
 * I.e. {1,4,5} + {3,4} => {1,3,4,5}
 */
export function mergeColumnHeaders(
  a: (ColumnHeader | null)[] | null,
  b: (ColumnHeader | null)[] | null
): [
  (ColumnHeader | null)[] | null,
  { [key: number]: number },
  { [key: number]: number }
] {
  if (a === null || a.length == 0) {
    return [b, simpleMap(0), simpleMap(b!.length)];
  } else if (b === null || b.length == 0) {
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
    if (pA == numA && pB == numB) {
      break;
    }
    if (pA == numA) {
      // Copy in the rest of B.
      for (let i = 0; i < numB; i++) {
        bMap[i] = ret.length;
        ret.push(b[i]);
      }
      break;
    }
    if (pB == numB) {
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
  } as DataFrame;
  if (a.header!.length == 0) {
    a.header = b.header;
  }
  ret.skip = b.skip;

  const ps: ParamSet = {};

  addParamSet(ps, a.paramset);
  addParamSet(ps, b.paramset);

  normalize(ps);
  ret.paramset = ps as ReadOnlyParamSet;

  const traceLen = ret.header!.length;

  for (const [key, sourceTrace] of Object.entries(a.traceset)) {
    if (!ret.traceset[key]) {
      ret.traceset[key] = new Array(traceLen).fill(MISSING_DATA_SENTINEL);
    }
    const destTrace = ret.traceset[key];
    sourceTrace.forEach((sourceValue, sourceOffset) => {
      destTrace[aMap[sourceOffset]] = sourceValue;
    });
  }

  for (const [key, sourceTrace] of Object.entries(b.traceset)) {
    if (!ret.traceset[key]) {
      ret.traceset[key] = new Array(traceLen).fill(MISSING_DATA_SENTINEL);
    }
    const destTrace = ret.traceset[key];
    sourceTrace.forEach((sourceValue, sourceOffset) => {
      destTrace[bMap[sourceOffset]] = sourceValue;
    });
  }

  return ret;
}

/** buildParamSet rebuilds d.paramset from the keys of d.traceset. */
export function buildParamSet(d: DataFrame): void {
  const paramSet: ParamSet = {};
  for (const key of Object.keys(d.traceset)) {
    const params = fromKey(key);
    addParamsToParamSet(paramSet, params);
  }
  normalize(paramSet);
  d.paramset = paramSet as ReadOnlyParamSet;
}

/** timestampBounds returns the timestamps for the first and last header values.
 * If df is null, or its header value is null or of length 0, this will return
 * [NaN, NaN].
 */
export function timestampBounds(df: DataFrame | null): [number, number] {
  if (df === null || df.header === null || df.header.length == 0) {
    return [NaN, NaN];
  }
  let ret: [number, number] = [NaN, NaN];
  ret[0] = df.header![0]!.timestamp;
  ret[1] = df.header![df.header!.length - 1]!.timestamp;
  return ret;
}

function normalize(ps: ParamSet): void {
  for (const [k, v] of Object.entries(ps)) {
    v.sort();
  }
}

function simpleMap(n: number): { [key: number]: number } {
  const ret: { [key: number]: number } = {};

  for (let i = 0; i < n; i += 1) {
    ret[i] = i;
  }
  return ret;
}
