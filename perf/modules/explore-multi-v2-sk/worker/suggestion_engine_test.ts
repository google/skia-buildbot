import { expect } from 'chai';
import { computeSuggestions } from './suggestion_engine';
import { Param, Query } from './worker-types';

describe('suggestion_engine memory-lookup path', () => {
  const testParams: Param[] = [
    { id: 1, key: 'cpu', value: 'x86' },
    { id: 2, key: 'mode', value: 'release' },
    { id: 3, key: 'os', value: 'linux' },
    { id: 4, key: 'cpu', value: 'arm64' },
    { id: 5, key: 'mode', value: 'debug' },
  ];

  const dummyShouldAbort = () => false;

  it('should suggest a single attribute (sanity check)', async () => {
    const queryInput = 'x86';
    const currentQuery: Query = {};

    const suggestions = await computeSuggestions(
      queryInput,
      currentQuery,
      testParams,
      null, // availableParams
      null, // traceData
      dummyShouldAbort
    );

    expect(suggestions).to.not.be.null;
    expect(suggestions!.length).to.be.greaterThan(0);
    expect(suggestions![0].params.length).to.equal(1);
    expect(suggestions![0].params[0]).to.deep.equal({ id: 1, key: 'cpu', value: 'x86' });
  });

  it('should suggest value-only glob matches correctly', async () => {
    const queryInput = 'x8*';
    const currentQuery: Query = {};

    const suggestions = await computeSuggestions(
      queryInput,
      currentQuery,
      testParams,
      null,
      null,
      dummyShouldAbort
    );

    expect(suggestions).to.not.be.null;
    expect(suggestions!.length).to.equal(1);
    expect(suggestions![0].params[0]).to.deep.equal({ id: -1, key: 'cpu', value: 'x8*' });
  });

  it('should suggest combinations and exclude single parameters when combinations exist', async () => {
    const queryInput = 'linu deb';
    const currentQuery: Query = {};

    const suggestions = await computeSuggestions(
      queryInput,
      currentQuery,
      testParams,
      null,
      null,
      dummyShouldAbort
    );

    expect(suggestions).to.not.be.null;
    suggestions!.forEach((s) => {
      expect(s.params.length).to.be.greaterThan(1);
    });
  });

  it('should suggest combined suggestions (Cartesian product) for space-separated queries', async () => {
    const queryInput = 'linu deb';
    const currentQuery: Query = {};

    const suggestions = await computeSuggestions(
      queryInput,
      currentQuery,
      testParams,
      null,
      null,
      dummyShouldAbort
    );

    expect(suggestions).to.not.be.null;
    const combined = suggestions!.find((s) => s.params.length > 1);
    expect(combined).to.not.be.undefined;
    expect(combined!.params.map((p) => p.value)).to.include('linux');
    expect(combined!.params.map((p) => p.value)).to.include('debug');
  });

  it('should suggest combinations and compute exact intersection counts with traceData', async () => {
    const queryInput = 'linu deb';
    const currentQuery: Query = {};

    // 1-based IDs in paramSets:
    // Trace 0: cpu=x86 (id=1), mode=release (id=2), os=linux (id=3) -> [1, 2, 3, 0] (stride=4)
    // Trace 1: cpu=arm64 (id=4), mode=debug (id=5), os=linux (id=3) -> [4, 5, 3, 0] (stride=4)
    // Trace 2: cpu=x86 (id=1), mode=debug (id=5), os=linux (id=3) -> [1, 5, 3, 0] (stride=4)
    // Param IDs:
    // cpu=x86: 1
    // mode=release: 2
    // os=linux: 3
    // cpu=arm64: 4
    // mode=debug: 5
    const paramSets = new Uint16Array([1, 2, 3, 0, 4, 5, 3, 0, 1, 5, 3, 0]);

    const matchingParams = new Int32Array(100);
    // Setup matchingParams so all parameters are considered matched for pool filtering
    matchingParams.fill(1);
    matchingParams[3] = 3; // os=linux (id=3) is in 3 traces
    matchingParams[5] = 2; // mode=debug (id=5) is in 2 traces

    const mockTraceData = {
      paramSets,
      stride: 4,
      numTraces: 3,
      bitsetSize: 10,
      matchingParams,
    } as any;

    const suggestions = await computeSuggestions(
      queryInput,
      currentQuery,
      testParams,
      null,
      mockTraceData,
      dummyShouldAbort
    );

    expect(suggestions).to.not.be.null;
    // The combination: os=linux (id 3) and mode=debug (id 5)
    // Matching traces for combination: Trace 1 and Trace 2 (both have linux and debug). So count should be 2!
    const combined = suggestions!.find((s) => s.params.length > 1);
    expect(combined).to.not.be.undefined;
    expect(combined!.count).to.equal(2);
  });

  it('should ignore virtual/non-existent query keys (like stat) and still match combinations correctly', async () => {
    const queryInput = 'linu deb';
    // currentQuery contains a virtual key 'stat'
    const currentQuery: Query = {
      stat: ['max', 'min'],
    };

    const paramSets = new Uint16Array([1, 2, 3, 0, 4, 5, 3, 0, 1, 5, 3, 0]);

    const matchingParams = new Int32Array(100);
    matchingParams.fill(1);
    matchingParams[3] = 3; // os=linux (id=3) is in 3 traces
    matchingParams[5] = 2; // mode=debug (id=5) is in 2 traces

    const mockTraceData = {
      paramSets,
      stride: 4,
      numTraces: 3,
      bitsetSize: 10,
      matchingParams,
    } as any;

    const suggestions = await computeSuggestions(
      queryInput,
      currentQuery,
      testParams,
      null,
      mockTraceData,
      dummyShouldAbort
    );

    expect(suggestions).to.not.be.null;
    // Should still find the combination of os=linux (id 3) and mode=debug (id 5) with count 2, ignoring stat!
    const combined = suggestions!.find((s) => s.params.length > 1);
    expect(combined).to.not.be.undefined;
    expect(combined!.count).to.equal(2);
  });

  it('should suggest other values for a key when key is in currentQuery, omitting only exact active key=value pair', async () => {
    const params: Param[] = [
      { id: 1, key: 'os', value: 'linux' },
      { id: 2, key: 'os', value: 'linux_official' },
      { id: 3, key: 'os', value: 'windows' },
    ];
    const queryInput = 'linu';
    const currentQuery: Query = {
      os: ['linux'],
    };

    const suggestions = await computeSuggestions(
      queryInput,
      currentQuery,
      params,
      null,
      null,
      dummyShouldAbort
    );

    expect(suggestions).to.not.be.null;
    expect(suggestions!.length).to.equal(1);
    expect(suggestions![0].params[0]).to.deep.equal({ id: 2, key: 'os', value: 'linux_official' });
  });
});
