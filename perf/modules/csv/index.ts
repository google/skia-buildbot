// Functions for generating CSV from a DataFrame.
import { ColumnHeader, DataFrame, Params } from '../json';
import { fromKey } from '../paramtools';

export function parseIdsIntoParams(ids: string[]): {[key: string]: Params} {
  const ret: {[key: string]: Params} = {};

  ids.forEach((id: string) => {
    ret[id] = fromKey(id);
  });

  return ret;
}

export function allParamKeysSorted(allParams: {[key: string]: Params}): string[] {
  const paramKeys: Set<string> = new Set();

  Object.keys(allParams).forEach((key) => {
    Object.keys(allParams[key]).forEach((pkey) => paramKeys.add(pkey));
  });

  return Array.from<string>(paramKeys).sort();
}

export function dataFrameToCSV(df: DataFrame): string {
  const csv: string[] = [];
  // First figure out how many columns we need to represent all the ids in the
  // DataFrame, then add all those column names.
  const traceIDs = Object.keys(df.traceset);
  const traceIDToParams = parseIdsIntoParams(traceIDs);
  const sortedColumnNames = allParamKeysSorted(traceIDToParams);

  let line: (string | number)[] = sortedColumnNames.slice(0);
  df.header!.forEach((ch: ColumnHeader|null) => {
    line.push(new Date(ch!.timestamp * 1000).toISOString());
  });
  csv.push(line.join(','));
  Object.keys(df.traceset).forEach((traceId) => {
    if (traceId.startsWith('special_')) {
      return;
    }
    // Create the first columns of the CSV row which contain all the Param
    // values for each column header, noting that not all columns will have a
    // value.
    const traceParams = traceIDToParams[traceId];
    line = sortedColumnNames.map((columnName) => traceParams[columnName] || '');
    df.traceset[traceId]!.forEach((f) => {
      if (!Number.isNaN(f)) {
        line.push(f);
      } else {
        line.push('');
      }
    });
    csv.push(line.join(','));
  });

  return csv.join('\n');
}
