import './index';
import { AndroidLayersSk } from './android-layers-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { expect } from 'chai';

describe('android-layers-sk', () => {
  const newInstance = setUpElementUnderTest<AndroidLayersSk>('android-layers-sk');

  let element: AndroidLayersSk;
  beforeEach(() => {
    element = newInstance((el: AndroidLayersSk) => {
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
