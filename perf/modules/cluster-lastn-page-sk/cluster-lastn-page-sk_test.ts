import './index';
import { assert } from 'chai';
import fetchMock from 'fetch-mock';
import { ClusterLastNPageSk } from './cluster-lastn-page-sk';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import {
  Alert,
  SerializesToString,
  RegressionAtCommit,
  CommitNumber,
  TraceSet,
  ReadOnlyParamSet,
} from '../json';

describe('cluster-lastn-page-sk', () => {
  const newInstance = setUpElementUnderTest<ClusterLastNPageSk>('cluster-lastn-page-sk');

  let element: ClusterLastNPageSk;

  beforeEach(() => {
    window.perf = {
      key_order: ['config'],
    } as any;
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
    fetchMock.get('/_/alert/new', {
      id_as_string: '-1',
      display_name: 'New Alert',
      query: 'config=8888',
      alert: 'sample_alert',
      interesting: 0,
      bug_uri_template: '',
      algo: 'kmeans',
      state: 'ACTIVE',
      owner: 'someone@example.org',
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
    } as Alert);
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

  const setupElement = async () => {
    element = newInstance();
    const _ = await fetchMock.flush(true);
    // Wait for Promise.all in connectedCallback to finish.
    await new Promise((resolve) => setTimeout(resolve, 100));
  };

  it('renders initial state', async () => {
    await setupElement();
    assert.include(element.textContent, 'Use this page to test out an Alert configuration');
    assert.include(element.textContent, 'Algo: original/kmeans');
  });

  it('opens alert config dialog', async () => {
    await setupElement();
    const dialog = element.querySelector<HTMLDialogElement>('#alert-config-dialog')!;
    assert.isFalse(dialog.open);

    element.querySelector<HTMLButtonElement>('#config-button')!.click();
    assert.isTrue(dialog.open);
  });

  it('runs a dryrun', async () => {
    await setupElement();

    const regression: RegressionAtCommit = {
      cid: {
        offset: CommitNumber(100),
        hash: 'abcdef123456',
        ts: 1600000000,
        author: 'alice@example.com',
        message: 'Fixed a bug',
        url: 'http://example.com/c/100',
        body: '',
      },
      regression: {
        sub_name: '',
        low: null,
        high: {
          centroid: [1.0, 2.0],
          shortcut: 'shortcut',
          param_summaries2: [],
          step_fit: null,
          step_point: null,
          num: 10,
          ts: '2020-05-01T00:00:00Z',
        },
        frame: {
          dataframe: {
            traceset: TraceSet({}),
            header: [],
            paramset: ReadOnlyParamSet({}),
            skip: 0,
            traceMetadata: [],
          },
          skps: [],
          msg: '',
          display_mode: 'display_plot',
          anomalymap: {},
        },
        low_status: { status: '', message: '' },
        high_status: { status: 'untriaged', message: '' },
        id: 'reg-1',
        commit_number: CommitNumber(100),
        prev_commit_number: CommitNumber(99),
        alert_id: -1,
        bugs: [],
        all_bugs_fetched: true,
        creation_time: '2020-05-01T00:00:00Z',
        median_before: 10,
        median_after: 20,
        is_improvement: false,
        cluster_type: 'high',
      },
    };

    fetchMock.post(
      '/_/dryrun/start',
      (_url, _opts) => {
        return {
          id: 'request-123',
          url: '/_/progress/request-123',
          status: 'Finished',
          messages: [{ key: 'Status', value: 'Finished' }],
          results: [regression],
        };
      },
      { overwriteRoutes: true }
    );

    const runButton = element.querySelector<HTMLButtonElement>('#run-button')!;
    runButton.click();
    const _ = await fetchMock.flush(true);
    await new Promise((resolve) => setTimeout(resolve, 100));

    const rows = element.querySelectorAll('table#regressions-table tr');
    // Header + Header2 + 1 regression row.
    assert.equal(rows.length, 3);
    assert.include(rows[2].textContent, 'alice@example.com');
  });

  it('saves an alert', async () => {
    await setupElement();
    fetchMock.post('/_/alert/update', {
      IDAsString: '123',
    });

    const saveButton = element.querySelectorAll<HTMLButtonElement>('div.saving button')[0];
    saveButton.click();
    const _ = await fetchMock.flush(true);

    assert.include(saveButton.textContent, 'Update Alert');
  });

  it('starts triage when triage-status-sk emits event', async () => {
    await setupElement();
    const triageData = { status: 'untriaged' as const, message: '' };
    const fullSummary: any = { summary: { num: 10 } };

    (element as any).triageStart(
      new CustomEvent('start-triage', {
        detail: {
          full_summary: fullSummary,
          triage: triageData,
          cluster_type: 'high',
        },
      })
    );
    await Promise.resolve();

    assert.isTrue(element.querySelector<HTMLDialogElement>('#triage-cluster-dialog')!.open);
  });

  it('closes triage dialog', async () => {
    await setupElement();
    const triageDialog = element.querySelector<HTMLDialogElement>('#triage-cluster-dialog')!;
    triageDialog.show();
    assert.isTrue(triageDialog.open);

    (element as any).triageClose();
    assert.isFalse(triageDialog.open);
  });
});
