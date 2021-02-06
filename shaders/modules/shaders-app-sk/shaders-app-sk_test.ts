import './index';
import { assert } from 'chai';
import { ShadersAppSk } from './shaders-app-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';

describe('shaders-app-sk', () => {
  const newInstance = setUpElementUnderTest<ShadersAppSk>('shaders-app-sk');

  let element: ShadersAppSk;
  beforeEach(() => {
    element = newInstance();
  });

  describe('some action', () => {
    it('some result', () => {
      assert.isNotNull(element);
    });
  });
});
