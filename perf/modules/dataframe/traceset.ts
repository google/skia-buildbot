import '@google-web-components/google-chart';

import { DataFrame } from '../json';
import { DataTable } from './dataframe_context';
import { formatSpecialFunctions } from '../paramtools';

export const labelKeys = [
  'master',
  'bot',
  'benchmark',
  'improvement_direction',
  'stat',
  'unit',
  'test',
  'subtest_1',
  'subtest_2',
  'subtest_3',
  'subtest_4',
  'subtest_5',
  'subtest_6',
];
/**
 * getAttributes extracts the attributes from the traceKeys
 * The attributes are the keys to the trace.
 * i.e. benchmark, test, story, subtest.
 *
 * The dataframe can have inconsistent attributes from one
 * trace to the other. This function gets all unique attributes
 * across every trace.
 */
// TODO(b/362831653): Create or modify this function to match
// the functionality of fromKey(): Params.
export const getAttributes = (df: DataFrame): string[] => {
  const traceKeys = Object.keys(df.traceset);
  const pairs = convertKeysToPairs(traceKeys);
  const attributes = pairs.map((p) => p.split('=')[0]);
  return Array.from(new Set(attributes));
};

/**
 * getTitle converts the traceKeys into the title.
 * The title is all common traceKey key-value pairs
 * @param df The dataframe
 * @returns title as a hashmap with (key, value) = (traceKey, attribute)
 *
 * @example
 * a df with keys = [
 *   ",benchmark=JetStream2,story=Total,test=avg",
 *   ",benchmark=JetStream2,story=Air,test=std"
 * ]
 * title = {
 *  "benchmark": "JetStream2"
 * }
 */
// One option with getTitle and getLegend is to create separate versions
// for DataTable and DataFrame. They would use the same logic. The only
// difference is getting the traceKeys:
// const traceKeys = Object.keys(df!.traceset);
// Just depends on whether explore-simple-sk needs to invoke this logic
export const getTitle = (dt: DataTable): object => {
  const numCols = dt!.getNumberOfColumns();
  const traceKeys: string[] = [];
  // skip the first two columns since they are domains
  for (let i = 2; i < numCols; i++) {
    traceKeys.push(dt!.getColumnLabel(i));
  }
  const pairs = convertKeysToPairs(traceKeys);

  // filters key-value pairs that appear across all keys
  // assumes that key-value pairs cannot appear more than once
  // within the same traceKey
  const commonPairs = pairs.filter((pair, _, arr) => {
    const count = arr.filter((x) => x === pair).length;
    return count === traceKeys.length;
  });

  return Object.fromEntries(
    [...new Set(commonPairs)].map((value) => {
      return value.split('=');
    })
  );
};

/**
 * getLegend converts the traceKeys into the legend.
 * The legend is any element that is not common across all traceKeys
 * @param df The dataframe
 * @returns legend as a hashmap array with (key, value) = (traceKey, attribute)
 *  Each entry belongs to the corresponding trace
 *
 * @example
 * a df with keys = [
 *   ",benchmark=JetStream2,story=Total,test=avg",
 *   ",benchmark=JetStream2,story=Air,test=std"
 *   ",benchmark=JetStream2,story=Air"
 * ]
 * legend = [{
 *   "story":"Total",
 *   "test":"avg",
 *  }, {
 *   "story":"Air",
 *   "test":"std",
 *  }, {
 *   "story":"Air",
 *   "test":"untitled_key",
 *  }]
 */
export const getLegend = (dt: DataTable): object[] => {
  const numCols = dt!.getNumberOfColumns();
  const traceKeys = [];
  // skip the first two columns since they are domains
  for (let i = 2; i < numCols; i++) {
    const k = dt!.getColumnLabel(i);
    const formattedKey = formatSpecialFunctions(k);
    // There are special traces with no data, only "special_zero" as the trace name.
    // They do not show any data on the graph, so we remove them.
    if (formattedKey !== 'special_zero') {
      traceKeys.push(formattedKey);
    }
  }
  const pairs = convertKeysToPairs(traceKeys);

  const uniqKVP = [];
  // for each key, split the traceKey into key-value pairs
  // and filter for entries that do not appear across all traceKeys

  if (traceKeys.length >= 1) {
    for (const key of traceKeys) {
      const kvp = key.split(',').filter((pair) => pair.length);
      const uniq = kvp.filter((item) => pairs.filter((x) => x === item).length < traceKeys.length);
      uniqKVP.push(
        Object.fromEntries(
          uniq.map((item) => {
            return item.split('=');
          })
        )
      );
    }
  }

  // Get all unique keys from the objects in the array
  const allKeys = new Set(uniqKVP.flatMap(Object.keys));

  // Fill in missing or blank keys in the map with empty string
  const legend = uniqKVP.map((obj) => {
    const newObj: { [key: string]: string } = {};
    Array.from(allKeys)
      .sort() // Sort the keys alphabetically
      .forEach((key) => {
        newObj[key] = obj[key] ? obj[key] : 'Default';
      });
    return newObj;
  });

  return legend;
};

/**
 * converts traceKeys to a list of individual key-value pairs and
 * removing any trailing spaces and commas. If any of the trace keys
 * are special function keys like norm(,a=A,b=B,c=C,), extract out
 * the actual key ,a=A,b=B,c=C, from the trace name.
 */
function convertKeysToPairs(traceKeys: string[]) {
  return traceKeys
    .map((key) => {
      const formattedKey = formatSpecialFunctions(key);
      return formattedKey.split(',').filter((pair) => pair.length);
    })
    .flat();
}

/**
 * titleFormatter converts the getTitle dictionary into a string
 * @param df The output of getTitle()
 * @returns Title with extra attributes omitted
 *
 * @example
 * title = {
 *  "benchmark": "JetStream2",
 *  "story": "Box2D",
 * }
 * returns "JetStream2/Box2D"
 */
export function titleFormatter(title: object): string {
  return Object.values(title).join('/');
}

/**
 * legendFormatter converts the getLegend dictionary into a string
 * @param df The output of getLegend()
 * @returns Legend with extra attributes omitted
 *
 * @example
 * legend = [{
 *   "story":"Total",
 *   "test":"avg"
 *  }, {
 *   "story":"Air"
 *   "test":"std"
 *  }]
 * returns ["Total/avg", "Air/std"]
 */
export function legendFormatter(legend: object[]): string[] {
  return legend.map((entry) =>
    Object.values(entry)
      .filter((value) => value)
      .join('/')
  );
}

/**
 * getLegendKeysTitle converts a label from Google chart into a string that combines all legends key
 * @returns Title with extra keys omitted
 *
 * @example
 * legend = {
 *  "subtest_1": "fencedframe",
 *  "subtest_2": "PageLoad.Clients",
 * }
 * returns "subtest_1/subtest_2"
 */
export function getLegendKeysTitle(label: object): string {
  return Object.keys(label).join('/');
}

/**
 * isSingleTrace identifies if there is only one trace in the Dataframe
 * @param df: Dataframe or undefined dataframe
 * @returns: null if undefined dataframe. Otherwise true/false.
 */
export function isSingleTrace(dt: DataTable | undefined): boolean | null {
  if (!dt) {
    return null;
  }
  // first two cols are domains (commit position / date)
  return dt!.getNumberOfColumns() === 3;
}

/**
 * updateTraceByLegend function identifies the trace's last value
 * and sets the visibility of the specified property
 * based on the checkbox's checked state
 * @param dt: DataTable or undefined dataframe
 * @param legendTraceId: legend value from side panel
 * @param isChecked: checkbox checked state
 */
export function findTraceByLabel(dt: DataTable | undefined, legendTraceId: string): string | null {
  if (!dt) {
    return null;
  }
  const numCols = dt!.getNumberOfColumns();
  // skip the first two columns since they are domains (commit position / date)
  for (let i = 2; i < numCols; i++) {
    const label = dt!.getColumnLabel(i);
    if (legendTraceId === label) {
      return label;
    }
  }
  return null;
}

/**
 * Finds traces that contain the given key values param pair.
 * @param dt DataTable or undefined dataframe
 * @param paramKey Param key
 * @param paramValues Param values
 * @returns List of trace labels containing the key value pair.
 */
export function findTracesForParam(
  dt: DataTable | undefined,
  paramKey: string,
  paramValues: string[]
): string[] | null {
  if (!dt) {
    return null;
  }
  // A matching trace will contain the key value pair in the form 'key=value'
  const expectedLabels: string[] = [];
  paramValues.forEach((paramValue) => {
    const expectedLabelContent = ',' + paramKey + '=' + paramValue + ',';
    expectedLabels.push(expectedLabelContent);
  });

  const numCols = dt!.getNumberOfColumns();
  const traces: string[] = [];
  // skip the first two columns since they are domains (commit position / date)
  for (let i = 2; i < numCols; i++) {
    const label = dt!.getColumnLabel(i);
    expectedLabels.forEach((expectedLabelContent) => {
      if (label.includes(expectedLabelContent)) {
        traces.push(label);
      }
    });
  }
  return traces;
}
