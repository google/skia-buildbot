import { DetailsDialogSk } from './details-dialog-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { expect } from 'chai';

describe('details-dialog-sk', () => {
  const newInstance = setUpElementUnderTest<DetailsDialogSk>('details-dialog-sk');

  let element: DetailsDialogSk;
  beforeEach(() => {
    element = newInstance((el: DetailsDialogSk) => {
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
