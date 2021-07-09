import './index';
import { assert } from 'chai';
import { $$ } from 'common-sk/modules/dom';
import { AutoRefreshSk } from './auto-refresh-sk';

import { eventPromise, setUpElementUnderTest } from '../../../infra-sk/modules/test_util';

describe('auto-refresh-sk', () => {
  const newInstance = setUpElementUnderTest<AutoRefreshSk>('auto-refresh-sk');

  let element: AutoRefreshSk;
  beforeEach(() => {
    element = newInstance((el: AutoRefreshSk) => {
      el.refreshing = false;
    });
  });

  describe('clicking on icon', () => {
    it('toggles refresh', () => {
      const toggle = $$<HTMLSpanElement>('#refresh', element)!;
      toggle.click();
      // eslint-disable-next-line dot-notation
      assert.notEqual(element['timeout'], 0);
      assert.isTrue(element.refreshing);
      toggle.click();
      assert.isFalse(element.refreshing);
    });
  });

  it('should trigger a custom event', async () => {
    const myEvent = eventPromise('refresh-page');
    const toggle = $$<HTMLSpanElement>('#refresh', element)!;
    toggle.click();
    await myEvent;
  });
});
