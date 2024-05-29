import './index';
import { expect } from 'chai';
import { TestPickerSk } from './test-picker-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';

describe('test-picker-sk', () => {
  const newInstance = setUpElementUnderTest<TestPickerSk>('test-picker-sk');

  let element: TestPickerSk;
  beforeEach(() => {
    element = newInstance((el: TestPickerSk) => {
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
