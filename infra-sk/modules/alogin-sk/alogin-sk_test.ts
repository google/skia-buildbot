import './index';
import { assert } from 'chai';
import { AloginSk } from './alogin-sk';
import { setUpElementUnderTest } from '../test_util';

describe('alogin-sk', () => {
  const newInstance = setUpElementUnderTest<AloginSk>('alogin-sk');

  let element: AloginSk;
  beforeEach(() => {
    element = newInstance((el: AloginSk) => {
      el.setAttribute('testing_offline', 'true');
    });
  });

  describe('alogin-sk', () => {
    it('returns a fake email when testing', async () => {
      const status = await element.statusPromise;
      assert.equal(status.email, 'test@example.com');
    });
  });
});
