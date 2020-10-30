// Utility functions for trybot results.

import { TryBotResponse } from '../json';

// The average stddevRatio across all traces with the given keyValue.
export interface AveForParam {
    keyValue: string;
    aveStdDevRatio: number;
}

interface runningTotal {
    totalStdDevRatio: number;
    n: number;
}

/** Returns an array of AveForParam, where each one contains
 *  a key=value param and the average stddevRatio across
 *  all traces that match that key=value.
 *
 *  The results are sorted on aveStdDevRatio descending.
 */
export function byParams(res: TryBotResponse): AveForParam[] {
  // Sum up all the stdDevRatios over all the key=value axes.
  const runningTotals: {[key: string]: runningTotal} = {};
  res.results!.forEach((r) => {
    Object.entries(r.params).forEach((keyValue) => {
      const [key, value] = keyValue;
      const runningTotalsKey = `${key}=${value}`;
      let t = runningTotals[runningTotalsKey];
      if (!t) {
        t = {
          totalStdDevRatio: 0,
          n: 0,
        };
      }
      t.totalStdDevRatio += r.stddevRatio;
      t.n += 1;
      runningTotals[runningTotalsKey] = t;
    });
  });

  // Now determine the average across each key=value axis.
  const ret: AveForParam[] = [];
  Object.keys(runningTotals).forEach((runningTotalKey) => {
    const r = runningTotals[runningTotalKey];
    ret.push({
      keyValue: runningTotalKey,
      aveStdDevRatio: r.totalStdDevRatio / r.n,
    });
  });

  // Sort by aveStdDevRatio.
  ret.sort((a: AveForParam, b: AveForParam) => b.aveStdDevRatio - a.aveStdDevRatio);

  return ret;
}
