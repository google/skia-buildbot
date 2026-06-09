import { Param, TraceData, WasmExports, Query } from './worker-types';
import { scoreParamAny, fuzzyScore } from '../fuzzy';
import { scanWasmBatch } from './wasm_utils';

export interface SuggestionResult {
  params: Param[];
  score: number;
  count?: number;
  countIsLowerBound?: boolean;
}

export interface SearchCache {
  query: string;
  contextStr: string;
  indices: Int32Array | null;
}

export async function computeSuggestions(
  queryInput: string,
  currentQuery: Query,
  params: Param[],
  availableParams: Param[] | null,
  traceData: TraceData | null,
  shouldAbort: () => boolean,
  wasmFilter: WasmExports | null = null
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
  const tokenCandidateSets = tokens.map((token) => {
    const eqIdx = token.indexOf('=');
    const vPartCheck = eqIdx !== -1 ? token.substring(eqIdx + 1) : token;
    const hasGlobChar =
      vPartCheck.includes('*') || vPartCheck.includes('?') || vPartCheck.includes(',');
    const isGlobSearch = hasGlobChar;

    if (isGlobSearch) {
      const kPart = eqIdx !== -1 ? token.substring(0, eqIdx) : '';
      const vPart = eqIdx !== -1 ? token.substring(eqIdx + 1) : token;

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
        return matches.slice(0, 50).map((m) => ({ p: m.p, score: m.score }));
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
      const cutoff = bestScore >= 100000 ? 50 : bestScore - 40;
      const qualified = matches.filter((m) => m.score >= cutoff);
      return qualified.slice(0, 1000).map((m) => ({ p: m.p, score: m.score }));
    }
    return [];
  });

  if (tokenCandidateSets.some((set) => set.length === 0)) {
    return [];
  }

  // Generate combinations (Cartesian product) of candidates
  let combinations: { params: Param[]; score: number }[] = [];

  if (tokens.length === 1) {
    combinations = tokenCandidateSets[0].map((c) => ({
      params: [c.p],
      score: c.score,
    }));
  } else {
    const sets = tokenCandidateSets.map((set) => set.slice(0, 10));
    const tempResults: { params: Param[]; score: number }[] = [];

    function generateCombinations(setIndex: number, currentParams: Param[], currentScore: number) {
      if (setIndex === sets.length) {
        if (currentParams.length > 0) {
          tempResults.push({
            params: [...currentParams],
            score: currentScore,
          });
        }
        return;
      }

      for (const candidate of sets[setIndex]) {
        if (currentParams.some((p) => p.key === candidate.p.key)) {
          continue;
        }
        currentParams.push(candidate.p);
        generateCombinations(setIndex + 1, currentParams, currentScore + candidate.score);
        currentParams.pop();
      }
    }

    generateCombinations(0, [], 0);
    combinations = tempResults;

    // Also include individual candidates as fallback
    const seenKeys = new Set<string>();
    for (const set of tokenCandidateSets) {
      for (const candidate of set) {
        const key = `${candidate.p.key}=${candidate.p.value}`;
        if (!seenKeys.has(key)) {
          seenKeys.add(key);
          combinations.push({
            params: [candidate.p],
            score: candidate.score,
          });
        }
      }
    }
  }

  const OUTPUT_LIMIT = 10000;
  const MAX_KEYS = 50;

  function getQueryPtr(td: any): number {
    const bitsetBufferSize = td.bitsetSize * (MAX_KEYS + 1);
    const outputSize = OUTPUT_LIMIT * 4;
    const bitsetOffset = td.matchingParamsPtr;
    const bitsetSizeBytes = bitsetBufferSize * 4;
    const outputPtrRaw = bitsetOffset + bitsetSizeBytes;
    const outputPtr = (outputPtrRaw + 3) & ~3;
    const queryPtr = outputPtr + outputSize;
    return queryPtr;
  }

  const keyToIndex = new Map<string, number>();
  currentQueryKeys.forEach((k, i) => keyToIndex.set(k, i));

  const suggestions: SuggestionResult[] = [];

  // Phase 1: O(1) Estimation for fast sorting of all combinations
  for (const comb of combinations) {
    if (shouldAbort()) return null;
    let estCount = 0;
    if (traceData) {
      let minCount = traceData.numTraces;
      for (const p of comb.params) {
        let bitsetOffset = 0;
        if (keyToIndex.has(p.key)) {
          const k = keyToIndex.get(p.key)!;
          bitsetOffset = (k + 1) * traceData.bitsetSize;
        }
        const pCount = traceData.matchingParams[bitsetOffset + p.id];
        if (pCount < minCount) {
          minCount = pCount;
        }
      }
      estCount = minCount;
    }

    suggestions.push({
      params: comb.params,
      score: comb.score,
      count: estCount,
    } as any);
  }

  // Sort suggestions by their fuzzy score in descending order
  suggestions.sort((a, b) => b.score - a.score);

  // Phase 2: O(N) exact Wasm-accelerated counting scan ONLY for the top 20 suggestions
  const topSuggestions = suggestions.slice(0, 20);
  const finalSuggestions: SuggestionResult[] = [];

  // Resolve currentQuery values to IDs for fast matching
  const currentQueryParamIdsMap = new Map<string, number[]>();
  if (traceData) {
    for (const [key, values] of Object.entries(currentQuery)) {
      const ids: number[] = [];
      for (const val of values) {
        const hasGlob = val.includes('*') || val.includes('?');
        if (hasGlob) {
          const escaped = val.replace(/[.+^${}()|[\]\\]/g, '\\$&');
          const pattern = '^' + escaped.replace(/\*/g, '.*').replace(/\?/g, '.') + '$';
          const regex = new RegExp(pattern, 'i');
          for (const p of params) {
            if (p.key === key && regex.test(p.value)) {
              ids.push(p.id);
            }
          }
        } else {
          const pm = params.find((item) => item.key === key && item.value === val);
          if (pm) {
            ids.push(pm.id);
          }
        }
      }
      if (ids.length > 0) {
        currentQueryParamIdsMap.set(key, ids);
      }
    }
  }

  for (const s of topSuggestions) {
    if (shouldAbort()) return null;

    if (traceData && wasmFilter && s.params.length > 1) {
      const queryMap = new Map<string, number[]>();

      // Add currentQueryParamIds
      for (const [key, ids] of currentQueryParamIdsMap) {
        queryMap.set(key, [...ids]);
      }

      // Add suggestion params
      for (const p of s.params) {
        const existing = queryMap.get(p.key) || [];
        if (!existing.includes(p.id)) {
          existing.push(p.id);
        }
        queryMap.set(p.key, existing);
      }

      const serializedQuery: number[] = [];
      serializedQuery.push(queryMap.size);

      let totalQueryValues = 0;
      for (const [_, ids] of queryMap) {
        serializedQuery.push(ids.length);
        serializedQuery.push(...ids);
        totalQueryValues += ids.length;
      }

      const queryPtr = getQueryPtr(traceData);
      const queryView = new Int32Array(traceData.memory.buffer, queryPtr, serializedQuery.length);
      queryView.set(serializedQuery);

      const count = await scanWasmBatch(
        wasmFilter,
        traceData,
        queryPtr,
        serializedQuery.length,
        OUTPUT_LIMIT,
        totalQueryValues,
        shouldAbort
      );

      if (count === -1) return null; // Aborted

      s.count = count;
      s.countIsLowerBound = false; // Exact count!
    } else {
      s.countIsLowerBound = false; // Single parameters are already 100% exact
    }

    if (!traceData || s.count! > 0) {
      finalSuggestions.push(s);
    }
  }

  // Filter: If combinations (length > 1) are present, show NOTHING but the combinations!
  const hasCombinations = finalSuggestions.some((s) => s.params.length > 1);
  if (hasCombinations) {
    return finalSuggestions.filter((s) => s.params.length > 1);
  }

  return finalSuggestions;
}
