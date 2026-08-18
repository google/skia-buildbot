// Functions for generating CSV from a DataFrame.
import { MISSING_DATA_SENTINEL } from '../const/const';
import { DataFrame, Trace, TraceSet } from '../json';
import { fromKey } from '../paramtools';
export { downloadCSV } from './download';

export function escapeCSV(val: string): string {
  if (val.includes(',') || val.includes('"') || val.includes('\n') || val.includes('\r')) {
    return `"${val.replace(/"/g, '""')}"`;
  }
  return val;
}

export function formatTraceIdAsQuery(traceId: string): string {
  const params = fromKey(traceId);
  const keys = Object.keys(params).sort();
  if (keys.length === 0) {
    return traceId;
  }
  return keys.map((k) => `${k}=${params[k]}`).join('&');
}

export interface TraceRowLike {
  commit_number: number;
  val: number;
  hash?: string;
  [key: string]: any;
}

export interface TraceSeriesLike {
  id: string;
  rows: TraceRowLike[];
  hidden?: boolean;
  [key: string]: any;
}

/**
 * removeSpecialTraces removes traces whose IDs start with 'special_' from either
 * a DataFrame or an array of TraceSeriesLike.
 */
export function removeSpecialTraces(df: DataFrame): DataFrame;
export function removeSpecialTraces(series: TraceSeriesLike[]): TraceSeriesLike[];
export function removeSpecialTraces(
  data: DataFrame | TraceSeriesLike[]
): DataFrame | TraceSeriesLike[] {
  if (!data) {
    return data;
  }
  if (Array.isArray(data)) {
    return data.filter((s) => !s.id?.startsWith('special_'));
  }
  if (data.traceset) {
    const filteredMap: { [key: string]: Trace } = {};
    for (const [key, val] of Object.entries(data.traceset)) {
      if (!key.startsWith('special_') && key !== '_traceSetBrand') {
        filteredMap[key] = val as Trace;
      }
    }
    return {
      ...data,
      traceset: TraceSet(filteredMap),
    };
  }
  return data;
}

/**
 * dataframeToCSV converts DataFrame trace values into a CSV where:
 * - The first two columns are 'offset' (commit number) and 'hash' (git hash).
 * - Each subsequent column corresponds to a trace (named by its query or 'value' if single trace).
 * - Each row corresponds to a commit revision with values for all displayed traces.
 */
export function dataframeToCSV(df: DataFrame): string {
  if (!df || !df.header || !df.traceset) {
    return '';
  }

  const traceIds = Object.keys(df.traceset);

  if (traceIds.length === 0) {
    return '';
  }
  // Build CSV Header
  const headerRow: string[] = ['offset', 'hash'];
  for (const id of traceIds) {
    const formatted = formatTraceIdAsQuery(id);
    headerRow.push(escapeCSV(formatted));
  }

  const csv: string[] = [headerRow.join(',')];

  // Build CSV Rows per commit
  const numCommits = df.header.length;
  for (let i = 0; i < numCommits; i++) {
    const colHeader = df.header[i];
    if (!colHeader) {
      continue;
    }

    const offsetStr =
      colHeader.offset !== undefined && colHeader.offset !== null ? `${colHeader.offset}` : '';
    const hashStr = colHeader.hash || '';

    const rowValues: string[] = [escapeCSV(offsetStr), escapeCSV(hashStr)];
    for (const id of traceIds) {
      const trace = df.traceset[id];
      const val = trace ? trace[i] : undefined;
      if (val === undefined || val === MISSING_DATA_SENTINEL || isNaN(val) || val === null) {
        rowValues.push('');
      } else {
        rowValues.push(`${val}`);
      }
    }

    csv.push(rowValues.join(','));
  }

  return csv.join('\n');
}

/**
 * traceSeriesToCSV converts an array of TraceSeries into a CSV where:
 * - The first two columns are 'offset' (commit number) and 'hash' (git hash).
 * - Each subsequent column corresponds to a trace series (named by its query).
 * - Each row corresponds to a commit revision with values for all displayed series.
 */
export function traceSeriesToCSV(series: TraceSeriesLike[]): string {
  if (!series || series.length === 0) {
    return '';
  }

  const activeSeries = series.filter((s) => !s.hidden && s.rows && s.rows.length > 0);

  if (activeSeries.length === 0) {
    return '';
  }

  // Collect all unique commit_numbers and any associated hash across all series.
  const commitMap = new Map<number, string>();
  for (const s of activeSeries) {
    for (const row of s.rows) {
      if (row.commit_number !== undefined && row.commit_number !== null) {
        if (!commitMap.has(row.commit_number) || (!commitMap.get(row.commit_number) && row.hash)) {
          commitMap.set(row.commit_number, row.hash || '');
        }
      }
    }
  }

  const sortedCommits = Array.from(commitMap.keys()).sort((a, b) => a - b);
  if (sortedCommits.length === 0) {
    return '';
  }

  // Pre-index series rows by commit_number for fast lookup
  const seriesRowMaps = activeSeries.map((s) => {
    const map = new Map<number, number>();
    for (const row of s.rows) {
      map.set(row.commit_number, row.val);
    }
    return map;
  });

  // Header row
  const headerRow: string[] = ['offset', 'hash'];
  for (const s of activeSeries) {
    headerRow.push(escapeCSV(formatTraceIdAsQuery(s.id)));
  }

  const csv: string[] = [headerRow.join(',')];

  // Data rows
  for (const commit of sortedCommits) {
    const hash = commitMap.get(commit) || '';
    const rowValues: string[] = [escapeCSV(`${commit}`), escapeCSV(hash)];

    for (let i = 0; i < activeSeries.length; i++) {
      const val = seriesRowMaps[i].get(commit);
      if (val === undefined || val === MISSING_DATA_SENTINEL || isNaN(val) || val === null) {
        rowValues.push('');
      } else {
        rowValues.push(`${val}`);
      }
    }

    csv.push(rowValues.join(','));
  }

  return csv.join('\n');
}
