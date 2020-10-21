import './index';
import { RotationsSk } from './rotations-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { expect } from 'chai';
import { $, $$ } from 'common-sk/modules/dom';

describe('rotations-sk', () => {
  const newInstance = setUpElementUnderTest<RotationsSk>('rotations-sk');

  let element: RotationsSk;
  beforeEach(() => {
    element = newInstance((el: RotationsSk) => {
      el.rotations = [
        { role: 'Arborist', icon: 'nature', currentUrl: '', docLink: '', name: 'alice' },
        { role: 'Wrangler', icon: 'gesture', currentUrl: '', docLink: '', name: 'bob' },
        { role: 'Android', icon: 'android', currentUrl: '', docLink: '', name: 'christy' },
        { role: 'Beekeeper', icon: 'grain', currentUrl: '', docLink: '', name: 'dan' },
      ];
    });
  });

  describe('displays', () => {
    it('rotations', () => {
      expect($('a', element)).to.have.length(4);
      expect($$('a', element)).to.have.property('innerText', 'Arborist: alice');
    });
  });
});
