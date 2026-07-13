import { makeKey } from '../paramtools';
import { TraceRow } from './trace-types';
/**
 * Find start and end indices of rows that are visible within viewMin/Max,
 * including one point buffer on each side for continuous lines.
 */
export function getCulledIndices(
  rows: any[],
  viewMin: number,
  viewMax: number,
  xAccessor: (r: any) => number = (r) => r.commit_number
): { start: number; end: number } {
  let startIdx = 0;
  while (startIdx < rows.length && xAccessor(rows[startIdx]) < viewMin) startIdx++;
  if (startIdx > 0) startIdx--;

  let endIdx = rows.length - 1;
  while (endIdx >= 0 && xAccessor(rows[endIdx]) > viewMax) endIdx--;
  if (endIdx < rows.length - 1) endIdx++;

  return {
    start: startIdx,
    end: Math.max(startIdx, endIdx),
  };
}

/**
 * Normalize a set of values based on centering and scaling strategies.
 * Operates on a range [start, end] of the input array to avoid allocations.
 */
export function normalizeValues(
  vals: number[] | { val: number }[] | { originalVal: number }[],
  center: 'none' | 'first' | 'average' | 'median',
  scaleStrategy: 'none' | 'minmax' | 'stddev' | 'iqr' | 'smoothed_std',
  start: number = 0,
  end: number = -1,
  valueKey: 'val' | 'originalVal' | null = null
): { offset: number; scale: number } {
  const actualEnd = end === -1 ? vals.length - 1 : end;
  const count = actualEnd - start + 1;

  if (count <= 0 || vals.length === 0) return { offset: 0, scale: 1 };

  const getVal = (idx: number): number => {
    const item = vals[idx];
    if (typeof item === 'number') return item;
    if (valueKey) return (item as any)[valueKey];
    return (item as any).val;
  };

  let offset = 0;
  if (center === 'first') {
    offset = getVal(start);
  } else if (center === 'average') {
    let sum = 0;
    for (let i = start; i <= actualEnd; i++) sum += getVal(i);
    offset = sum / count;
  } else if (center === 'median') {
    const slice = [];
    for (let i = start; i <= actualEnd; i++) slice.push(getVal(i));
    slice.sort((a, b) => a - b);
    offset = slice[Math.floor(slice.length / 2)];
  }

  let scale = 1;
  if (scaleStrategy === 'minmax') {
    let min = Infinity;
    let max = -Infinity;
    for (let i = start; i <= actualEnd; i++) {
      const v = getVal(i) - offset;
      if (v < min) min = v;
      if (v > max) max = v;
    }
    const range = max - min;
    if (range > 0) scale = 1 / range;
  } else if (scaleStrategy === 'stddev') {
    let sum = 0;
    for (let i = start; i <= actualEnd; i++) sum += getVal(i) - offset;
    const avg = sum / count;

    let sumSqDiff = 0;
    for (let i = start; i <= actualEnd; i++) {
      sumSqDiff += Math.pow(getVal(i) - offset - avg, 2);
    }
    const stdDev = Math.sqrt(sumSqDiff / count);
    if (stdDev > 0) scale = 1 / stdDev;
  } else if (scaleStrategy === 'iqr') {
    const centered = [];
    for (let i = start; i <= actualEnd; i++) centered.push(getVal(i) - offset);
    centered.sort((a, b) => a - b);
    const q1 = centered[Math.floor(centered.length * 0.25)];
    const q3 = centered[Math.floor(centered.length * 0.75)];
    const iqr = q3 - q1;
    if (iqr > 0) scale = 1 / iqr;
  } else if (scaleStrategy === 'smoothed_std') {
    if (count > 1) {
      const diffs = [];
      for (let i = start + 1; i <= actualEnd; i++) {
        diffs.push(Math.abs(getVal(i) - getVal(i - 1)));
      }
      diffs.sort((a, b) => a - b);
      const medianDiff = diffs[Math.floor(diffs.length / 2)];
      // 1.4826 is the scale factor for MAD to approximate stdDev for normal distribution
      let robustStd = medianDiff * 1.4826;
      if (robustStd === 0) {
        // Fallback to mean absolute difference if median is 0 (e.g. signal is mostly flat with spikes)
        let sumDiff = 0;
        for (let i = 0; i < diffs.length; i++) sumDiff += diffs[i];
        robustStd = (sumDiff / diffs.length) * 1.2533; // sqrt(pi/2) factor for mean absolute deviation
      }
      if (robustStd > 0) scale = 1 / robustStd;
    } else {
      // Fallback for single point
      scale = 1;
    }
  }

  return { offset, scale };
}

/**
 * Calculates measurement differences (delta and percentage)
 */
export function calculateMeasurement(
  startVal: number,
  endVal: number
): { diff: number; percent: number } {
  const diff = endVal - startVal;
  const percent = startVal !== 0 ? (diff / Math.abs(startVal)) * 100 : 0;
  return { diff, percent };
}

const HIDDEN_PARAMS = new Set(['master', 'unit', 'improvement_direction']);

function parseId(id: string): Record<string, string> {
  const pairs: Record<string, string> = {};
  id.split(',').forEach((p) => {
    const parts = p.split('=');
    if (parts.length === 2) {
      pairs[parts[0]] = parts[1];
    }
  });
  return pairs;
}

export function computeDiffParamNames(series: { id: string }[]): Map<string, string> {
  const map = new Map<string, string>();
  if (series.length === 0) return map;

  const parsed: Record<string, string>[] = [];
  series.forEach((s) => {
    const pairs = parseId(s.id);
    parsed.push(pairs);
  });

  if (parsed.length === 0) return map;

  const keys = Object.keys(parsed[0]);
  const commonKeys: string[] = [];

  keys.forEach((k) => {
    if (HIDDEN_PARAMS.has(k)) return;
    const firstVal = parsed[0][k];
    const allMatch = parsed.every((p) => p[k] === firstVal);
    if (allMatch) {
      commonKeys.push(k);
    }
  });

  series.forEach((s, idx) => {
    const p = parsed[idx];
    const diffs = Object.entries(p)
      .filter(([k]) => !HIDDEN_PARAMS.has(k) && !commonKeys.includes(k))
      .map(([k, v]) => `${k}=${v}`)
      .join(', ');
    map.set(s.id, diffs || 'Shared Baseline');
  });

  return map;
}

export function computeChartDimensions(series: { id: string }[]): string[] {
  if (series.length === 0) return [];

  const parsed: Record<string, string>[] = [];
  const allKeys = new Set<string>();

  series.forEach((s) => {
    const pairs = parseId(s.id);
    parsed.push(pairs);
    Object.keys(pairs).forEach((k) => allKeys.add(k));
  });

  const differingKeys: string[] = [];

  allKeys.forEach((k) => {
    if (HIDDEN_PARAMS.has(k)) return;
    const firstVal = parsed[0][k];
    const allMatch = parsed.every((p) => p[k] === firstVal);
    if (!allMatch) {
      differingKeys.push(k);
    }
  });

  return differingKeys.sort();
}

export function computeTraceDiffs(
  series: { id: string; rows: any[] }[],
  diffBase: { key: string; value: string }
): { id: string; rows: any[] }[] {
  const bases: { id: string; rows: any[] }[] = [];
  const targets: { id: string; rows: any[] }[] = [];

  series.forEach((s) => {
    const params = parseId(s.id);
    if (params[diffBase.key] === diffBase.value) {
      bases.push(s);
    } else {
      targets.push(s);
    }
  });

  if (bases.length === 0) return series;

  const baseMap = new Map<string, { id: string; rows: any[] }>();
  bases.forEach((b) => {
    const params = parseId(b.id);
    const sig = Object.entries(params)
      .filter(([k]) => k !== diffBase.key && !HIDDEN_PARAMS.has(k))
      .sort((e1, e2) => e1[0].localeCompare(e2[0]))
      .map(([k, v]) => `${k}=${v}`)
      .join('|');
    baseMap.set(sig, b);
  });

  const diffs: { id: string; rows: any[] }[] = [];
  targets.forEach((t) => {
    const params = parseId(t.id);
    const sig = Object.entries(params)
      .filter(([k]) => k !== diffBase.key && !HIDDEN_PARAMS.has(k))
      .sort((a, b) => a[0].localeCompare(b[0]))
      .map(([k, v]) => `${k}=${v}`)
      .join('|');

    const base = baseMap.get(sig);
    if (!base) return;

    const sortedBaseRows = [...base.rows].sort((a, b) => a.commit_number - b.commit_number);

    const newRows = t.rows
      .map((r) => {
        let bVal: number | null = null;
        let low = 0,
          high = sortedBaseRows.length - 1;
        let idx = -1;
        while (low <= high) {
          const mid = Math.floor((low + high) / 2);
          if (sortedBaseRows[mid].commit_number >= r.commit_number) {
            idx = mid;
            high = mid - 1;
          } else {
            low = mid + 1;
          }
        }

        if (idx !== -1) {
          const r2 = sortedBaseRows[idx];
          if (r2.commit_number === r.commit_number) {
            bVal = r2.val;
          } else if (idx > 0) {
            const rPrev = sortedBaseRows[idx - 1];
            const distNext = r2.commit_number - r.commit_number;
            const distPrev = r.commit_number - rPrev.commit_number;
            if (distNext < distPrev) {
              bVal = r2.val;
            } else {
              bVal = rPrev.val;
            }
          } else {
            bVal = r2.val;
          }
        } else if (sortedBaseRows.length > 0) {
          bVal = sortedBaseRows[sortedBaseRows.length - 1].val;
        }

        if (bVal !== null && bVal !== 0) {
          return { ...r, val: (r.val - bVal) / bVal };
        }
        return null;
      })
      .filter((r) => r !== null);

    if (newRows.length > 0) {
      diffs.push({ ...t, rows: newRows as any[] });
    }
  });

  return diffs;
}

export function computeSplitGroups(
  series: { id: string; rows: any[] }[],
  splitKeys: Set<string>,
  splitAll?: boolean
): { id: string; title: string; series: { id: string; rows: any[] }[]; values: string[] }[] {
  if (series.length === 0) return [];

  if (splitAll) {
    return series.map((s) => ({
      id: s.id,
      title: s.id,
      series: [s],
      values: [s.id],
    }));
  }

  const effectiveKeys = Array.from(splitKeys);

  const groups: Record<string, { id: string; rows: any[] }[]> = {};
  const keyValues: Record<string, Set<string>> = {};
  effectiveKeys.forEach((k) => (keyValues[k] = new Set()));

  series.forEach((s) => {
    const params = parseId(s.id);
    const values = effectiveKeys.map((k) => {
      const val = params[k] || '(missing)';
      keyValues[k].add(val);
      return val;
    });
    const key = values.join('|');
    if (!groups[key]) groups[key] = [];
    groups[key].push(s);
  });

  const sortedGroups = Object.entries(groups)
    .map(([compKey, groupSeries]) => {
      const parsed = groupSeries.map((s) => parseId(s.id));
      const allKeys = new Set<string>();
      parsed.forEach((p) => Object.keys(p).forEach((k) => allKeys.add(k)));

      const titleParts: string[] = [];
      allKeys.forEach((k) => {
        if (HIDDEN_PARAMS.has(k)) return;
        const firstVal = parsed[0][k];
        if (firstVal !== undefined) {
          const allMatch = parsed.every((p) => p[k] === firstVal);
          if (allMatch) {
            titleParts.push(`${k}=${firstVal}`);
          }
        }
      });
      titleParts.sort();

      const values = compKey.split('|');
      return {
        id: compKey,
        title: titleParts.join(', ') || 'Global Traces',
        series: groupSeries,
        values,
      };
    })
    .sort((a, b) => {
      for (let i = 0; i < effectiveKeys.length; i++) {
        const valA = a.values[i];
        const valB = b.values[i];
        const cmp = valA.localeCompare(valB);
        if (cmp !== 0) return cmp;
      }
      return 0;
    });

  return sortedGroups;
}

/**
 * Paginate traces after initial grouping to preserve group order but limit total visible traces.
 */
export function paginateTraces<T>(sortedGroups: T[][], pageSize: number): T[][] {
  const pages: T[][] = [];
  let currentPage: T[] = [];

  for (const group of sortedGroups) {
    if (group.length <= pageSize) {
      if (currentPage.length + group.length <= pageSize) {
        currentPage.push(...group);
      } else {
        if (currentPage.length > 0) pages.push(currentPage);
        currentPage = [...group];
      }
    } else {
      if (currentPage.length > 0) pages.push(currentPage);
      currentPage = [];
      for (let i = 0; i < group.length; i += pageSize) {
        pages.push(group.slice(i, i + pageSize));
      }
    }
  }
  if (currentPage.length > 0) pages.push(currentPage);
  return pages;
}

/**
 * Calculates the min and max commit numbers for each trace in the series.
 */
export function calculateLoadedBounds(
  series: { id: string; rows: { commit_number: number; createdat?: number }[] }[],
  byTime: boolean = false
): Record<string, { min: number; max: number }> {
  const bounds: Record<string, { min: number; max: number }> = {};
  series.forEach((s) => {
    if (s.rows && s.rows.length > 0) {
      let min = Infinity;
      let max = -Infinity;
      s.rows.forEach((r) => {
        const val = byTime ? r.createdat : r.commit_number;
        if (val !== undefined) {
          if (val < min) min = val;
          if (val > max) max = val;
        }
      });
      bounds[s.id] = { min, max };
    }
  });
  return bounds;
}

export function computeLeftPadding(maxY: number, minY: number): number {
  const maxLabel = Math.max(Math.abs(maxY), Math.abs(minY)).toFixed(2);
  const length = maxLabel.length;
  // Estimate width: 6 pixels per character + 25 pixels for margin and title
  const estimatedWidth = length * 6 + 25;
  return Math.max(60, estimatedWidth);
}

export function calculateSharedBounds(
  series: { id: string; source?: string; rows: any[] }[],
  globalBounds: Record<string, { min: number; max: number }> | null,
  isDateMode: boolean = false,
  xAccessor: (r: any) => number = (r) => r.commit_number
): Record<string, { min: number; max: number }> | null {
  if (!series || series.length === 0) return null;

  const boundsBySource: Record<string, { min: number; max: number }> = {};

  // First pass: Only look at actual loaded rows
  series.forEach((s) => {
    const source = s.source || 'chrome';
    if (!boundsBySource[source]) boundsBySource[source] = { min: Infinity, max: -Infinity };
    s.rows.forEach((r) => {
      const val = xAccessor(r);
      if (val < boundsBySource[source].min) boundsBySource[source].min = val;
      if (val > boundsBySource[source].max) boundsBySource[source].max = val;
    });
  });

  // Second pass: If we have literally no rows anywhere, THEN fallback to global bounds
  Object.keys(boundsBySource).forEach((source) => {
    if (boundsBySource[source].min === Infinity) {
      // ONLY fallback to globalBounds if we are NOT in date mode.
      if (!isDateMode) {
        series
          .filter((s) => (s.source || 'chrome') === source)
          .forEach((s) => {
            const gb = globalBounds?.[s.id];
            if (gb) {
              if (gb.min < boundsBySource[source].min) boundsBySource[source].min = gb.min;
              if (gb.max > boundsBySource[source].max) boundsBySource[source].max = gb.max;
            }
          });
      }
      if (boundsBySource[source].min === Infinity) {
        delete boundsBySource[source];
      }
    }
  });

  return Object.keys(boundsBySource).length > 0 ? boundsBySource : null;
}

const MISSING_DATA_SENTINEL = 1e32;

const avg = (arr: number[], i: number, windowSize: number): number => {
  const start = i - windowSize + 1;
  const end = i;
  const slice = arr
    .slice(Math.max(0, start), Math.min(arr.length, end + 1))
    .filter((v) => v !== MISSING_DATA_SENTINEL);
  if (slice.length === 0) return MISSING_DATA_SENTINEL;
  return slice.reduce((a, b) => a + b, 0) / slice.length;
};

const median = (arr: number[], i: number, windowSize: number): number => {
  const start = i - windowSize + 1;
  const end = i;
  const slice = arr
    .slice(Math.max(0, start), Math.min(arr.length, end + 1))
    .filter((v) => v !== MISSING_DATA_SENTINEL);
  if (slice.length === 0) return MISSING_DATA_SENTINEL;
  slice.sort((a, b) => a - b);
  return slice[Math.floor(slice.length / 2)];
};

const stddev = (arr: number[], i: number, windowSize: number): number => {
  const start = i - windowSize + 1;
  const end = i;
  const slice = arr
    .slice(Math.max(0, start), Math.min(arr.length, end + 1))
    .filter((v) => v !== MISSING_DATA_SENTINEL);
  if (slice.length === 0) return MISSING_DATA_SENTINEL;
  const avgVal = slice.reduce((a, b) => a + b, 0) / slice.length;
  const sumSq = slice.reduce((sum, v) => sum + Math.pow(v - avgVal, 2), 0);
  return Math.sqrt(sumSq / slice.length);
};

export interface TransformableSeries {
  id: string;
  originalId?: string;
  rows: TraceRow[];
  [key: string]: any;
}

export function computeCustomTransforms(
  series: TransformableSeries[],
  preset: string
): TransformableSeries[] {
  if (!preset || preset === 'none') return series;

  const result: TransformableSeries[] = [];
  series.forEach((s) => {
    result.push(s);

    const params = parseId(s.id);
    params['special'] = 'transform';
    const diffId = makeKey(params);

    const rows = s.rows || [];
    const X = rows.map((r: TraceRow) => r.val);
    const commits = rows.map((r: TraceRow) => r.commit_number);

    const diffRows: TraceRow[] = [];
    for (let i = 0; i < rows.length; i++) {
      const row = rows[i];
      let val = MISSING_DATA_SENTINEL;

      if (preset === 'delta') {
        if (i > 0) {
          val = X[i] - X[i - 1];
        }
      } else if (preset === 'rel_delta') {
        if (i > 0 && X[i - 1] !== 0 && X[i - 1] !== MISSING_DATA_SENTINEL) {
          val = (X[i] - X[i - 1]) / X[i - 1];
        }
      } else if (preset === 'velocity') {
        if (i > 0 && commits[i] !== commits[i - 1]) {
          val = (X[i] - X[i - 1]) / (commits[i] - commits[i - 1]);
        }
      } else if (preset === 'avg3') {
        val = avg(X, i, 3);
      } else if (preset === 'median3') {
        val = median(X, i, 3);
      } else if (preset === 'stddev10') {
        val = stddev(X, i, 10);
      }

      if (val !== MISSING_DATA_SENTINEL && !isNaN(val)) {
        diffRows.push({
          ...row,
          val: val,
          smoothedVal: undefined,
        });
      }
    }

    if (diffRows.length > 0) {
      result.push({
        ...s,
        id: diffId,
        originalId: s.id,
        rows: diffRows,
      });
    }
  });
  return result;
}
