import { FiddleEmbed } from './fiddle-embed';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { expect } from 'chai';

describe('fiddle-embed', () => {
  const newInstance = setUpElementUnderTest<FiddleEmbed>('fiddle-embed');

  let element: FiddleEmbed;
  beforeEach(() => {
    element = newInstance((el: FiddleEmbed) => {
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
