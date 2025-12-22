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

  it('renders nothing when no subscription is loaded', () => {
    assert.equal(element.textContent!.trim(), '');
  });

  it('renders subscription details when loaded', () => {
    const subscription = {
      name: 'V8 JavaScript Perf',
      contact_email: 'v8-perf@google.com',
      revision: 'abcdef123',
      bug_component: '12345',
      hotlists: ['hotlist1'],
      bug_priority: 1,
      bug_severity: 2,
      bug_cc_emails: ['cc1@google.com'],
    };
    const alerts: any[] = [{ query: 'config=8888', step: 'original', radius: 10 }];

    element.load(subscription, alerts);

    assert.include(element.textContent, 'V8 JavaScript Perf (1 Alert(s) Configured)');
    assert.include(element.textContent, 'v8-perf@google.com');
    assert.include(element.textContent, 'abcdef123');
    assert.include(element.textContent, '12345');
    assert.include(element.textContent, 'hotlist1');
    assert.include(element.textContent, 'Priority: 1');
    assert.include(element.textContent, 'Severity: 2');
    assert.include(element.textContent, 'cc1@google.com');

    assert.isNull(element.querySelector('#alerts-table'));
  });

  it('toggles alerts table', () => {
    const subscription = { name: 'Test' };
    const alerts: any[] = [{ query: 'config=8888', step: 'original' }];
    element.load(subscription, alerts);

    const toggleButton = element.querySelector<HTMLButtonElement>('#btn-toggle-alerts')!;
    assert.include(toggleButton.textContent, 'Show 1 Alert Configuration(s)');

    toggleButton.click();
    assert.include(toggleButton.textContent, 'Hide Alert Configuration(s)');
    assert.isNotNull(element.querySelector('#alerts-table'));

    const rows = element.querySelectorAll('table#alerts-table > tbody > tr');
    assert.equal(rows.length, 1);

    toggleButton.click();
    assert.include(toggleButton.textContent, 'Show 1 Alert Configuration(s)');
    assert.isNull(element.querySelector('#alerts-table'));
  });
});
