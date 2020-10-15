import './index';
import { CommandsSk } from './commands-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { expect } from 'chai';

describe('commands-sk', () => {
  const newInstance = setUpElementUnderTest<CommandsSk>('commands-sk');

  let element: CommandsSk;
  beforeEach(() => {
    element = newInstance((el: CommandsSk) => {
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
