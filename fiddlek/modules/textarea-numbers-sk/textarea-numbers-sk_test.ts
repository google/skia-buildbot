import { TextareaNumbersSk } from './textarea-numbers-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { expect } from 'chai';

describe('textarea-numbers-sk', () => {
  const newInstance = setUpElementUnderTest<TextareaNumbersSk>('textarea-numbers-sk');

  let element: TextareaNumbersSk;
  beforeEach(() => {
    element = newInstance((el: TextareaNumbersSk) => {
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
