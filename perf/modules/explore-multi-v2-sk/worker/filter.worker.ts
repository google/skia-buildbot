import { Param, TraceData, WasmExports, Query } from './worker-types';
import { computeSuggestions } from './suggestion_engine';
import { runWasmBatch } from './wasm_utils';
import { filterTraces } from './worker_logic';
import { TraceDatabase } from '../db';

const COMMON_PARAM_ID = 0;

export interface WorkerConfig {
  metaUrl: string;
  paramsUrlTemplate: string;
  wasmUrlTemplate: string;
  tracesUrlTemplate: string;
}

interface Meta {
  version: string;
  stride: number;
  count: number;
  commonParams?: Record<string, string>;
}

interface LoadedAssets {
  params: Param[];
  wasmModule: WebAssembly.Module;
  tracesBuffer: ArrayBuffer;
}

async function fetchAndCacheAssets(
  config: WorkerConfig,
  version: string,
  db: TraceDatabase,
  postStatus: (msg: string) => void
): Promise<LoadedAssets> {
  const cachedParams = await db.get(`static:params:${version}`);
  const cachedWasm = await db.get(`static:wasm:${version}`);
  const cachedTraces = await db.get(`static:traces:${version}`);

  if (cachedParams && cachedWasm && cachedTraces) {
    console.log('Worker: Serving static files from cache');
    const params = cachedParams as Param[];
    const wasmModule = await WebAssembly.compile(cachedWasm);
    const tracesBuffer = cachedTraces as ArrayBuffer;
    return { params, wasmModule, tracesBuffer };
  }

  console.log('Worker: Cache cache miss, fetching assets');
  postStatus('Downloading performance assets...');

  const paramsUrl = config.paramsUrlTemplate.replace('{version}', version);
  const wasmUrl = config.wasmUrlTemplate.replace('{version}', version);
  const tracesUrl = config.tracesUrlTemplate.replace('{version}', version);

  const [paramsResp, tracesResp, wasmResp] = await Promise.all([
    fetch(paramsUrl),
    fetch(tracesUrl),
    fetch(wasmUrl),
  ]);

  if (!paramsResp.ok) throw new Error(`Failed to fetch params: ${paramsResp.statusText}`);
  if (!tracesResp.ok) throw new Error(`Failed to fetch traces: ${tracesResp.statusText}`);
  if (!wasmResp.ok) throw new Error(`Failed to fetch Wasm: ${wasmResp.statusText}`);

  const params = (await paramsResp.json()) as Param[];
  const tracesBuffer = await tracesResp.arrayBuffer();
  const wasmClone = wasmResp.clone();

  const [wasmInstantiated, wasmBytes] = await Promise.all([
    WebAssembly.instantiateStreaming(wasmResp, {
      env: { abort: () => console.error('Wasm abort') },
    }),
    wasmClone.arrayBuffer(),
  ]);

  const wasmModule = wasmInstantiated.module;

  postStatus('Saving assets to local cache...');
  await Promise.all([
    db.set(`static:params:${version}`, params),
    db.set(`static:wasm:${version}`, wasmBytes),
    db.set(`static:traces:${version}`, tracesBuffer),
  ]);

  return { params, wasmModule, tracesBuffer };
}

interface ProcessedParams {
  flatParams: { key: string; value: string; count: number }[];
  groupedParams: Record<string, { value: string; count: number }[]>;
}

function processInitialParams(meta: Meta, params: Param[]): ProcessedParams {
  const flatParams = params.map((p) => ({ key: p.key, value: p.value, count: 0 }));
  const groupedParams: Record<string, { value: string; count: number }[]> = {};

  params.forEach((p) => {
    if (!groupedParams[p.key]) groupedParams[p.key] = [];
    groupedParams[p.key].push({ value: p.value, count: 0 });
  });

  if (meta.commonParams) {
    for (const [key, value] of Object.entries(meta.commonParams as Record<string, string>)) {
      flatParams.push({ key, value, count: meta.count });
      if (!groupedParams[key]) groupedParams[key] = [];
      groupedParams[key].push({ value, count: meta.count });
    }
  }

  Object.keys(groupedParams).forEach((k) =>
    groupedParams[k].sort((a, b) => a.value.localeCompare(b.value))
  );

  return { flatParams, groupedParams };
}

function serializeQuery(filteredQueryEntries: [string, string[]][], params: Param[]): number[] {
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
            // TODO(performance): This linear search over the entire params array for each wildcard value
            // in a query can be O(N*M). While fine for now, if performance becomes an issue,
            // consider a more optimized lookup for wildcards.
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
  return serializedQuery;
}

interface ReconstructedResult {
  index: number;
  params: Param[];
}

function reconstructResults(
  unionIndices: number[],
  traceData: TraceData,
  params: Param[],
  commonParams: Record<string, string>
): ReconstructedResult[] {
  const displayCount = Math.min(unionIndices.length, OUTPUT_LIMIT);
  const results: ReconstructedResult[] = [];

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
    for (const [key, value] of Object.entries(commonParams)) {
      traceParams.push({ id: COMMON_PARAM_ID, key, value });
    }

    results.push({ index: traceIndex, params: traceParams });
  }
  return results;
}

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

async function loadData(config: WorkerConfig) {
  const postStatus = (message: string) => {
    self.postMessage({ type: 'STATUS', payload: { message } });
  };
  const postProgress = (name: string, loaded: number, total: number) => {
    self.postMessage({ type: 'PROGRESS', payload: { name, loaded, total } });
  };

  try {
    postStatus('Connecting to engine...');

    // 1. Fetch meta.json
    postStatus('Loading performance database...');
    const metaResp = await fetch(config.metaUrl + '?t=' + Date.now());
    if (!metaResp.ok) throw new Error(`Failed to fetch meta: ${metaResp.statusText}`);
    const meta = (await metaResp.json()) as Meta;
    const { version } = meta;

    const db = new TraceDatabase();

    // 2. Fetch and Cache Assets
    const { params, wasmModule, tracesBuffer } = await fetchAndCacheAssets(
      config,
      version,
      db,
      postStatus
    );

    // 3. Process Params & Post PARAMS_READY
    postStatus('Processing dimensions...');
    paramsOnlyData = { params };

    const { flatParams, groupedParams } = processInitialParams(meta, params);

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

    // 4. Instantiate Wasm Instance with Memory
    postStatus('Initializing WebAssembly analyzer...');

    const stride = meta.stride;
    const count = meta.count;
    const traceDataSize = count * stride * 2;
    const maxParamId = params.reduce((max: number, p) => (p.id > max ? p.id : max), 0);

    const bitsetSize = maxParamId + 1;
    const bitsetElements = bitsetSize * (MAX_KEYS + 1);
    const bitsetSizeBytes = bitsetElements * 4;
    const outputSize = OUTPUT_LIMIT * 4;

    const instance = await WebAssembly.instantiate(wasmModule, {
      env: { abort: () => console.error('Wasm abort') },
    });
    const wasmFilter = instance.exports as unknown as WasmExports;
    const memory = wasmFilter.memory;
    const heapBase = wasmFilter.heap_base.value as number;

    const dataPtr = heapBase;
    const bitsetPtr = (dataPtr + traceDataSize + 3) & ~3;
    const outputPtrRaw = bitsetPtr + bitsetSizeBytes;
    const outputPtr = (outputPtrRaw + 3) & ~3;
    const queryPtr = outputPtr + outputSize;

    const totalNeededBytes = queryPtr + QUERY_BUFFER_SIZE;
    const currentBytes = memory.buffer.byteLength;

    if (totalNeededBytes > currentBytes) {
      const neededPages = Math.ceil((totalNeededBytes - currentBytes) / 65536);
      memory.grow(neededPages);
    }

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

    // 5. Calculate Global Counts
    postStatus('Optimizing query index...');
    const queryView = new Int32Array(memory.buffer, queryPtr, 10);
    queryView[0] = 0; // 0 keys

    postProgress('Calculating Global Counts...', 0, 1);
    await runWasmBatch(wasmFilter, traceData, queryPtr, 1, OUTPUT_LIMIT, 0, () => false);

    const globalCounts = new Int32Array(bitsetSize);
    globalCounts.set(matchingParams.subarray(0, bitsetSize));

    loadedData = {
      params,
      traceData,
      wasmFilter,
      globalCounts,
      commonParams: meta.commonParams || {},
    };

    self.postMessage({ type: 'READY' });
  } catch (error: unknown) {
    console.error('Worker initialization failed:', error);
    const message = error instanceof Error ? error.message : String(error);
    self.postMessage({ type: 'ERROR', payload: { message } });
  }
}

async function handleFilter(queries: Query[], requestId: number, numUserQueries: number = 1) {
  try {
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
        const groupedParams: Record<string, { value: string; count: number }[]> = {};
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
            requestId,
          },
        });
      }
      return;
    }

    const { wasmFilter, params, traceData, globalCounts, commonParams } = loadedData;

    await filterTraces(queries, traceData, async (queryString) => {
      const resp = await fetch(`/_/wasm/query_traces?query=${encodeURIComponent(queryString)}`);
      if (!resp.ok) {
        throw new Error(`Failed to fetch traces: ${resp.statusText}`);
      }
      return await resp.arrayBuffer();
    });

    let totalFilteredCount = 0;
    const finalTraceIndices = new Set<number>();
    const queryResults = new Array(queries.length);

    for (let currentQueryIdx = queries.length - 1; currentQueryIdx >= 0; currentQueryIdx--) {
      if (requestId !== latestFilterRequestId) return; // Check interrupt

      const query = queries[currentQueryIdx];
      traceData.matchingParams.fill(0);
      const queryEntries = Object.entries(query);
      const queryKeysOrder = queryEntries.map(([k]) => k);

      let count = 0;
      const filteredQueryEntries: [string, string[]][] = [];
      let alwaysFalse = false;

      for (const [key, values] of queryEntries) {
        if (commonParams && commonParams[key]) {
          const commonVal = commonParams[key];
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
        const serializedQuery = serializeQuery(filteredQueryEntries, params);

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

      queryResults[currentQueryIdx] = {
        availableParams: flatParams,
        paramsByKey: groupedParams,
        queryKeysOrder,
      };
    }

    const unionIndices = Array.from(finalTraceIndices);
    unionIndices.sort((a, b) => a - b);
    const results = reconstructResults(unionIndices, traceData, params, commonParams);

    self.postMessage({
      type: 'RESULT',
      payload: {
        filteredCount: totalFilteredCount,
        results,
        queryResults,
        requestId,
      },
    });
  } catch (error: unknown) {
    console.error('Worker filter failed:', error);
    const message = error instanceof Error ? error.message : String(error);
    self.postMessage({ type: 'ERROR', payload: { message } });
  }
}

async function handleSuggest(
  queryInput: string,
  currentQuery: Query,
  requestId: number,
  idx: number,
  availableParams?: Param[]
) {
  try {
    if (requestId !== latestSuggestRequestId) return;

    const params = loadedData ? loadedData.params : paramsOnlyData?.params;
    if (!params) return;

    const suggestions = await computeSuggestions(
      queryInput,
      currentQuery,
      params,
      availableParams || null,
      loadedData?.traceData ?? null,
      () => requestId !== latestSuggestRequestId,
      loadedData?.wasmFilter ?? null
    );

    if (suggestions && requestId === latestSuggestRequestId) {
      self.postMessage({ type: 'SUGGEST_RESULT', payload: suggestions, requestId, idx });
    }
  } catch (error: unknown) {
    console.error('Worker suggest failed:', error);
    const message = error instanceof Error ? error.message : String(error);
    self.postMessage({ type: 'ERROR', payload: { message } });
  }
}

self.onmessage = (e: MessageEvent) => {
  if (e.data.type === 'INIT') {
    const config = e.data.payload as WorkerConfig;
    void loadData(config);
  } else if (e.data.type === 'FILTER') {
    latestFilterRequestId = e.data.requestId || latestFilterRequestId;
    const queries = e.data.queries || [e.data.query];
    void handleFilter(queries, e.data.requestId || 0, e.data.numUserQueries || 1);
  } else if (e.data.type === 'GET_RANDOM_TRACE') {
    if (!loadedData) {
      self.postMessage({ type: 'RANDOM_TRACE_RESULT', payload: null });
      return;
    }
    const { traceData, params, wasmFilter, globalCounts, commonParams } = loadedData;
    const query: Record<string, string[]> = {};
    let currentMatchCount = traceData.numTraces;
    const selectedKeys = new Set<string>();

    for (let iter = 0; iter < 10; iter++) {
      if (currentMatchCount <= 1) {
        break;
      }

      let counts: Int32Array;
      let bitsetOffset = 0;

      if (selectedKeys.size === 0) {
        counts = globalCounts;
        bitsetOffset = 0;
      } else {
        const serializedQuery: number[] = [];
        serializedQuery.push(selectedKeys.size);

        const queryEntries = Object.entries(query);
        for (const [key, values] of queryEntries) {
          const ids: number[] = [];
          for (const v of values) {
            const foundParam = params.find((param) => param.key === key && param.value === v);
            if (foundParam) ids.push(foundParam.id);
          }
          serializedQuery.push(ids.length);
          serializedQuery.push(...ids);
        }

        const queryPtr = getQueryPtr(traceData);
        const queryView = new Int32Array(traceData.memory.buffer, queryPtr, serializedQuery.length);
        queryView.set(serializedQuery);

        traceData.matchingParams.fill(0);

        const count = wasmFilter.filterTraces(
          traceData.numTraces,
          0,
          traceData.stride,
          queryPtr,
          serializedQuery.length,
          traceData.dataPtr,
          traceData.outPtr,
          0,
          OUTPUT_LIMIT,
          traceData.matchingParamsPtr,
          traceData.bitsetSize
        );

        if (count <= 0) {
          break;
        }
        currentMatchCount = count;
        if (currentMatchCount <= 1) {
          break;
        }

        counts = traceData.matchingParams;
        bitsetOffset = selectedKeys.size * traceData.bitsetSize;
      }

      // Find the parameter with the highest trace count that narrows it down.
      let bestParam: Param | null = null;
      let maxCount = 0;

      for (let i = 0; i < params.length; i++) {
        const p = params[i];
        if (selectedKeys.has(p.key)) continue;

        const pCount = counts[bitsetOffset + p.id];
        if (pCount > 0 && pCount < currentMatchCount) {
          if (pCount > maxCount) {
            maxCount = pCount;
            bestParam = p;
          }
        }
      }

      if (bestParam) {
        query[bestParam.key] = [bestParam.value];
        selectedKeys.add(bestParam.key);
        currentMatchCount = maxCount;
      } else {
        // Narrow down to 1 trace using the first matched trace!
        const matchedTraceIndex = traceData.filteredTraceIndices[0];
        const offset = matchedTraceIndex * traceData.stride;
        for (let j = 0; j < traceData.stride; j++) {
          const pid = traceData.paramSets[offset + j];
          if (pid === 0) break;
          const p = params[pid - 1];
          if (p && !selectedKeys.has(p.key)) {
            query[p.key] = [p.value];
            selectedKeys.add(p.key);
          }
        }
        break;
      }
    }

    if (commonParams) {
      for (const [key, value] of Object.entries(commonParams)) {
        query[key] = [value as string];
      }
    }
    self.postMessage({ type: 'RANDOM_TRACE_RESULT', payload: query });
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
