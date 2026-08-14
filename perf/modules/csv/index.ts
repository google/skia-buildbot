// Functions for generating CSV from a DataFrame.
import { MISSING_DATA_SENTINEL } from '../const/const';
import { DataFrame } from '../json';
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
