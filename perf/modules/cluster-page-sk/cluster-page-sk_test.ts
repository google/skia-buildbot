import './index';
import { assert } from 'chai';
import fetchMock from 'fetch-mock';
import { ClusterPageSk } from './cluster-page-sk';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import {
  CommitNumber,
  TimestampSeconds,
  TraceSet,
  ReadOnlyParamSet,
  RegressionDetectionResponse,
} from '../json';

describe('cluster-page-sk', () => {
  const newInstance = setUpElementUnderTest<ClusterPageSk>('cluster-page-sk');

  let element: ClusterPageSk;

  beforeEach(() => {
    window.perf = {
      radius: 10,
      interesting: 20,
      key_order: ['config'],
    } as any;

    fetchMock.get('glob:/_/initpage/?*', {
      dataframe: {
        paramset: {
          config: ['8888', 'gl'],
        },
      },
    });

    fetchMock.get('/_/login/status', {
      email: 'someone@example.org',
      roles: ['editor'],
    });

    fetchMock.post('/_/cidRange/', [
      {
        author: 'alice@example.com',
        message: 'Fixed a bug',
        url: 'https://skia.googlesource.com/infra/+/1',
        ts: 1600000000,
        hash: '1111111111111111',
        offset: CommitNumber(100),
        body: 'Body 1',
      } as any,
    ]);

    fetchMock.post('/_/cid/', {
      commitSlice: [],
      logEntry: '',
    });

    fetchMock.post(
      'path:/_/count',
      {
        count: 100,
        paramset: {
          config: ['8888', 'gl'],
        },
      },
      { overwriteRoutes: true }
    );
  });

  afterEach(() => {
    fetchMock.restore();
  });

  const setupElement = async () => {
    element = newInstance();
    const _ = await fetchMock.flush(true);
    // Wait for Promise.all in connectedCallback to finish.
    await new Promise((resolve) => setTimeout(resolve, 0));
  };

  it('renders initial state', async () => {
    await setupElement();
    assert.include(element.textContent, 'Commit');
    assert.include(element.textContent, 'Algorithm');
    assert.include(element.textContent, 'Query');
    assert.include(element.textContent, 'No clusters found.');
  });

  it('runs a cluster request', async () => {
    await setupElement();

    // Set offset to avoid disabled Run button
    (element as any).state.offset = 100;
    (element as any).state.query = 'config=8888';
    (element as any)._render();

    const response: RegressionDetectionResponse = {
      summary: {
        Clusters: [
          {
            centroid: [1.0],
            shortcut: 'shortcut',
            param_summaries2: [],
            step_fit: {
              least_squares: 0,
              turning_point: 0,
              step_size: 0,
              regression: 0,
              status: 'Low',
            },
            step_point: {
              offset: CommitNumber(100),
              timestamp: TimestampSeconds(0),
              hash: 'abcdef',
              author: 'author',
              message: 'message',
              url: 'url',
            },
            num: 10,
            ts: '2020-05-01T00:00:00Z',
          },
        ],
        StdDevThreshold: 0,
        K: 0,
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
    };

    fetchMock.post('/_/cluster/start', {
      id: 'request-123',
      url: '/_/progress/request-123',
      status: 'Running',
      messages: [{ key: 'Status', value: 'Running' }],
    });

    fetchMock.get('/_/progress/request-123', {
      id: 'request-123',
      url: '/_/progress/request-123',
      status: 'Finished',
      messages: [{ key: 'Status', value: 'Finished' }],
      results: response,
    });

    element.querySelector<HTMLButtonElement>('button#start')!.click();
    const _ = await fetchMock.flush(true);
    // Wait for the polling timeout.
    await new Promise((resolve) => setTimeout(resolve, 400));
    const __ = await fetchMock.flush(true);
    await new Promise((resolve) => setTimeout(resolve, 0));

    const clusters = element.querySelectorAll('cluster-summary2-sk');
    assert.equal(clusters.length, 1);
  });

  it('updates state on input', async () => {
    await setupElement();
    const kInput = element.querySelector<HTMLInputElement>('#k_input')!;
    kInput.value = '5';
    kInput.dispatchEvent(new InputEvent('input'));
    assert.equal((element as any).state.k, 5);

    const radiusInput = element.querySelector<HTMLInputElement>('#radius_input')!;
    radiusInput.value = '15';
    radiusInput.dispatchEvent(new InputEvent('input'));
    assert.equal((element as any).state.radius, 15);

    const interestingInput = element.querySelector<HTMLInputElement>('#interesting_input')!;
    interestingInput.value = '25';
    interestingInput.dispatchEvent(new InputEvent('input'));
    assert.equal((element as any).state.interesting, 25);

    const _sparseCheckbox = element.querySelector<any>('checkbox-sk')!;
    (element as any).sparseChange({ target: { checked: true } });
    assert.isTrue((element as any).state.sparse);
  });

  it('handles algo-change event', async () => {
    await setupElement();
    const algoSelect = element.querySelector('algo-select-sk')!;
    algoSelect.dispatchEvent(
      new CustomEvent('algo-change', {
        detail: { algo: 'stepfit' },
      })
    );
    assert.equal((element as any).state.algo, 'stepfit');
  });

  it('handles query-change event', async () => {
    await setupElement();
    const querySk = element.querySelector('query-sk')!;
    querySk.dispatchEvent(
      new CustomEvent('query-change', {
        detail: { q: 'config=gl' },
      })
    );
    assert.equal((element as any).state.query, 'config=gl');
  });

  it('handles commit-selected event from picker', async () => {
    await setupElement();
    const picker = element.querySelector('commit-detail-picker-sk')!;
    picker.dispatchEvent(
      new CustomEvent('commit-selected', {
        detail: { commit: { offset: 500 } },
      })
    );
    assert.equal((element as any).state.offset, 500);
  });
});
