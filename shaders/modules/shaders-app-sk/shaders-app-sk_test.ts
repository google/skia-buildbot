import './index';
import { ShadersAppSk } from './shaders-app-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { expect } from 'chai';

describe('shaders-app-sk', () => {
  const newInstance = setUpElementUnderTest<ShadersAppSk>('shaders-app-sk');

  let element: ShadersAppSk;
  beforeEach(() => {
    element = newInstance((el: ShadersAppSk) => {
      // Place here any code that must run after the element is instantiated but
      // before it is attached to the DOM (e.g. property setter calls,
      // document-level event listeners, etc.).
    });
  });

  describe('some action', () => {
    it('some result', () => {
      expect(element).to.not.be.null;
    });
  });
});
