import './index';
import { expect } from 'chai';
import { ZoomSk } from './zoom-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';

describe('zoom-sk', () => {
  const newInstance = setUpElementUnderTest<ZoomSk>('zoom-sk');

  let element: ZoomSk;
  beforeEach(() => {
    element = newInstance((el: ZoomSk) => {
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
