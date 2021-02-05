import './index';
import { UniformDimensionsSk } from './uniform-dimensions-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { assert } from 'chai';

describe('uniform-dimensions-sk', () => {
  const newInstance = setUpElementUnderTest<UniformDimensionsSk>(
    'uniform-dimensions-sk'
  );

  let element: UniformDimensionsSk;
  beforeEach(() => {
    element = newInstance((el: UniformDimensionsSk) => {
      // Place here any code that must run after the element is instantiated but
      // before it is attached to the DOM (e.g. property setter calls,
      // document-level event listeners, etc.).
    });
  });

  describe('unform-dimensions-sk', () => {
    it('applies uniforms correctly', () => {
      // Make uniforms longer than needed to show we don't disturb other values.
      const uniforms = new Float32Array(5);

      element.uniform = {
        name: 'iDimensions',
        columns: 3,
        rows: 1,
        slot: 1,
      };

      element.x = 800;
      element.y = 600;
      element.applyUniformValues(uniforms);
      assert.deepEqual(uniforms, new Float32Array([0, 800, 600, 0, 0]));
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
