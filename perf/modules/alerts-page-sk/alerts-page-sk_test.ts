import './index';
import { assert } from 'chai';
import fetchMock from 'fetch-mock';
import { AlertsPageSk } from './alerts-page-sk';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { Alert, SkPerfConfig, SerializesToString } from '../json';

describe('alerts-page-sk', () => {
  const newInstance = setUpElementUnderTest<AlertsPageSk>('alerts-page-sk');

  let element: AlertsPageSk;

  beforeEach(() => {
    window.perf = {
      need_alert_action: false,
      notifications: 'none',
    } as SkPerfConfig;
    fetchMock.get('/_/login/status', {
      email: 'someone@example.org',
      roles: ['editor'],
    });
    fetchMock.get('/_/initpage/', {
      dataframe: {
        paramset: {
          config: ['8888', 'gl'],
        },
      },
    });
    fetchMock.post('/_/count', {
      count: 100,
      paramset: {
        config: ['8888', 'gl'],
      },
    });
  });

  afterEach(() => {
    fetchMock.restore();
  });

  const setupElement = async (needAlertAction = false, notifications = 'none', search = '') => {
    window.perf = {
      need_alert_action: needAlertAction,
      notifications: notifications,
    } as SkPerfConfig;
    // Mock window.location.search
    const originalSearch = window.location.search;

    try {
      window.history.replaceState({}, '', search || '/');
      fetchMock.get('/_/alert/list/false', [
        {
          id_as_string: '1',
          display_name: 'Alert 1',
          query: 'config=8888',
          alert: 'sample_alert',
          interesting: 0,
          bug_uri_template: '',
          algo: 'kmeans',
          state: 'ACTIVE',
          owner: 'user@google.com',
          step_up_only: false,
          direction: 'BOTH',
          radius: 10,
          k: 50,
          group_by: '',
          sparse: false,
          minimum_num: 0,
          category: 'Experimental',
          step: '',
          issue_tracker_component: SerializesToString('123'),
        } as Alert,
      ]);
      element = newInstance();
      await fetchMock.flush(true).then(() => {});
      // Wait for Promise.all in connectedCallback to finish.
      await new Promise((resolve) => setTimeout(resolve, 0));
    } finally {
      // Restore window.location.search
      window.history.replaceState({}, '', originalSearch || '/');
    }
  };

  it('renders alerts', async () => {
    await setupElement();
    const rows = element.querySelectorAll(
      'table#alerts-table > tr, table#alerts-table > tbody > tr'
    );
    // Header + 1 alert row.
    assert.equal(rows.length, 2);
    assert.include(rows[1].textContent, 'Alert 1');
  });

  it('displays warning for invalid alert', async () => {
    fetchMock.get(
      '/_/alert/list/false',
      [
        {
          id_as_string: '1',
          display_name: 'Invalid Alert',
          query: '', // Invalid
          owner: 'user@google.com',
          state: 'ACTIVE',
        } as Alert,
      ],
      { overwriteRoutes: true }
    );
    element = newInstance();
    await fetchMock.flush(true).then(() => {});
    await new Promise((resolve) => setTimeout(resolve, 0));

    const rows = element.querySelectorAll(
      'table#alerts-table > tr, table#alerts-table > tbody > tr'
    );
    assert.include(rows[1].textContent, 'An alert must have a non-empty query.');
  });

  it('renders alert action column when needed', async () => {
    await setupElement(true);
    const headers = element.querySelectorAll('table#alerts-table th');
    assert.equal(headers[4].textContent?.trim(), 'Action');
  });

  it('renders component header when notifications is markdown_issuetracker', async () => {
    await setupElement(false, 'markdown_issuetracker');
    const headers = element.querySelectorAll('table#alerts-table th');
    assert.equal(headers[3].textContent?.trim(), 'Component');
    const rows = element.querySelectorAll(
      'table#alerts-table > tr, table#alerts-table > tbody > tr'
    );
    assert.include(rows[1].innerHTML, 'issuetracker.google.com');
  });

  it('opens alert on load if id is in search', async () => {
    await setupElement(false, 'none', '?1');
    const dialog = element.querySelector<HTMLDialogElement>('dialog')!;
    assert.isTrue(dialog.open);
  });

  it('toggles showDeleted', async () => {
    await setupElement();
    fetchMock.get('/_/alert/list/true', [
      {
        id_as_string: '1',
        display_name: 'Alert 1',
        query: 'config=8888',
        alert: 'sample_alert',
        interesting: 0,
        bug_uri_template: '',
        algo: 'kmeans',
        state: 'ACTIVE',
        owner: 'user@google.com',
        step_up_only: false,
        direction: 'BOTH',
        radius: 10,
        k: 50,
        group_by: '',
        sparse: false,
        minimum_num: 0,
        category: 'Experimental',
        step: '',
        issue_tracker_component: SerializesToString('123'),
      } as Alert,
      {
        id_as_string: '2',
        display_name: 'Deleted Alert',
        query: 'config=gl',
        alert: 'sample_alert',
        interesting: 0,
        bug_uri_template: '',
        algo: 'kmeans',
        state: 'DELETED',
        owner: 'user@google.com',
        step_up_only: false,
        direction: 'BOTH',
        radius: 10,
        k: 50,
        group_by: '',
        sparse: false,
        minimum_num: 0,
        category: 'Experimental',
        step: '',
        issue_tracker_component: SerializesToString('123'),
      } as Alert,
    ]);

    const checkbox = element.querySelector<HTMLInputElement>('#showDeletedConfigs')!;
    checkbox.click();
    await fetchMock.flush(true).then(() => {});

    const rows = element.querySelectorAll(
      'table#alerts-table > tr, table#alerts-table > tbody > tr'
    );
    assert.equal(rows.length, 3);
    assert.include(rows[2].textContent, 'Deleted Alert');
    assert.include(rows[2].textContent, 'Archived');
  });

  it('adds a new alert', async () => {
    await setupElement();
    const newAlert: Alert = {
      id_as_string: '-1',
      display_name: 'New Alert',
      query: '',
      alert: 'sample_alert',
      interesting: 0,
      bug_uri_template: '',
      algo: 'kmeans',
      state: 'ACTIVE',
      owner: 'user@google.com',
      step_up_only: false,
      direction: 'BOTH',
      radius: 10,
      k: 50,
      group_by: '',
      sparse: false,
      minimum_num: 0,
      category: 'Experimental',
      step: '',
      issue_tracker_component: SerializesToString('123'),
    } as Alert;
    fetchMock.get('/_/alert/new', newAlert);

    element.querySelector<HTMLButtonElement>('button.action')!.click();
    await fetchMock.flush(true).then(() => {});

    const dialog = element.querySelector<HTMLDialogElement>('dialog')!;
    assert.isTrue(dialog.open);
  });

  it('deletes an alert', async () => {
    await setupElement();
    fetchMock.post('/_/alert/delete/1', 200);
    // After delete, it calls list() again.
    fetchMock.get('/_/alert/list/false', [], { overwriteRoutes: true });

    element.querySelector<HTMLElement>('delete-icon-sk')!.click();
    await fetchMock.flush(true).then(() => {});
    await new Promise((resolve) => setTimeout(resolve, 0));

    const rows = element.querySelectorAll(
      'table#alerts-table > tr, table#alerts-table > tbody > tr'
    );
    assert.equal(rows.length, 1); // Only header remains.
  });

  it('edits and accepts an alert', async () => {
    await setupElement();
    fetchMock.post('/_/alert/update', 200);
    fetchMock.get(
      '/_/alert/list/false',
      [
        {
          id_as_string: '1',
          display_name: 'Updated Alert',
          query: 'config=8888',
          alert: 'sample_alert',
          interesting: 0,
          bug_uri_template: '',
          algo: 'kmeans',
          state: 'ACTIVE',
          owner: 'user@google.com',
          step_up_only: false,
          direction: 'BOTH',
          radius: 10,
          k: 50,
          group_by: '',
          sparse: false,
          minimum_num: 0,
          category: 'Experimental',
          step: '',
          issue_tracker_component: SerializesToString('123'),
        } as Alert,
      ],
      { overwriteRoutes: true }
    );

    element.querySelector<HTMLElement>('create-icon-sk')!.click();
    const dialog = element.querySelector<HTMLDialogElement>('dialog')!;
    assert.isTrue(dialog.open);

    // Mock the change in alert-config-sk
    const alertConfig = element.querySelector<any>('alert-config-sk')!;
    alertConfig.config = {
      ...alertConfig.config,
      display_name: 'Updated Alert',
    };

    element.querySelector<HTMLButtonElement>('button.accept')!.click();
    await fetchMock.flush(true).then(() => {});
    await new Promise((resolve) => setTimeout(resolve, 0));

    assert.isFalse(dialog.open);
    const rows = element.querySelectorAll(
      'table#alerts-table > tr, table#alerts-table > tbody > tr'
    );
    assert.include(rows[1].textContent, 'Updated Alert');
  });

  it('prevents editing if not an editor', async () => {
    await setupElement();
    // Bypassing LoggedIn() cache by manually setting isEditor.
    (element as any).isEditor = false;

    element.querySelector<HTMLElement>('create-icon-sk')!.click();
    const dialog = element.querySelector<HTMLDialogElement>('dialog')!;
    assert.isFalse(dialog.open);
  });

  it('shows error message if accept fails', async () => {
    await setupElement();
    fetchMock.post('/_/alert/update', {
      status: 500,
      body: 'Internal Server Error',
    });

    const errorPromise = new Promise<void>((resolve) => {
      document.addEventListener(
        'error-sk',
        (e) => {
          assert.equal((e as any).detail.message, 'Internal Server Error');
          resolve();
        },
        { once: true }
      );
    });

    element.querySelector<HTMLElement>('create-icon-sk')!.click();
    element.querySelector<HTMLButtonElement>('button.accept')!.click();
    await fetchMock.flush(true).then(() => {});

    assert.isTrue(fetchMock.called('/_/alert/update'));
    await errorPromise;
  });
});
