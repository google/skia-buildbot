import { Param, TraceData, WasmExports, Query } from './worker-types';
import { computeSuggestions, SearchCache } from './suggestion_engine';
import { runWasmBatch, scanWasmBatch, yieldToMain } from './wasm_utils';
import { filterTraces } from './worker_logic';

console.log('Worker: filter.worker.ts loaded top-level');

let loadedData: {
  params: Param[];
  traceData: TraceData;
  wasmFilter: WasmExports;
  globalCounts: Int32Array;
  commonParams: Record<string, string>;
} | null = null;
let paramsOnlyData: { params: Param[] } | null = null;

// Constants
const OUTPUT_LIMIT = 10000;
const MAX_KEYS = 50;
const QUERY_BUFFER_SIZE = 4 * 1024 * 1024; // Increased to 4MB to accommodate large bitsetSize and prevent Wasm overflow

// Cache State
const searchCaches: Record<number, SearchCache> = {};

// State for Interruptibility
let latestFilterRequestId = 0;
let latestSuggestRequestId = 0;

function getQueryPtr(traceData: TraceData): number {
  const bitsetBufferSize = traceData.bitsetSize * (MAX_KEYS + 1);
  const outputSize = OUTPUT_LIMIT * 4;
  const bitsetOffset = traceData.matchingParamsPtr;
  const bitsetSizeBytes = bitsetBufferSize * 4;
  const outputPtrRaw = bitsetOffset + bitsetSizeBytes;
  const outputPtr = (outputPtrRaw + 3) & ~3;
  const queryPtr = outputPtr + outputSize;
  return queryPtr;
}

async function loadData(
  meta: any,
  params: Param[],
  wasmBuffer: ArrayBuffer,
  tracesBuffer: ArrayBuffer
) {
  console.log('Worker: loadData started with provided data');
  try {
    const postProgress = (name: string, loaded: number, total: number) => {
      self.postMessage({ type: 'PROGRESS', payload: { name, loaded, total } });
    };

    const { stride, count, commonParams } = meta;
    console.log('Worker: Using meta:', meta);
    console.log('Worker: Using params, count:', params.length);

    let maxParamId = 0;
    for (const p of params) {
      if (p.id > maxParamId) maxParamId = p.id;
    }

    console.log(
      'Worker loadData: maxParamId =',
      maxParamId,
      'bitsetSize =',
      maxParamId + 1,
      'QUERY_BUFFER_SIZE =',
      QUERY_BUFFER_SIZE
    );

    paramsOnlyData = { params };

    const flatParams = params.map((p) => ({ key: p.key, value: p.value, count: 0 }));
    const groupedParams: Record<string, any[]> = {};
    params.forEach((p) => {
      if (!groupedParams[p.key]) groupedParams[p.key] = [];
      groupedParams[p.key].push({ value: p.value, count: 0 }); // Init with 0
    });

    // Add common params to available params
    if (commonParams) {
      for (const [key, value] of Object.entries(commonParams as Record<string, string>)) {
        flatParams.push({ key, value, count: count });
        if (!groupedParams[key]) groupedParams[key] = [];
        groupedParams[key].push({ value, count: count });
      }
    }

    Object.keys(groupedParams).forEach((k) =>
      groupedParams[k].sort((a: any, b: any) => a.value.localeCompare(b.value))
    );

    self.postMessage({
      type: 'PARAMS_READY',
      payload: {
        availableParams: flatParams,
        paramsByKey: groupedParams,
        filteredCount: 0,
        results: [],
        queryKeysOrder: [],
      },
    });

    const traceDataSize = count * stride * 2;

    const bitsetSize = maxParamId + 1;
    const bitsetElements = bitsetSize * (MAX_KEYS + 1);
    const bitsetSizeBytes = bitsetElements * 4;
    const outputSize = OUTPUT_LIMIT * 4;

    console.log('Worker: Instantiating Wasm module');
    const wasmModuleResult = await WebAssembly.instantiate(wasmBuffer, {
      env: {
        abort: () => console.error('Wasm abort'),
      },
    });
    const wasmFilter = wasmModuleResult.instance.exports as unknown as WasmExports;
    const memory = wasmFilter.memory;
    const heapBase = wasmFilter.heap_base.value as number;

    const dataPtr = heapBase;
    const bitsetPtr = (dataPtr + traceDataSize + 3) & ~3; // Align to 4 bytes
    const outputPtrRaw = bitsetPtr + bitsetSizeBytes;
    const outputPtr = (outputPtrRaw + 3) & ~3;
    const queryPtr = outputPtr + outputSize;

    const totalNeededBytes = queryPtr + QUERY_BUFFER_SIZE;
    const currentBytes = memory.buffer.byteLength;

    if (totalNeededBytes > currentBytes) {
      const neededPages = Math.ceil((totalNeededBytes - currentBytes) / 65536);
      memory.grow(neededPages);
    }

    console.log('Worker: Fetching traces.bin');
    console.log('Worker: Using provided tracesBuffer, size:', tracesBuffer.byteLength);

    const paramSets = new Uint16Array(memory.buffer, dataPtr, count * stride);
    paramSets.set(new Uint16Array(tracesBuffer));

    const matchingParams = new Int32Array(memory.buffer, bitsetPtr, bitsetElements);
    const filteredTraceIndices = new Int32Array(memory.buffer, outputPtr, OUTPUT_LIMIT);

    const traceData = {
      memory,
      paramSets,
      matchingParams,
      filteredTraceIndices,
      stride,
      numTraces: count,
      maxParamId,
      dataPtr,
      matchingParamsPtr: bitsetPtr,
      outPtr: outputPtr,
      bitsetSize,
    };

    // Calculate Global Counts
    const queryView = new Int32Array(memory.buffer, queryPtr, 10);
    queryView[0] = 0; // 0 keys

    postProgress('Calculating Global Counts...', 0, 1);

    await runWasmBatch(
      wasmFilter,
      traceData,
      queryPtr,
      1,
      OUTPUT_LIMIT,
      0, // totalQueryValues
      () => false
    );

    // Copy counts
    const globalCounts = new Int32Array(bitsetSize);
    globalCounts.set(matchingParams.subarray(0, bitsetSize));

    loadedData = {
      params,
      traceData,
      wasmFilter,
      globalCounts,
      commonParams: commonParams || {},
    };

    self.postMessage({ type: 'READY' });
  } catch (e: any) {
    self.postMessage({ type: 'ERROR', payload: e.message || String(e) });
  }
}

async function handleFilter(queries: Query[], requestId: number, numUserQueries: number = 1) {
  if (requestId !== latestFilterRequestId) return;
  console.log('Worker handleFilter queries:', queries);

  if (!loadedData) {
    console.log('Worker handleFilter: loadedData is null');
    if (paramsOnlyData) {
      const flatParams = paramsOnlyData.params.map((p) => ({
        key: p.key,
        value: p.value,
        count: 0,
      }));
      const groupedParams: Record<string, any[]> = {};
      paramsOnlyData.params.forEach((p) => {
        if (!groupedParams[p.key]) groupedParams[p.key] = [];
        groupedParams[p.key].push({ value: p.value, count: 0 });
      });

      const queryResults = queries.map((q) => ({
        availableParams: flatParams,
        paramsByKey: groupedParams,
        queryKeysOrder: Object.keys(q),
      }));

      self.postMessage({
        type: 'RESULT',
        payload: {
          filteredCount: 0,
          results: [],
          queryResults,
        },
      });
    }
    return;
  }

  await filterTraces(queries, loadedData.traceData, async (queryString) => {
    const resp = await fetch(`/_/wasm/query_traces?query=${encodeURIComponent(queryString)}`);
    if (!resp.ok) {
      throw new Error(`Failed to fetch traces: ${resp.statusText}`);
    }
    return await resp.arrayBuffer();
  });

  const { wasmFilter, params, traceData, globalCounts } = loadedData;

  let totalFilteredCount = 0;
  const finalTraceIndices = new Set<number>();
  const queryResults: any[] = [];

  for (let currentQueryIdx = 0; currentQueryIdx < queries.length; currentQueryIdx++) {
    if (requestId !== latestFilterRequestId) return; // Check interrupt

    const query = queries[currentQueryIdx];
    traceData.matchingParams.fill(0);
    const queryEntries = Object.entries(query);
    const queryKeysOrder = queryEntries.map(([k]) => k);

    let count = 0;
    const filteredQueryEntries: [string, string[]][] = [];
    let alwaysFalse = false;

    for (const [key, values] of queryEntries) {
      if (loadedData && loadedData.commonParams && loadedData.commonParams[key]) {
        const commonVal = loadedData.commonParams[key];
        const matches = values.some((v) => {
          const parts = v.split(',').map((s) => s.trim());
          return parts.some((part) => {
            if (part.includes('*') || part.includes('?')) {
              try {
                const escaped = part.replace(/[.+^${}()|[\]\\]/g, '\\$&');
                const pattern = '^' + escaped.replace(/\*/g, '.*').replace(/\?/g, '.') + '$';
                const regex = new RegExp(pattern, 'i');
                return regex.test(commonVal);
              } catch (_e) {
                return false;
              }
            }
            return part === commonVal;
          });
        });
        if (!matches) {
          alwaysFalse = true;
          break;
        }
        continue; // All traces have this value, no need to filter in Wasm
      }
      filteredQueryEntries.push([key, values]);
    }

    if (alwaysFalse) {
      count = 0;
      console.log('Worker: Query contradicts commonParams. 0 matches.');
    } else if (filteredQueryEntries.length === 0) {
      traceData.matchingParams.set(globalCounts, 0);
      count = traceData.numTraces;
      const limit = Math.min(count, OUTPUT_LIMIT);
      for (let k = 0; k < limit; k++) {
        traceData.filteredTraceIndices[k] = k;
      }
    } else {
      const serializedQuery: number[] = [];
      serializedQuery.push(filteredQueryEntries.length);

      for (const [key, values] of filteredQueryEntries) {
        const ids: number[] = [];
        for (const v of values) {
          const parts = v
            .split(',')
            .map((s) => s.trim())
            .filter(Boolean);
          for (const part of parts) {
            if (part.includes('*') || part.includes('?')) {
              try {
                const escaped = part.replace(/[.+^${}()|[\]\\]/g, '\\$&');
                const pattern = '^' + escaped.replace(/\*/g, '.*').replace(/\?/g, '.') + '$';
                const regex = new RegExp(pattern, 'i');
                for (let i = 0; i < params.length; i++) {
                  const p = params[i];
                  if (p.key === key && regex.test(p.value)) ids.push(p.id);
                }
              } catch (_e) {}
            } else {
              const foundParam = params.find((param) => param.key === key && param.value === part);
              if (foundParam) ids.push(foundParam.id);
            }
          }
        }
        const uniqueIds = Array.from(new Set(ids));
        serializedQuery.push(uniqueIds.length);
        serializedQuery.push(...uniqueIds);
      }

      console.log('Worker serializedQuery:', serializedQuery);
      const queryPtr = getQueryPtr(traceData);
      const queryView = new Int32Array(traceData.memory.buffer, queryPtr, serializedQuery.length);
      queryView.set(serializedQuery);

      let totalQueryValues = 0;
      for (const [_, values] of queryEntries) {
        totalQueryValues += values.length;
      }

      count = await runWasmBatch(
        wasmFilter,
        traceData,
        queryPtr,
        serializedQuery.length,
        OUTPUT_LIMIT,
        totalQueryValues,
        () => requestId !== latestFilterRequestId
      );

      console.log('Worker Wasm count:', count);
      if (count === -1) return; // Aborted

      if (queryEntries.length === 1) {
        const bitsetOffset = traceData.bitsetSize;
        traceData.matchingParams.set(globalCounts, bitsetOffset);
      }
    }

    totalFilteredCount += count;
    if (currentQueryIdx < numUserQueries) {
      const displayCount = Math.min(count, OUTPUT_LIMIT);
      for (let i = 0; i < displayCount; i++) {
        finalTraceIndices.add(traceData.filteredTraceIndices[i]);
      }
    }

    const { matchingParams, bitsetSize } = traceData;
    const keyToIndex = new Map<string, number>();
    queryKeysOrder.forEach((k, i) => keyToIndex.set(k, i));

    const flatParams: { key: string; value: string; count: number }[] = [];
    const groupedParams: Record<string, { value: string; count: number }[]> = {};

    for (let i = 0; i < params.length; i++) {
      const p = params[i];
      let bitsetOffset = 0;
      if (keyToIndex.has(p.key)) {
        const k = keyToIndex.get(p.key)!;
        bitsetOffset = (k + 1) * bitsetSize;
      }
      const pCount = matchingParams[bitsetOffset + p.id];
      if (pCount > 0) {
        flatParams.push({ key: p.key, value: p.value, count: pCount });
        if (!groupedParams[p.key]) groupedParams[p.key] = [];
        groupedParams[p.key].push({ value: p.value, count: pCount });
      }
    }

    Object.keys(groupedParams).forEach((k) =>
      groupedParams[k].sort((a, b) => a.value.localeCompare(b.value))
    );

    queryResults.push({
      availableParams: flatParams,
      paramsByKey: groupedParams,
      queryKeysOrder,
    });
  }

  const unionIndices = Array.from(finalTraceIndices);
  unionIndices.sort((a, b) => a - b);
  const displayCount = Math.min(unionIndices.length, OUTPUT_LIMIT);
  const results = [];

  for (let i = 0; i < displayCount; i++) {
    const traceIndex = unionIndices[i];
    const offset = traceIndex * traceData.stride;
    const traceParams: Param[] = [];
    for (let j = 0; j < traceData.stride; j++) {
      const pid = traceData.paramSets[offset + j];
      if (pid === 0) break;
      const p = params[pid - 1];
      if (p) traceParams.push(p);
    }

    // Append common params
    if (loadedData && loadedData.commonParams) {
      for (const [key, value] of Object.entries(loadedData.commonParams)) {
        traceParams.push({ id: 0, key, value });
      }
    }

    results.push({ index: traceIndex, params: traceParams });
  }

  self.postMessage({
    type: 'RESULT',
    payload: {
      filteredCount: totalFilteredCount,
      results,
      queryResults,
    },
  });
}

async function handleSuggest(
  queryInput: string,
  currentQuery: Query,
  requestId: number,
  idx: number,
  availableParams?: Param[]
) {
  if (requestId !== latestSuggestRequestId) return;

  const params = loadedData ? loadedData.params : paramsOnlyData?.params;
  if (!params) return;

  const suggestions = await computeSuggestions(
    queryInput,
    currentQuery,
    params,
    availableParams || null,
    loadedData?.traceData ?? null,
    loadedData?.wasmFilter ?? null,
    searchCaches[idx] || { query: '', contextStr: '', indices: null },
    (cache: SearchCache) => {
      searchCaches[idx] = cache;
    },
    yieldToMain,
    () => requestId !== latestSuggestRequestId,
    getQueryPtr,
    scanWasmBatch
  );

  if (suggestions && requestId === latestSuggestRequestId) {
    self.postMessage({ type: 'SUGGEST_RESULT', payload: suggestions, requestId, idx });
  }
}

self.onmessage = (e: MessageEvent) => {
  if (e.data.type === 'INIT') {
    const { meta, params, wasmBuffer, tracesBuffer } = e.data.payload;
    void loadData(meta, params, wasmBuffer, tracesBuffer);
  } else if (e.data.type === 'FILTER') {
    latestFilterRequestId = e.data.requestId || latestFilterRequestId;
    const queries = e.data.queries || [e.data.query];
    void handleFilter(queries, e.data.requestId || 0, e.data.numUserQueries || 1);
  } else if (e.data.type === 'SUGGEST') {
    latestSuggestRequestId = e.data.requestId;
    void handleSuggest(
      e.data.query,
      e.data.currentQuery,
      e.data.requestId,
      e.data.idx,
      e.data.availableParams
    );
  }
};

// Notify orchestrator that worker is loaded and ready to receive messages.
self.postMessage({ type: 'LOADED' });
