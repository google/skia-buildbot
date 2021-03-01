import './index';
import { EditChildShaderSk } from './edit-child-shader-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { expect } from 'chai';

describe('edit-child-shader-sk', () => {
  const newInstance = setUpElementUnderTest<EditChildShaderSk>('edit-child-shader-sk');

  let element: EditChildShaderSk;
  beforeEach(() => {
    element = newInstance((el: EditChildShaderSk) => {
      // Place here any code that must run after the element is instantiated but
      // before it is attached to the DOM (e.g. property setter calls,
      // document-level event listeners, etc.).
    });
  });

  describe('some action', () => {
    it('some result', () => {});
      expect(element).to.not.be.null;
  });
});
