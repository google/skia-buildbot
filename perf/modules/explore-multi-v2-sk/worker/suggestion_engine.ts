import { Param, TraceData, WasmExports, Query } from './worker-types';
import { scoreParamAny, fuzzyScore } from '../fuzzy';

export interface SuggestionResult {
  params: Param[];
  score: number;
}

export interface SearchCache {
  query: string;
  contextStr: string;
  indices: Int32Array | null;
}

const MAX_CANDIDATES = 100;
const OUTPUT_LIMIT = 1000000;
const YIELD_INTERVAL_MS = 12;

export async function computeSuggestions(
  queryInput: string,
  currentQuery: Query,
  params: Param[],
  availableParams: Param[] | null,
  traceData: TraceData | null,
  wasmFilter: WasmExports | null,
  searchCache: SearchCache,
  updateCache: (cache: SearchCache) => void,
  yieldToMain: () => Promise<void>,
  shouldAbort: () => boolean,
  getQueryPtr: (data: TraceData) => number,
  scanWasmBatch: (
    wasmFilter: WasmExports,
    traceData: TraceData,
    queryPtr: number,
    queryLen: number,
    outputLimit: number,
    totalQueryValues: number,
    checkInterrupt: () => boolean
  ) => Promise<number>
): Promise<SuggestionResult[] | null> {
  if (shouldAbort()) return null;

  const tokens = queryInput
    .trim()
    .split(/\s+/)
    .filter((t) => t.length > 0);
  if (tokens.length === 0) {
    return [];
  }

  let pool: Param[] = [];
  const currentQueryKeys = Object.keys(currentQuery);
  const queryKeySet = new Set(currentQueryKeys);

  if (availableParams) {
    // Build a fast lookup map for global params: "key=value" -> id
    const paramLookup = new Map<string, number>();
    for (const p of params) {
      paramLookup.set(`${p.key}=${p.value}`, p.id);
    }

    for (const ap of availableParams) {
      if (queryKeySet.has(ap.key)) continue; // Exclude existing keys

      const id = paramLookup.get(`${ap.key}=${ap.value}`);
      if (id !== undefined) {
        pool.push({
          id: id,
          key: ap.key,
          value: ap.value,
        });
      }
    }
  } else if (traceData) {
    const { matchingParams, bitsetSize } = traceData;
    const keyToIndex = new Map<string, number>();
    currentQueryKeys.forEach((k, i) => keyToIndex.set(k, i));
    const isQueryEmpty = currentQueryKeys.length === 0;

    for (const p of params) {
      if (queryKeySet.has(p.key)) continue; // Exclude existing keys

      let bitsetOffset = 0;
      if (keyToIndex.has(p.key)) {
        const k = keyToIndex.get(p.key)!;
        bitsetOffset = (k + 1) * bitsetSize;
      }
      if (isQueryEmpty || matchingParams[bitsetOffset + p.id] > 0) {
        pool.push(p);
      }
    }
  } else {
    pool = params;
  }
  console.log('[computeSuggestions] tokens:', tokens, 'pool size:', pool.length);

  const tokenCandidateSets = tokens.map((token) => {
    const eqIdx = token.indexOf('=');
    const vPartCheck = eqIdx !== -1 ? token.substring(eqIdx + 1) : '';
    const hasGlobChar =
      vPartCheck.includes('*') || vPartCheck.includes('?') || vPartCheck.includes(',');
    const isGlobSearch = eqIdx !== -1 && hasGlobChar;

    if (isGlobSearch) {
      const kPart = token.substring(0, eqIdx);
      const vPart = token.substring(eqIdx + 1);

      if (!vPart) return [];

      try {
        const parts = vPart
          .split(',')
          .map((s) => s.trim())
          .filter(Boolean);
        const regexes = parts.map((part) => {
          const escaped = part.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
          const pattern = '^' + escaped.replace(/\\\*/g, '.*').replace(/\\\?/g, '.') + '$';
          return new RegExp(pattern, 'i');
        });

        const keyScores = new Map<string, number>();
        for (const p of pool) {
          if (!keyScores.has(p.key)) {
            const score = kPart ? fuzzyScore(p.key, kPart) : 0;
            keyScores.set(p.key, score);
          }
        }

        const matches: { p: Param; score: number }[] = [];
        const seenKeys = new Set<string>();

        for (const p of pool) {
          const kScore = keyScores.get(p.key)!;
          if (kScore === -Infinity) continue;

          if (!seenKeys.has(p.key)) {
            const matchesRegex = regexes.some((r) => r.test(p.value));
            if (matchesRegex) {
              seenKeys.add(p.key);
              matches.push({
                p: { id: -1, key: p.key, value: vPart },
                score: kScore + 1000, // High priority
              });
            }
          }
        }

        matches.sort((a, b) => b.score - a.score);
        return matches.slice(0, 50).map((m) => m.p);
      } catch (_e) {
        return [];
      }
    }

    const scored = pool.map((p) => ({ p, score: scoreParamAny(p, token) }));
    const matches = scored.filter((s) => s.score > -Infinity);
    console.log('[computeSuggestions] matches count for token', token, ':', matches.length);
    matches.sort((a, b) => b.score - a.score);

    if (matches.length > 0) {
      const bestScore = matches[0].score;
      const cutoff = bestScore === Infinity ? 50 : bestScore - 40;
      const qualified = matches.filter((m) => m.score >= cutoff);
      return qualified.slice(0, 1000).map((m) => m.p);
    }
    return [];
  });

  if (tokenCandidateSets.some((set) => set.length === 0)) {
    return [];
  }

  let suggestions: any[] = [];
  let isSampled = false;

  if (traceData && wasmFilter) {
    const currentContextStr = JSON.stringify(currentQuery);
    let sourceIndices: Int32Array | null = null;
    let sourceCount = 0;
    let isRefinement = false;

    const newChars = queryInput.substring(searchCache.query.length);
    const addedGlob = newChars.includes('*') || newChars.includes('?') || newChars.includes(',');

    if (
      searchCache.indices &&
      queryInput.startsWith(searchCache.query) &&
      currentContextStr === searchCache.contextStr &&
      !addedGlob
    ) {
      sourceIndices = searchCache.indices;
      sourceCount = sourceIndices.length;
      isRefinement = true;
    }

    if (!isRefinement) {
      const smallSets: { set: Param[]; index: number }[] = [];
      tokenCandidateSets.forEach((set, i) => {
        if (set.length <= MAX_CANDIDATES) smallSets.push({ set, index: i });
      });

      if (smallSets.length === 0 && tokens.length > 0) {
        let bestIdx = -1;
        let minSize = Infinity;
        tokenCandidateSets.forEach((set, i) => {
          if (set.length < minSize) {
            minSize = set.length;
            bestIdx = i;
          }
        });

        if (bestIdx !== -1) {
          const RESCUE_LIMIT = 200;
          const truncatedSet = tokenCandidateSets[bestIdx].slice(0, RESCUE_LIMIT);
          smallSets.push({ set: truncatedSet, index: bestIdx });
        }
      }

      const serializedQuery: number[] = [];
      const currentQueryEntries = Object.entries(currentQuery);
      const totalKeys = smallSets.length + currentQueryEntries.length;
      serializedQuery.push(totalKeys);

      for (const { set } of smallSets) {
        const expandedIds: number[] = [];
        for (const p of set) {
          if (p.id === -1) {
            try {
              const parts = p.value
                .split(',')
                .map((s) => s.trim())
                .filter(Boolean);
              const regexes = parts.map((part) => {
                const escaped = part.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
                const pattern = '^' + escaped.replace(/\\\*/g, '.*').replace(/\\\?/g, '.') + '$';
                return new RegExp(pattern, 'i');
              });
              for (const globalP of params) {
                if (globalP.key === p.key && regexes.some((r) => r.test(globalP.value))) {
                  expandedIds.push(globalP.id);
                }
              }
            } catch (_e) {}
          } else {
            expandedIds.push(p.id);
          }
        }
        const uniqueIds = Array.from(new Set(expandedIds));
        serializedQuery.push(uniqueIds.length);
        serializedQuery.push(...uniqueIds);
      }
      for (const [key, values] of currentQueryEntries) {
        const ids = values
          .map((v) => params.find((p) => p.key === key && p.value === v)?.id)
          .filter((id): id is number => id !== undefined);
        serializedQuery.push(ids.length);
        serializedQuery.push(...ids);
      }

      const queryPtr = getQueryPtr(traceData);
      const queryView = new Int32Array(traceData.memory.buffer, queryPtr, serializedQuery.length);
      queryView.set(serializedQuery);

      let totalQueryValues = 0;
      for (const { set } of smallSets) totalQueryValues += set.length;
      for (const [_, values] of currentQueryEntries) totalQueryValues += values.length;

      sourceCount = await scanWasmBatch(
        wasmFilter,
        traceData,
        queryPtr,
        serializedQuery.length,
        OUTPUT_LIMIT,
        totalQueryValues,
        shouldAbort
      );

      if (sourceCount === -1) return null;

      isSampled = sourceCount > OUTPUT_LIMIT;
      sourceIndices = new Int32Array(
        traceData.memory.buffer,
        traceData.outPtr,
        Math.min(sourceCount, OUTPUT_LIMIT)
      );
    }

    const checkCount = sourceIndices!.length;
    const uniquePaths = new Map<string, { params: Param[]; score: number; frequency: number }>();
    const survivingIndices: number[] = [];
    let lastYield = performance.now();

    for (let k = 0; k < checkCount; k++) {
      if (performance.now() - lastYield > YIELD_INTERVAL_MS) {
        await yieldToMain();
        if (shouldAbort()) return null;
        lastYield = performance.now();
      }

      const traceIndex = sourceIndices![k];
      const offset = traceIndex * traceData.stride;
      const traceParamIds = [];
      for (let j = 0; j < traceData.stride; j++) {
        const pid = traceData.paramSets[offset + j];
        if (pid === 0) continue;
        traceParamIds.push(pid);
      }
      const traceParams = traceParamIds.map((id) => params[id - 1]);

      const tokenMatches: { tokenIndex: number; candidates: { p: Param; score: number }[] }[] = [];
      let possible = true;
      for (let t = 0; t < tokens.length; t++) {
        const matchesWithScore = traceParams
          .map((p) => ({ p, score: scoreParamAny(p, tokens[t]) }))
          .filter((m) => m.score > -Infinity);
        if (matchesWithScore.length === 0) {
          possible = false;
          break;
        }
        const globCand = tokenCandidateSets[t].find(
          (cand) => cand.id === -1 && matchesWithScore.some((m) => m.p.key === cand.key)
        );
        if (globCand) {
          const bestScore = Math.max(...matchesWithScore.map((m) => m.score));
          tokenMatches.push({ tokenIndex: t, candidates: [{ p: globCand, score: bestScore }] });
        } else {
          matchesWithScore.sort((a, b) => b.score - a.score);
          tokenMatches.push({ tokenIndex: t, candidates: matchesWithScore });
        }
      }
      if (!possible) continue;
      survivingIndices.push(traceIndex);

      tokenMatches.sort((a, b) => a.candidates.length - b.candidates.length);
      const assignments = new Array(tokens.length).fill(null);
      const usedKeys = new Set<string>();
      let maxTotalScore = -Infinity;
      let bestPath: Param[] | null = null;

      const solve = (idx: number, currentScore: number) => {
        if (idx === tokens.length) {
          if (currentScore > maxTotalScore) {
            maxTotalScore = currentScore;
            const path: Param[] = new Array(tokens.length);
            for (let i = 0; i < tokens.length; i++) {
              const tIdx = tokenMatches[i].tokenIndex;
              path[tIdx] = assignments[i];
            }
            bestPath = path;
          }
          return;
        }
        const { candidates } = tokenMatches[idx];
        for (const cand of candidates) {
          if (!usedKeys.has(cand.p.key)) {
            usedKeys.add(cand.p.key);
            assignments[idx] = cand.p;
            solve(idx + 1, currentScore + cand.score);
            usedKeys.delete(cand.p.key);
            assignments[idx] = null;
          }
        }
      };
      solve(0, 0);

      if (bestPath) {
        const p = bestPath as Param[];
        const sortedForGrouping = [...p].sort((a, b) => a.id - b.id);
        const pathKey = sortedForGrouping.map((p) => p.id).join(',');
        if (uniquePaths.has(pathKey)) {
          const entry = uniquePaths.get(pathKey)!;
          entry.frequency++;
          if (maxTotalScore > entry.score) entry.score = maxTotalScore;
        } else {
          uniquePaths.set(pathKey, { params: p, score: maxTotalScore, frequency: 1 });
        }
      }
    }

    const canCache = isRefinement || (!isRefinement && sourceCount <= OUTPUT_LIMIT);
    if (canCache && survivingIndices.length <= OUTPUT_LIMIT) {
      updateCache({
        query: queryInput,
        contextStr: currentContextStr,
        indices: new Int32Array(survivingIndices),
      });
    } else {
      updateCache({ query: '', contextStr: '', indices: null });
    }

    const allPaths = Array.from(uniquePaths.values());
    allPaths.sort((a, b) => b.score - a.score);
    suggestions = allPaths.slice(0, 20).map((p) => ({
      params: p.params,
      score: p.score,
      count: p.frequency,
      countIsLowerBound: isSampled,
    }));
  } else {
    const topCombo = tokenCandidateSets.map((set) => set[0]);
    let totalScore = 0;
    const uniqueCombo: Param[] = [];
    const seenKeys = new Set<string>();
    for (let i = 0; i < topCombo.length; i++) {
      const p = topCombo[i];
      if (!seenKeys.has(p.key)) {
        uniqueCombo.push(p);
        seenKeys.add(p.key);
      }
      totalScore += scoreParamAny(p, tokens[i]);
    }
    suggestions = [{ params: uniqueCombo, score: totalScore }];
  }
  return suggestions;
}
