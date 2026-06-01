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

  describe('getConfigUrl', () => {
    it('returns empty string if configUrl is not set', () => {
      assert.equal(element['getConfigUrl']('123'), '');
    });

    it('formats normal urls correctly', () => {
      element.configUrl = 'https://example.com/{revision}/config.json';
      assert.equal(element['getConfigUrl']('123'), 'https://example.com/123/config.json');
    });

    it('formats chrome-internal urls correctly', () => {
      element.configUrl = 'https://chrome-internal.googlesource.com/foo/+/{revision}/bar';
      assert.equal(
        element['getConfigUrl']('123'),
        'https://source.corp.google.com/h/chrome-internal/foo/+/123:bar'
      );
    });
  });

  describe('formatConfigUrl', () => {
    it('should return an empty string if configUrl is not provided', () => {
      const result = element['formatConfigUrl']('123');
      const container = document.createElement('div');
      render(result, container);
      const anchor = container.querySelector('a');
      assert.isNull(anchor);
    });

    it('should return a link with the correct URL', () => {
      element.configUrl = 'https://example.com/{revision}/config.json';
      const result = element['formatConfigUrl']('123');
      const container = document.createElement('div');
      render(result, container);
      const anchor = container.querySelector('a');
      assert.isNotNull(anchor);
      assert.equal(anchor!.href, 'https://example.com/123/config.json');
      // //example.com/123/config.json is the result of split(':').pop() on https://example.com/123/config.json
      assert.equal(anchor!.textContent, '//example.com/123/config.json');
    });
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

  it('renders nothing when no subscription is loaded', async () => {
    assert.equal(element.textContent!.trim(), '');
  });

  it('renders subscription details when loaded', async () => {
    element.configUrl = 'https://example.com/{revision}/config.json';
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
    await element.updateComplete;

    assert.include(element.textContent, 'V8 JavaScript Perf');
    assert.include(element.textContent, 'v8-perf@google.com');
    assert.include(element.textContent, 'abcdef123');
    assert.include(element.textContent, '12345');
    assert.include(element.textContent, 'hotlist1');
    assert.include(element.textContent, 'Priority: 1');
    assert.include(element.textContent, 'Severity: 2');
    assert.include(element.textContent, 'cc1@google.com');

    // Check for config URL.
    const anchors = element.querySelectorAll('a');
    let configLinkFound = false;
    anchors.forEach((a) => {
      if (a.href === 'https://example.com/abcdef123/config.json') {
        configLinkFound = true;
      }
    });
    assert.isTrue(configLinkFound, 'Config URL link was not found');

    assert.isNull(element.querySelector('#alerts-table'));
  });

  it('toggles alerts table', async () => {
    const subscription = { name: 'Test' };
    const alerts: any[] = [{ query: 'config=8888', step: 'original' }];
    element.load(subscription, alerts);
    await element.updateComplete;

    const toggleButton = element.querySelector<HTMLButtonElement>('#btn-toggle-alerts')!;
    assert.include(toggleButton.textContent, 'Show alerts configuration (1)');

    toggleButton.click();
    await element.updateComplete;
    assert.include(toggleButton.textContent, 'Hide alerts configuration');
    assert.isNotNull(element.querySelector('#alerts-table'));

    const rows = element.querySelectorAll('table#alerts-table > tbody > tr');
    assert.equal(rows.length, 1);

    toggleButton.click();
    await element.updateComplete;
    assert.include(toggleButton.textContent, 'Show alerts configuration (1)');
    assert.isNull(element.querySelector('#alerts-table'));
  });
});
