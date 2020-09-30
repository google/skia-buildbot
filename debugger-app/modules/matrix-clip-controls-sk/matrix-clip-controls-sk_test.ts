import './index';
import { MatrixClipControlsSk } from './matrix-clip-controls-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { expect } from 'chai';

describe('matrix-clip-controls-sk', () => {
  const newInstance = setUpElementUnderTest<MatrixClipControlsSk>('matrix-clip-controls-sk');

  let element: MatrixClipControlsSk;
  beforeEach(() => {
    element = newInstance((el: MatrixClipControlsSk) => {
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
