import './index';
import { expect } from 'chai';
import { PickerFieldSk } from './picker-field-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';

describe('picker-field-sk', () => {
  const newInstance = setUpElementUnderTest<PickerFieldSk>('picker-field-sk');

  let element: PickerFieldSk;
  beforeEach(() => {
    element = newInstance((_el: PickerFieldSk) => {
      // Place here any code that must run after the element is instantiated but
      // before it is attached to the DOM (e.g. property setter calls,
      // document-level event listeners, etc.).
    });
  });

  describe('', () => {
    it('some result', () => {
      // eslint-disable-next-line @typescript-eslint/no-unused-expressions
      expect(element).to.not.be.null;
    });
  });
});
