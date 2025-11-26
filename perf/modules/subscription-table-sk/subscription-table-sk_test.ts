import './index';
import { assert } from 'chai';
import { SubscriptionTableSk } from './subscription-table-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { render } from 'lit';

describe('SubscriptionTableSk', () => {
  const newInstance = setUpElementUnderTest<SubscriptionTableSk>('subscription-table-sk');

  let element: SubscriptionTableSk;
  beforeEach(() => {
    element = newInstance();
  });

  describe('formatBugComponent', () => {
    it('should return an empty string if component is not provided', () => {
      const result = element['formatBugComponent']('');
      const container = document.createElement('div');
      render(result, container);
      const anchor = container.querySelector('a');
      assert.isNull(anchor);
    });

    it('should return a link with the correct URL encoding', () => {
      const component = '12345';
      const result = element['formatBugComponent'](component);
      const container = document.createElement('div');
      render(result, container);
      const anchor = container.querySelector('a');
      assert.isNotNull(anchor);
      const expectedQuery = encodeURIComponent('status:open componentid:12345');
      assert.equal(
        anchor!.href,
        `https://g-issues.chromium.org/issues?q=${expectedQuery}&s=created_time:desc`
      );
      assert.equal(anchor!.textContent, component);
    });
  });
});
