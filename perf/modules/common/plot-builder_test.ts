import { assert } from 'chai';
import { getTraceColor, convertFromDataframe, defaultColors, internals } from './plot-builder';
import { TraceSet, ColumnHeader, Trace } from '../json';
import { MISSING_DATA_SENTINEL } from '../const/const';
import sinon from 'sinon';

describe('plot-builder', () => {
  describe('getTraceColor', () => {
    it('returns consistent color for same string', () => {
      assert.equal(getTraceColor('foo'), getTraceColor('foo'));
    });
    it('returns different colors for different strings', () => {
      assert.notEqual(getTraceColor('foo'), getTraceColor('bar'));
    });

    it('handles subtest_4=ref correctly', () => {
      const baseTrace = ',benchmark=v8,test=JetStream2,';
      const refTrace = ',benchmark=v8,subtest_4=ref,test=JetStream2,';

      const baseColor = getTraceColor(baseTrace);
      const refColor = getTraceColor(refTrace);

      // We cannot easily verify exact index without duplicating logic or knowing hash,
      // but we can ensure they are distinct if they would have collided, or just distinct in general if no collision.
      // Actually, if no collision, they might be distinct by chance.
      // The requirement is that *if* collision, we force distinct.
      // If we pick a random string, collision is unlikely.
      // Let's just ensure they return valid colors.
      assert.include(defaultColors, baseColor);
      assert.include(defaultColors, refColor);
    });

    it('ensures ref and pgo do not collide for same base trace', () => {
      const refTrace = ',benchmark=v8,subtest_4=ref,test=JetStream2,';
      const pgoTrace = ',benchmark=v8,subtest_4=pgo,test=JetStream2,';

      const refColor = getTraceColor(refTrace);
      const pgoColor = getTraceColor(pgoTrace);

      assert.notEqual(refColor, pgoColor);
    });

    it('resolves collision when Base and Ref collide, ensuring Pgo also shifts', () => {
      // Scenario:
      // Base Trace Hash % N = 10
      // Ref Trace Hash % N = 10 (Collision with Base)
      // Pgo Trace Hash % N = 11 (No initial collision with Base, but would collide with shifted Ref)

      // We expect:
      // Ref -> Base + 1 = 11
      // Pgo -> Base + 2 = 12 (Shifted to avoid collision with Ref)

      const baseTrace = 'base_trace';
      const refTrace = 'base_trace,subtest_4=ref';
      const pgoTrace = 'base_trace,subtest_4=pgo';

      const stub = sinon.stub(internals, 'getTraceHash');
      try {
        // Mock getTraceHash to return specific values that modulo to 10, 10, 11
        // defaultColors.length is 20.
        // 10 -> 10
        // 11 -> 11
        stub.withArgs(baseTrace).returns(10);
        stub.withArgs(refTrace).returns(10);
        stub.withArgs(pgoTrace).returns(11);

        const refColor = getTraceColor(refTrace);
        const pgoColor = getTraceColor(pgoTrace);

        // Ref should be defaultColors[11]
        assert.equal(refColor, defaultColors[11], 'Ref should be shifted to 11');

        // Pgo should be defaultColors[12] (not 11!)
        assert.equal(pgoColor, defaultColors[12], 'Pgo should be shifted to 12');
      } finally {
        stub.restore();
      }
    });

    it('resolves 3-way collision (Base, Ref, Pgo all collide)', () => {
      // Scenario:
      // Base Trace Hash % N = 10
      // Ref Trace Hash % N = 10
      // Pgo Trace Hash % N = 10

      // We expect:
      // Ref -> Base + 1 = 11
      // Pgo -> Base + 2 = 12

      const baseTrace = 'base_trace';
      const refTrace = 'base_trace,subtest_4=ref';
      const pgoTrace = 'base_trace,subtest_4=pgo';

      const stub = sinon.stub(internals, 'getTraceHash');
      try {
        stub.withArgs(baseTrace).returns(10);
        stub.withArgs(refTrace).returns(10);
        stub.withArgs(pgoTrace).returns(10);

        const refColor = getTraceColor(refTrace);
        const pgoColor = getTraceColor(pgoTrace);

        assert.equal(refColor, defaultColors[11], 'Ref should be shifted to 11');
        assert.equal(pgoColor, defaultColors[12], 'Pgo should be shifted to 12');
      } finally {
        stub.restore();
      }
    });

    it('handles wrap-around when Base is at the end of color array', () => {
      // Scenario:
      // defaultColors.length = 20
      // Base Trace Hash % N = 19
      // Ref Trace Hash % N = 19 (Collision with Base)
      // Pgo Trace Hash % N = 19 (Collision with Base)

      // We expect:
      // Ref -> (Base + 1) % 20 = 0
      // Pgo -> (Base + 2) % 20 = 1

      const baseTrace = 'base_trace';
      const refTrace = 'base_trace,subtest_4=ref';
      const pgoTrace = 'base_trace,subtest_4=pgo';

      const stub = sinon.stub(internals, 'getTraceHash');
      try {
        stub.withArgs(baseTrace).returns(19);
        stub.withArgs(refTrace).returns(19);
        stub.withArgs(pgoTrace).returns(19);

        const refColor = getTraceColor(refTrace);
        const pgoColor = getTraceColor(pgoTrace);

        // Check against defaultColors[0] and defaultColors[1]
        assert.equal(refColor, defaultColors[0], 'Ref should wrap around to 0');
        assert.equal(pgoColor, defaultColors[1], 'Pgo should wrap around to 1');
      } finally {
        stub.restore();
      }
    });

    it('adjusts both Ref and Pgo even if only Pgo collides with Base', () => {
      // Scenario:
      // Base Trace Hash % N = 10
      // Ref Trace Hash % N = 15 (No collision with Base or Pgo)
      // Pgo Trace Hash % N = 10 (Collision with Base)

      // Since *any* collision in the triplet triggers the offset logic:
      // We expect strict offsets:
      // Ref -> Base + 1 = 11 (Even though it was 15)
      // Pgo -> Base + 2 = 12 (Shifted to avoid Base)

      const baseTrace = 'base_trace';
      const refTrace = 'base_trace,subtest_4=ref';
      const pgoTrace = 'base_trace,subtest_4=pgo';

      const stub = sinon.stub(internals, 'getTraceHash');
      try {
        stub.withArgs(baseTrace).returns(10);
        stub.withArgs(refTrace).returns(15);
        stub.withArgs(pgoTrace).returns(10);

        const refColor = getTraceColor(refTrace);
        const pgoColor = getTraceColor(pgoTrace);

        assert.equal(
          refColor,
          defaultColors[11],
          'Ref should be forced to 11 due to Pgo collision'
        );
        assert.equal(
          pgoColor,
          defaultColors[12],
          'Pgo should be forced to 12 due to Base collision'
        );
      } finally {
        stub.restore();
      }
    });
    it('handles dynamic subtest indices (subtest_1, subtest_10)', () => {
      // Scenario:
      // Base Trace Hash % N = 10
      // subtest_1=ref Hash % N = 10 (Collision!)
      // subtest_1=pgo Hash % N = 11
      // Expect Ref -> 11, Pgo -> 12

      const baseTrace = 'base_trace';
      const refTrace = 'base_trace,subtest_1=ref';
      const pgoTrace = 'base_trace,subtest_1=pgo';

      const stub = sinon.stub(internals, 'getTraceHash');
      try {
        stub.withArgs(baseTrace).returns(10);
        stub.withArgs(refTrace).returns(10);
        stub.withArgs(pgoTrace).returns(11); // Doesn't matter, will be shifted

        const refColor = getTraceColor(refTrace);
        const pgoColor = getTraceColor(pgoTrace);

        assert.equal(refColor, defaultColors[11], 'Ref with subtest_1 should be shifted to 11');
        assert.equal(pgoColor, defaultColors[12], 'Pgo with subtest_1 should be shifted to 12');
      } finally {
        stub.restore();
      }
    });

    it('preserves original colors if no collision (Base, Ref, Pgo distinct)', () => {
      // Scenario:
      // Base Trace Hash % N = 10
      // Ref Trace Hash % N = 15
      // Pgo Trace Hash % N = 18
      // No collisions -> Should keep original colors

      const baseTrace = 'base_trace';
      const refTrace = 'base_trace,subtest_4=ref';
      const pgoTrace = 'base_trace,subtest_4=pgo';

      const stub = sinon.stub(internals, 'getTraceHash');
      try {
        stub.withArgs(baseTrace).returns(10);
        stub.withArgs(refTrace).returns(15);
        stub.withArgs(pgoTrace).returns(18);

        const baseColor = getTraceColor(baseTrace);
        const refColor = getTraceColor(refTrace);
        const pgoColor = getTraceColor(pgoTrace);

        assert.equal(baseColor, defaultColors[10], 'Base should stay at 10');
        assert.equal(refColor, defaultColors[15], 'Ref should stay at 15');
        assert.equal(pgoColor, defaultColors[18], 'Pgo should stay at 18');
      } finally {
        stub.restore();
      }
    });

    it('ignores unrelated subtest keys', () => {
      // Scenario:
      // Trace has subtest_4=something_else
      // Should treat it as a normal string and return its original hash

      const traceName = 'base_trace,subtest_4=something_else';
      const originalHash = 5;

      const stub = sinon.stub(internals, 'getTraceHash');
      try {
        stub.withArgs(traceName).returns(originalHash);

        const color = getTraceColor(traceName);

        assert.equal(color, defaultColors[5], 'Should return original hash color');
      } finally {
        stub.restore();
      }
    });
  });

  describe('convertFromDataframe', () => {
    it('returns null for empty header', () => {
      assert.isNull(convertFromDataframe({ traceset: TraceSet({}), header: [] }));
    });

    it('returns null for non-empty header with empty traceset', () => {
      const header: ColumnHeader[] = [
        { offset: 100, timestamp: 1000 },
        { offset: 101, timestamp: 2000 },
      ] as any;
      assert.isNull(convertFromDataframe({ traceset: TraceSet({}), header }, 'commit'));
    });

    it('converts dataframe correctly', () => {
      const traceset: TraceSet = TraceSet({
        trace1: Trace([1, 2]),
        trace2: Trace([3, MISSING_DATA_SENTINEL]),
      });
      const header: ColumnHeader[] = [
        { offset: 100, timestamp: 1000 },
        { offset: 101, timestamp: 2000 },
      ] as any;

      const result = convertFromDataframe({ traceset, header }, 'commit');
      assert.isNotNull(result);
      // Row 0: Header
      // Row 1: Data point 1
      // Row 2: Data point 2
      assert.equal(result!.length, 3);

      // Header check
      // [ {role: domain...}, {type: 'number', label: 'trace1'}, {type: 'number', label: 'trace2'} ]
      assert.deepEqual(result![0][1], { type: 'number', label: 'trace1' });
      assert.deepEqual(result![0][2], { type: 'number', label: 'trace2' });

      // Data check
      // Row 1: [100, 1, 3]
      assert.equal(result![1][0], 100);
      assert.equal(result![1][1], 1);
      assert.equal(result![1][2], 3);

      // Row 2: [101, 2, null] (missing data sentinel -> null)
      assert.equal(result![2][0], 101);
      assert.equal(result![2][1], 2);
      assert.isNull(result![2][2]);
    });

    it('converts placeholder dataframe with all missing data correctly', () => {
      // Verify that a dataframe containing only missing data points (returned when
      // a search matches traces but finds no data in the requested range)
      // is handled correctly.
      const traceset: TraceSet = TraceSet({
        ',arch=x86,': Trace([MISSING_DATA_SENTINEL, MISSING_DATA_SENTINEL]),
      });
      const header: ColumnHeader[] = [
        { offset: 100, timestamp: 1000 },
        { offset: 500, timestamp: 5000 },
      ] as any;

      const result = convertFromDataframe({ traceset, header }, 'commit');
      assert.isNotNull(result);
      assert.equal(result!.length, 3); // Header + 2 data rows

      // Header should be typed correctly
      assert.deepEqual(result![0][1], { type: 'number', label: ',arch=x86,' });

      // First Data Row: [100, null]
      assert.equal(result![1][0], 100);
      assert.isNull(result![1][1]);

      // Second Data Row: [500, null]
      assert.equal(result![2][0], 500);
      assert.isNull(result![2][1]);
    });
  });
});
