import { DataFrame } from '../json';

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
export const getTitle = (df: DataFrame): object => {
  const traceKeys = Object.keys(df!.traceset);
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
 * ]
 * legend = [{
 *   "story":"Total",
 *   "test":"avg"
 *  }, {
 *   "story":"Air"
 *   "test":"std"
 *  }]
 */
export const getLegend = (df: DataFrame): object[] => {
  const traceKeys = Object.keys(df!.traceset);
  const pairs = convertKeysToPairs(traceKeys);

  const legend = [];
  // for each key, split the traceKey into key-value pairs
  // and filter for entries that do not appear across all traceKeys
  for (const key of traceKeys) {
    const kvp = key.split(',').filter((pair) => pair.length);
    const uniq = kvp.filter((item) => pairs.filter((x) => x === item).length < traceKeys.length);
    legend.push(
      Object.fromEntries(
        uniq.map((item) => {
          return item.split('=');
        })
      )
    );
  }

  return legend;
};

// converts traceKeys to a list of individual key-value pairs
// and removing any trailing spaces and commas
function convertKeysToPairs(traceKeys: string[]) {
  return traceKeys.map((key) => key.split(',').filter((pair) => pair.length)).flat();
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
export function isSingleTrace(df: DataFrame | undefined): boolean | null {
  if (!df) {
    return null;
  }
  const traceKeys = Object.keys(df.traceset);
  return traceKeys.length === 1;
}
