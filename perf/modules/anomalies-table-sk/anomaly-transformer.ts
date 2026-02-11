import { Anomaly } from '../json';
import { AnomalyGroup } from './grouping';
import { getPercentChange } from '../common/anomaly';

/**
 * Shape of the processed anomaly data for display.
 */
export interface ProcessedAnomaly {
  bugId: number;
  revision: number;
  bot: string;
  testsuite: string;
  test: string;
  delta: number;
  isImprovement: boolean;
}

export class AnomalyTransformer {
  /**
   * Transforms a raw Anomaly into a display-ready ProcessedAnomaly object.
   */
  static getProcessedAnomaly(anomaly: Anomaly): ProcessedAnomaly {
    const bugId = anomaly.bug_id;
    const testPathPieces = anomaly.test_path.split('/');
    const bot = testPathPieces[1];
    const testsuite = testPathPieces[2];
    const test = testPathPieces.slice(3, testPathPieces.length).join('/');
    const revision = anomaly.start_revision;
    const delta = getPercentChange(anomaly.median_before_anomaly, anomaly.median_after_anomaly);
    return {
      bugId,
      revision,
      bot,
      testsuite,
      test,
      delta,
      isImprovement: anomaly.is_improvement,
    };
  }

  /**
   * Computes a readable revision range string.
   */
  static computeRevisionRange(start: number | null, end: number | null): string {
    if (start === null || end === null) {
      return '';
    }
    if (start === end) {
      return '' + end;
    }
    return start + ' - ' + end;
  }

  /**
   * Finds the longest common test path prefix for a list of anomalies.
   * Used for group summary rows.
   */
  static findLongestSubTestPath(anomalyList: Anomaly[]): string {
    if (anomalyList.length === 0) {
      return '';
    }
    // Check if this character exists at the same position in all other strings.
    let longestCommonTestPath = anomalyList[0]!.test_path;

    for (let i = 1; i < anomalyList.length; i++) {
      const currentString = anomalyList[i].test_path;
      // While the current string doesn't start with the prefix, shorten the prefix
      while (currentString.indexOf(longestCommonTestPath) !== 0) {
        longestCommonTestPath = longestCommonTestPath.substring(
          0,
          longestCommonTestPath.length - 1
        );

        if (longestCommonTestPath === '') {
          return '*';
        }
      }
    }

    // Return the common test path plus '' if the paths in the grouped rows are not the same.
    // '*' indicates where the test names differ in the collapsed rows.
    if (longestCommonTestPath.length !== anomalyList[0]!.test_path.length) {
      const testPath = longestCommonTestPath.split('/');
      // If we sliced off too much, we might have partial path segments.
      // Ideally we should split by '/' first, but sticking to original logic for parity now.
      // The original logic assumes test_path structure matches split('/').
      return testPath.slice(3, testPath.length).join('/') + '*';
    }
    // else return the original test path.
    return anomalyList[0]!.test_path;
  }

  /**
   * Determines the summary delta for a group of anomalies.
   * Returns [deltaValue, isRegression].
   */
  static determineSummaryDelta(anomalyGroup: AnomalyGroup): [number, boolean] {
    const regressions = anomalyGroup.anomalies.filter((a) => !a.is_improvement);
    let targetAnomalies = anomalyGroup.anomalies;
    if (regressions.length > 0) {
      // If there are regressions, find the one with the largest magnitude.
      targetAnomalies = regressions;
    }

    if (targetAnomalies.length === 0) {
      return [0, false];
    }

    const biggestChangeAnomaly = targetAnomalies.reduce((prev, current) => {
      const prevDelta = Math.abs(
        getPercentChange(prev.median_before_anomaly, prev.median_after_anomaly)
      );
      const currentDelta = Math.abs(
        getPercentChange(current.median_before_anomaly, current.median_after_anomaly)
      );
      return prevDelta > currentDelta ? prev : current;
    });

    return [
      getPercentChange(
        biggestChangeAnomaly.median_before_anomaly,
        biggestChangeAnomaly.median_after_anomaly
      ),
      regressions.length > 0,
    ];
  }
}
