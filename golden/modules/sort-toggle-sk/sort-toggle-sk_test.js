import './index';
import { $$ } from 'common-sk/modules/dom';
import { eventPromise, setUpElementUnderTest } from '../../../infra-sk/modules/test_util';

describe('sort-toggle-sk', () => {
  const newInstance = setUpElementUnderTest('sort-toggle-sk');

  let sortToggleSk;
  beforeEach(() => {
    sortToggleSk = newInstance();
    sortToggleSk.key = 'age';
  });

  describe('emitting sort-toggle events', () => {
    it('defaults to ascending', async () => {
      const sortChanged = eventPromise('sort-change', 100);
      $$('div', sortToggleSk).click();
      const ev = await sortChanged;
      expect(sortToggleSk.currentKey).to.equal('age');
      expect(sortToggleSk.direction).to.equal('asc');
      expect(ev.detail.key).to.equal('age');
      expect(ev.detail.direction).to.equal('asc');
    });

    it('toggles from ascending to descending', async () => {
      sortToggleSk.toggle();
      expect(sortToggleSk.direction).to.equal('asc');

      const sortChanged = eventPromise('sort-change', 100);
      $$('div', sortToggleSk).click();
      const ev = await sortChanged;
      expect(sortToggleSk.currentKey).to.equal('age');
      expect(sortToggleSk.direction).to.equal('desc');
      expect(ev.detail.key).to.equal('age');
      expect(ev.detail.direction).to.equal('desc');
    });
  });
});
