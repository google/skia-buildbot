import './index';
import { assert } from 'chai';
import { UniformDimensionsSk } from './uniform-dimensions-sk';

import { setUpElementUnderTest } from '../test_util';

describe('uniform-dimensions-sk', () => {
  const newInstance = setUpElementUnderTest<UniformDimensionsSk>(
    'uniform-dimensions-sk',
  );

  let element: UniformDimensionsSk;
  beforeEach(() => {
    element = newInstance();
  });

  describe('unform-dimensions-sk', () => {
    it('applies uniforms correctly', () => {
      // Make uniforms longer than needed to show we don't disturb other values.
      const uniforms = [0, 0, 0, 0, 0];

      element.uniform = {
        name: 'iDimensions',
        columns: 3,
        rows: 1,
        slot: 1,
      };

      element.applyUniformValues(uniforms);
      assert.deepEqual(uniforms, [0, 512, 512, 0, 0]);
    });

    it('changes the applied uniforms when the choice changes', () => {
      // Make uniforms longer than needed to show we don't disturb other values.
      const uniforms = [0, 0, 0, 0, 0];

      element.uniform = {
        name: 'iDimensions',
        columns: 3,
        rows: 1,
        slot: 1,
      };

      element.choice = 0;
      element.applyUniformValues(uniforms);
      assert.deepEqual(uniforms, [0, 128, 128, 0, 0]);
    });

    it('throws on invalid uniforms', () => {
      assert.throws(() => {
        element.uniform = {
          name: '',
          columns: 1,
          rows: 1,
          slot: 1,
        };
      });
    });
  });
});
