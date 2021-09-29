import './index';
import { expect } from 'chai';
import { ExampleControlSk } from './example-control-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';

describe('example-control-sk', () => {
  const newInstance = setUpElementUnderTest<ExampleControlSk>('example-control-sk');

  let element: ExampleControlSk;
  beforeEach(() => {
    element = newInstance((el: ExampleControlSk) => {
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
