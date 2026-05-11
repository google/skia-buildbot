import { expect } from 'chai';
import { computeSuggestions, SearchCache } from './suggestion_engine';
import { Param, Query, TraceData } from './worker-types';

describe('suggestion_engine fallback path', () => {
  const testParams: Param[] = [
    { id: 1, key: 'cpu', value: 'x86' },
    { id: 2, key: 'mode', value: 'release' },
    { id: 3, key: 'os', value: 'linux' },
    { id: 4, key: 'cpu', value: 'arm64' },
    { id: 5, key: 'mode', value: 'debug' },
  ];

  const dummyUpdateCache = (_cache: SearchCache) => {};
  const dummyYield = async () => {};
  const dummyShouldAbort = () => false;
  const dummyGetQueryPtr = (_data: any) => 0;
  const dummyScanWasmBatch = async () => 0;

  it('should suggest a single attribute (sanity check)', async () => {
    const queryInput = 'x86';
    const currentQuery: Query = {};

    const suggestions = await computeSuggestions(
      queryInput,
      currentQuery,
      testParams,
      null, // availableParams
      null, // traceData (forces fallback)
      null, // wasmFilter (forces fallback)
      { query: '', contextStr: '', indices: null },
      dummyUpdateCache,
      dummyYield,
      dummyShouldAbort,
      dummyGetQueryPtr,
      dummyScanWasmBatch
    );

    expect(suggestions).to.not.be.null;
    expect(suggestions!.length).to.equal(1);
    expect(suggestions![0].params.length).to.equal(1);
    expect(suggestions![0].params[0]).to.deep.equal({ id: 1, key: 'cpu', value: 'x86' });
  });

  it('should suggest a combination of multiple attributes separated by space', async () => {
    const queryInput = 'x86 release';
    const currentQuery: Query = {};

    const suggestions = await computeSuggestions(
      queryInput,
      currentQuery,
      testParams,
      null, // availableParams
      null, // traceData (forces fallback)
      null, // wasmFilter (forces fallback)
      { query: '', contextStr: '', indices: null },
      dummyUpdateCache,
      dummyYield,
      dummyShouldAbort,
      dummyGetQueryPtr,
      dummyScanWasmBatch
    );

    expect(suggestions).to.not.be.null;
    // Fallback path returns exactly 1 top combo suggestion if it finds it
    expect(suggestions!.length).to.equal(1);

    const params = suggestions![0].params;
    expect(params.length).to.equal(2);

    // We expect both cpu=x86 and mode=release to be in the suggestion
    const hasX86 = params.some((p) => p.key === 'cpu' && p.value === 'x86');
    const hasRelease = params.some((p) => p.key === 'mode' && p.value === 'release');

    expect(hasX86).to.be.true;
    expect(hasRelease).to.be.true;
  });
});

describe('suggestion_engine Wasm path JS logic', () => {
  const testParams: Param[] = [
    { id: 1, key: 'cpu', value: 'x86' },
    { id: 2, key: 'mode', value: 'release' },
    { id: 3, key: 'os', value: 'linux' },
  ];

  let traceData: TraceData;
  let memory: WebAssembly.Memory;
  const stride = 4;
  const bitsetSize = 10;
  const outputPtr = 1000;

  beforeEach(() => {
    memory = new WebAssembly.Memory({ initial: 1 });

    const paramSets = new Uint16Array(memory.buffer, 0, 10 * stride);
    const matchingParams = new Int32Array(memory.buffer, 200, bitsetSize * 2);
    const filteredTraceIndices = new Int32Array(memory.buffer, outputPtr, 10);

    traceData = {
      memory,
      paramSets,
      matchingParams,
      filteredTraceIndices,
      stride,
      numTraces: 1,
      maxParamId: 3,
      dataPtr: 0,
      matchingParamsPtr: 200,
      outPtr: outputPtr,
      bitsetSize,
    };

    // Populate trace 0 with params: cpu=x86 (id 1), mode=release (id 2), os=linux (id 3)
    // Storing p.id directly for tests (assuming pid in paramSets maps to p.id)
    paramSets[0] = 1; // cpu=x86
    paramSets[1] = 2; // mode=release
    paramSets[2] = 3; // os=linux
    paramSets[3] = 0; // sentinel
  });

  it('should suggest combinations by scanning mocked Wasm results', async () => {
    const queryInput = 'x86 release';
    const currentQuery: Query = {};

    const mockScanWasmBatch = async (
      _wasmFilter: any,
      td: TraceData,
      _queryPtr: number,
      _queryLen: number,
      _outputLimit: number,
      _totalQueryValues: number,
      _checkInterrupt: () => boolean
    ) => {
      // Write trace index 0 to output buffer
      const outBuffer = new Int32Array(td.memory.buffer, td.outPtr, 1);
      outBuffer[0] = 0;
      return 1; // 1 trace matched
    };

    const dummyUpdateCache = (_cache: SearchCache) => {};
    const dummyYield = async () => {};
    const dummyShouldAbort = () => false;
    const dummyGetQueryPtr = (_data: any) => 0;
    const mockWasmFilter = {} as any;

    const suggestions = await computeSuggestions(
      queryInput,
      currentQuery,
      testParams,
      null, // availableParams
      traceData,
      mockWasmFilter,
      { query: '', contextStr: '', indices: null },
      dummyUpdateCache,
      dummyYield,
      dummyShouldAbort,
      dummyGetQueryPtr,
      mockScanWasmBatch
    );

    expect(suggestions).to.not.be.null;
    expect(suggestions!.length).to.be.greaterThan(0);

    const firstSug = suggestions![0];
    expect(firstSug.params.length).to.equal(2);

    const hasX86 = firstSug.params.some((p) => p.key === 'cpu' && p.value === 'x86');
    const hasRelease = firstSug.params.some((p) => p.key === 'mode' && p.value === 'release');

    expect(hasX86).to.be.true;
    expect(hasRelease).to.be.true;
  });

  it('should work even if availableParams are missing id property (production case)', async () => {
    const queryInput = 'x86 release';
    const currentQuery: Query = {};

    // availableParams from production don't have 'id'
    const availableParamsWithoutId = [
      { key: 'cpu', value: 'x86' },
      { key: 'mode', value: 'release' },
      { key: 'os', value: 'linux' },
    ] as any[];

    const mockScanWasmBatch = async (
      _wasmFilter: any,
      td: TraceData,
      queryPtr: number,
      _queryLen: number,
      _outputLimit: number,
      _totalQueryValues: number,
      _checkInterrupt: () => boolean
    ) => {
      const qView = new Int32Array(td.memory.buffer, queryPtr, 5);
      console.log('TEST VERIFY Wasm Query buffer:', Array.from(qView));

      // Simulate real Wasm behavior: if param IDs are invalid (0), it matches nothing!
      if (qView[2] === 0 || qView[4] === 0) {
        return 0;
      }

      const outBuffer = new Int32Array(td.memory.buffer, td.outPtr, 1);
      outBuffer[0] = 0;
      return 1;
    };

    const dummyUpdateCache = (_cache: SearchCache) => {};
    const dummyYield = async () => {};
    const dummyShouldAbort = () => false;
    const dummyGetQueryPtr = (_data: any) => 0;
    const mockWasmFilter = {} as any;

    const suggestions = await computeSuggestions(
      queryInput,
      currentQuery,
      testParams,
      availableParamsWithoutId, // lacks IDs!
      traceData,
      mockWasmFilter,
      { query: '', contextStr: '', indices: null },
      dummyUpdateCache,
      dummyYield,
      dummyShouldAbort,
      dummyGetQueryPtr,
      mockScanWasmBatch
    );

    expect(suggestions).to.not.be.null;
    expect(suggestions!.length).to.be.greaterThan(0);

    const firstSug = suggestions![0];
    expect(firstSug.params.length).to.equal(2);

    const hasX86 = firstSug.params.some((p) => p.key === 'cpu' && p.value === 'x86');
    const hasRelease = firstSug.params.some((p) => p.key === 'mode' && p.value === 'release');

    expect(hasX86).to.be.true;
    expect(hasRelease).to.be.true;
  });
});
