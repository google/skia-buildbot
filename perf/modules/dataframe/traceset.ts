import '@google-web-components/google-chart';

import { DataFrame } from '../json';
import { DataTable } from './dataframe_context';
import { removeSpecialFunctions } from '../paramtools';

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
  const traceKeys = [];
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
 *   "test":"-",
 *  }]
 */
export const getLegend = (dt: DataTable): object[] => {
  const numCols = dt!.getNumberOfColumns();
  const traceKeys = [];
  // skip the first two columns since they are domains
  for (let i = 2; i < numCols; i++) {
    const k = dt!.getColumnLabel(i);
    const formattedKey = removeSpecialFunctions(k);
    traceKeys.push(formattedKey);
  }
  const pairs = convertKeysToPairs(traceKeys);

  const uniqKVP = [];
  // for each key, split the traceKey into key-value pairs
  // and filter for entries that do not appear across all traceKeys
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

  // Get all unique keys from the objects in the array
  const allKeys = new Set(uniqKVP.flatMap(Object.keys));

  // Fill in missing or blank keys with "-"
  const legend = uniqKVP.map((obj) => {
    const newObj: { [key: string]: string } = {};
    Array.from(allKeys)
      .sort() // Sort the keys alphabetically
      .forEach((key) => {
        newObj[key] = obj[key] ? obj[key] : '-';
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
      const formattedKey = removeSpecialFunctions(key);
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
  return legend.map((entry) => Object.values(entry).join('/'));
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
 * @param attributeValue: legend value from side panel
 * @param isChecked: checkbox checked state
 */
export function updateTraceByLegend(
  dt: DataTable | undefined,
  attributeValue: string,
  isChecked: boolean
): string | null {
  if (!dt) {
    return null;
  }
  const numCols = dt!.getNumberOfColumns();
  // skip the first two columns since they are domains (commit position / date)
  for (let i = 2; i < numCols; i++) {
    const label = dt!.getColumnLabel(i);
    const lastSubsetValue = getlastLabel(label);

    if (attributeValue.split('/').pop() === lastSubsetValue) {
      dt!.setColumnProperty(i, 'visible', isChecked);
      return label;
    }
  }
  return null;
}

/**
 * Find the last subtest value from an object label from Google chart
 * @param a special function key like norm(,a=A,b=B,c=C,)
 * @example
 * Input: google chart label e.g ",benchmark=JetStream2,story=Total,test=avg,subtest_1=test1,"
 * Output: "test1"
 * @returns the last subtest value. Return subtest_3 if exists, else if return subtest_2 if exists,
 * else return subtest_1 if exists.
 */
export function getlastLabel(label: string): string {
  const temp = removeSpecialFunctions(label).split(',');
  const trimmed = temp.filter((val) => val.trim().length > 0);
  let lastKeyValue = '';
  trimmed.forEach((pair) => {
    if (pair.startsWith('subtest')) {
      const keyValue = pair.split('=');
      // if the key name is subtest_3
      if (keyValue[0] === labelKeys.at(labelKeys.length - 1)) {
        lastKeyValue = keyValue[1];
        // if the key name is subtest_2
      } else if (keyValue[0] === labelKeys.at(labelKeys.length - 2)) {
        lastKeyValue = keyValue[1];
        // if the key name is subtest_1
      } else if (keyValue[0] === labelKeys.at(labelKeys.length - 3)) {
        lastKeyValue = keyValue[1];
      }
    }
  });
  return lastKeyValue;
}
