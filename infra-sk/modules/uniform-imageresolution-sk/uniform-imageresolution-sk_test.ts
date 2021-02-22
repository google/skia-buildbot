import './index';
import { assert } from 'chai';
import { imageSize, UniformImageresolutionSk } from './uniform-imageresolution-sk';

import { setUpElementUnderTest } from '../test_util';

describe('uniform-imageresolution-sk', () => {
  const newInstance = setUpElementUnderTest<UniformImageresolutionSk>('uniform-imageresolution-sk');

  let element: UniformImageresolutionSk;
  beforeEach(() => {
    element = newInstance();
  });

  describe('unform-imageresolution-sk', () => {
    it('reports the right image sizes', () => {
      const uniforms = [0, 0, 0];
      element.applyUniformValues(uniforms);
      assert.deepEqual(uniforms, [imageSize, imageSize, 0]);
    });

    it('does not need raf updates', () => {
      assert.isFalse(element.needsRAF());
    });
  });
});
