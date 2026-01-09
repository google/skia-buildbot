import './index';
import { ClusterSummary2Sk } from './cluster-summary2-sk';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { assert } from 'chai';
import {
  ClusterSummary,
  FrameResponse,
  FullSummary,
  TriageStatus,
  Alert,
  TraceSet,
  CommitNumber,
  TimestampSeconds,
  ReadOnlyParamSet,
  SerializesToString,
} from '../json';
import fetchMock from 'fetch-mock';

describe('cluster-summary2-sk', () => {
  const newInstance = setUpElementUnderTest<ClusterSummary2Sk>('cluster-summary2-sk');

  let element: ClusterSummary2Sk;

  const summary: ClusterSummary = {
    centroid: [1, 2, 3],
    shortcut: 'X123',
    param_summaries2: [],
    step_fit: {
      regression: 10,
      least_squares: 0.1,
      step_size: 5,
      turning_point: 1,
      status: 'High',
    },
    step_point: {
      offset: CommitNumber(100),
      timestamp: TimestampSeconds(1234567890),
      author: 'me',
      hash: 'abc',
      message: 'commit message',
      url: 'http://commit',
    },
    num: 10,
    ts: '2023-01-01T00:00:00Z',
    notification_id: '123',
  };

  const frame: FrameResponse = {
    dataframe: {
      traceset: TraceSet({}),
      header: [
        {
          offset: CommitNumber(99),
          timestamp: TimestampSeconds(1234567880),
          author: 'me',
          hash: 'aaa',
          message: 'prev',
          url: 'http://prev',
        },
        {
          offset: CommitNumber(100),
          timestamp: TimestampSeconds(1234567890),
          author: 'me',
          hash: 'abc',
          message: 'commit message',
          url: 'http://commit',
        },
      ],
      paramset: ReadOnlyParamSet({}),
      skip: 0,
      traceMetadata: [],
    },
    skps: [],
    msg: '',
    display_mode: 'display_plot',
    anomalymap: {},
  };

  const triage: TriageStatus = {
    status: 'untriaged',
    message: 'some message',
  };

  const fullSummary: FullSummary = {
    summary: summary,
    triage: triage,
    frame: frame,
  };

  const alert: Alert = {
    id_as_string: '1',
    display_name: 'Alert 1',
    query: '',
    alert: '',
    step: '',
    radius: 0,
    k: 0,
    algo: 'kmeans',
    interesting: 0,
    sparse: false,
    bug_uri_template: '',
    state: 'ACTIVE',
    owner: '',
    step_up_only: false,
    direction: 'BOTH',
    group_by: '',
    minimum_num: 0,
    category: '',
    action: 'noaction',
    issue_tracker_component: SerializesToString(''),
  };

  beforeEach(async () => {
    // Mock login
    fetchMock.get('/_/login/status', {
      email: 'user@google.com',
      roles: ['editor'],
    });

    fetchMock.post('/_/cid/', {
      commitSlice: [],
      logEntry: '',
    });

    element = newInstance();
    // Wait for login check
    await fetchMock.flush(true);
  });

  afterEach(() => {
    fetchMock.reset();
  });

  it('renders correctly with data', async () => {
    element.full_summary = fullSummary;
    element.alert = alert;

    assert.include(element.innerHTML, 'Cluster Size');
    assert.include(element.innerHTML, '10'); // summary.num
    assert.include(element.innerHTML, 'Regression Factor:');
  });

  it('displays bug link if notification_id is present', async () => {
    element.full_summary = fullSummary;
    element.alert = alert;

    const bugLink = element.querySelector('a[href="http://b/123"]');
    assert.isNotNull(bugLink);
    assert.equal(bugLink?.textContent, 'b/123');
  });

  it('fires open-keys event on shortcut click', async () => {
    element.full_summary = fullSummary;
    element.alert = alert;

    let eventCaught = false;
    element.addEventListener('open-keys', (e: any) => {
      eventCaught = true;
      assert.equal(e.detail.shortcut, 'X123');
    });

    const btn = element.querySelector('#shortcut') as HTMLElement;
    btn.click();
    assert.isTrue(eventCaught);
  });

  it('fires triaged event on update click', async () => {
    element.full_summary = fullSummary;
    element.alert = alert;

    let eventCaught = false;
    element.addEventListener('triaged', (e: any) => {
      eventCaught = true;
      // We expect the original triage status because we haven't changed the inputs
      assert.equal(e.detail.triage.status, 'untriaged');
    });

    const btn = element.querySelector('button.action') as HTMLElement;
    btn.click();
    assert.isTrue(eventCaught);
  });

  it('handles null step_fit without crashing', async () => {
    const summaryNoStepFit = JSON.parse(JSON.stringify(summary));
    summaryNoStepFit.step_fit = null;
    const fullSummaryNoStepFit: FullSummary = {
      summary: summaryNoStepFit,
      triage: triage,
      frame: frame,
    };

    element.full_summary = fullSummaryNoStepFit;

    // Should render without errors, but might be missing data.
    // Check that we didn't crash and rendered something.
    assert.include(element.innerHTML, 'Cluster Size');
  });
});
