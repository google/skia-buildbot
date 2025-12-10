import { Anomaly } from '../json';

export type RevisionGroupingMode = 'EXACT' | 'OVERLAPPING' | 'ANY';
export type GroupingCriteria = 'BENCHMARK' | 'BOT' | 'TEST' | 'NONE';

export interface AnomalyGroupingConfig {
  revisionMode: RevisionGroupingMode;
  groupBy: Set<GroupingCriteria>;
  /** * If true, applies the 'groupBy' logic to anomalies that did not
   * fit into any revision group. */
  groupSingles: boolean;
}

export class AnomalyGroup {
  anomalies: Anomaly[] = [];

  expanded: boolean = false;
}

export function doRangesOverlap(a: Anomaly, b: Anomaly): boolean {
  if (a.start_revision > b.start_revision) {
    [a, b] = [b, a];
  }

  if (
    a.start_revision === null ||
    a.end_revision === null ||
    b.start_revision === null ||
    b.end_revision === null
  ) {
    return false;
  }
  return a.start_revision <= b.end_revision && a.end_revision >= b.start_revision;
}

/**
 * Helper method to group anomalies based on a predicate.
 *
 * It takes a list of anomalies, groups them, and then partitions the result
 * into groups containing multiple items and a flat list of anomalies that
 * remained in single-item groups.
 *
 * @param anomalies - The list of anomalies to group.
 * @param predicate - A function that returns true if two anomalies belong in the same group.
 * @returns An object containing the grouped anomalies and the remaining single anomalies.
 */
function groupAndPartition(
  anomalies: Anomaly[],
  predicate: (a: Anomaly, b: Anomaly) => boolean
): { multiItemGroups: AnomalyGroup[]; singleAnomalies: Anomaly[] } {
  if (!anomalies.length) {
    return { multiItemGroups: [], singleAnomalies: [] };
  }

  // Use reduce to iterate once and create all groups.
  const allGroups = anomalies.reduce((groups: AnomalyGroup[], anomaly) => {
    const existingGroup = groups.find((g) =>
      g.anomalies.every((other) => predicate(anomaly, other))
    );

    if (existingGroup) {
      existingGroup.anomalies.push(anomaly);
    } else {
      const newGroup = new AnomalyGroup();
      newGroup.anomalies = [anomaly];
      groups.push(newGroup);
    }
    return groups;
  }, []);

  // Now, partition the results into multi-item groups and singles.
  const multiItemGroups: AnomalyGroup[] = [];
  const singleAnomalies: Anomaly[] = [];
  for (const group of allGroups) {
    if (group.anomalies.length > 1) {
      multiItemGroups.push(group);
    } else {
      singleAnomalies.push(group.anomalies[0]);
    }
  }

  return { multiItemGroups, singleAnomalies };
}

export function isSameBenchmark(a: Anomaly, b: Anomaly) {
  const testSuiteA = a.test_path.split('/').length > 2 ? a.test_path.split('/')[2] : '';
  const testSuiteB = b.test_path.split('/').length > 2 ? b.test_path.split('/')[2] : '';
  return testSuiteA === testSuiteB;
}

export function isSameRevision(a: Anomaly, b: Anomaly) {
  return a.start_revision === b.start_revision && a.end_revision === b.end_revision;
}

export function isSameBot(a: Anomaly, b: Anomaly): boolean {
  const botA = a.test_path.split('/').length > 1 ? a.test_path.split('/')[1] : '';
  const botB = b.test_path.split('/').length > 1 ? b.test_path.split('/')[1] : '';
  return botA === botB;
}

export function isSameTest(a: Anomaly, b: Anomaly): boolean {
  const testA = a.test_path.split('/').length > 3 ? a.test_path.split('/')[3] : '';
  const testB = b.test_path.split('/').length > 3 ? b.test_path.split('/')[3] : '';
  return testA === testB;
}

/**
 * Groups anomalies based on a configurable strategy.
 *
 * The process is as follows:
 * 1. Anomalies with bug IDs are always grouped together.
 * 2. Anomalies without bug IDs are grouped by revision, based on the `revisionMode`.
 * 3. The resulting revision groups can be further split by attributes in `groupBy`.
 * 4. Anomalies that didn't fit into any revision group ('singles') can also be
 *    grouped based on the `groupSingles` and `groupBy` settings.
 */
export function groupAnomalies(
  anomalyList: Anomaly[],
  config: AnomalyGroupingConfig
): AnomalyGroup[] {
  // 1. Unconditional Override: Separate by Bug ID
  const withBugId: Anomaly[] = [];
  const withoutBugId: Anomaly[] = [];

  for (const anomaly of anomalyList) {
    if (anomaly.bug_id && anomaly.bug_id > 0) {
      withBugId.push(anomaly);
    } else {
      withoutBugId.push(anomaly);
    }
  }

  // Group the bug_id ones
  const bugIdGroupMap = withBugId.reduce((map, anomaly) => {
    const bugId = anomaly.bug_id!;
    const group = map.get(bugId) || [];
    map.set(bugId, [...group, anomaly]);
    return map;
  }, new Map<number, Anomaly[]>());

  const bugIdGroups: AnomalyGroup[] = Array.from(bugIdGroupMap.values()).map((anomalies) => {
    const group = new AnomalyGroup();
    group.anomalies = anomalies;
    return group;
  });

  // 2. Configurable Logic: Revision Grouping
  let revisionGroups: AnomalyGroup[] = [];
  let singlesPool: Anomaly[] = [];

  switch (config.revisionMode) {
    case 'EXACT': {
      // Only group if ranges are identical
      const result = groupAndPartition(withoutBugId, (a, b) => isSameRevision(a, b));
      revisionGroups = result.multiItemGroups;
      singlesPool = result.singleAnomalies;
      break;
    }
    case 'OVERLAPPING': {
      // Pass A: Group by Exact Match first
      const exactResult = groupAndPartition(withoutBugId, (a, b) => isSameRevision(a, b));

      // Pass B: Group remaining singles by Overlapping Range
      const overlapResult = groupAndPartition(exactResult.singleAnomalies, (a, b) =>
        doRangesOverlap(a, b)
      );

      // Combine both sets of groups
      revisionGroups = [...exactResult.multiItemGroups, ...overlapResult.multiItemGroups];
      singlesPool = overlapResult.singleAnomalies;
      break;
    }
    case 'ANY': {
      // Treat all anomalies as one large group
      if (withoutBugId.length > 0) {
        const group = new AnomalyGroup();
        group.anomalies = withoutBugId;
        revisionGroups = [group];
      }
      singlesPool = [];
      break;
    }
  }

  // 3. Configurable Logic: Attribute Splitting
  const predicateMap: Partial<Record<GroupingCriteria, (a: Anomaly, b: Anomaly) => boolean>> = {
    BENCHMARK: isSameBenchmark,
    BOT: isSameBot,
    TEST: isSameTest,
  };
  const predicates = Array.from(config.groupBy)
    .map((c) => predicateMap[c])
    .filter((p): p is (a: Anomaly, b: Anomaly) => boolean => !!p);

  let processedGroups: AnomalyGroup[] = [];
  if (predicates.length > 0) {
    const combinedPredicate = (a: Anomaly, b: Anomaly) => predicates.every((p) => p(a, b));
    for (const group of revisionGroups) {
      const { multiItemGroups, singleAnomalies } = groupAndPartition(
        group.anomalies,
        combinedPredicate
      );

      processedGroups.push(...multiItemGroups);
      singleAnomalies.forEach((a) => {
        const singleGroup = new AnomalyGroup();
        singleGroup.anomalies = [a];
        processedGroups.push(singleGroup);
      });
    }
  } else {
    processedGroups = revisionGroups;
  }

  // 4. Configurable Logic: Grouping Singles
  const processedSingles: AnomalyGroup[] = [];
  if (config.groupSingles && predicates.length > 0) {
    const combinedPredicate = (a: Anomaly, b: Anomaly) => predicates.every((p) => p(a, b));
    const { multiItemGroups, singleAnomalies } = groupAndPartition(singlesPool, combinedPredicate);
    processedSingles.push(...multiItemGroups);
    singleAnomalies.forEach((a) => {
      const singleGroup = new AnomalyGroup();
      singleGroup.anomalies = [a];
      processedSingles.push(singleGroup);
    });
  } else {
    singlesPool.forEach((a) => {
      const singleGroup = new AnomalyGroup();
      singleGroup.anomalies = [a];
      processedSingles.push(singleGroup);
    });
  }

  // 5. Final Assembly
  return [...bugIdGroups, ...processedGroups, ...processedSingles];
}
