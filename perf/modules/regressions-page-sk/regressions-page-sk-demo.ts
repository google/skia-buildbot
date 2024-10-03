import './index';
import fetchMock from 'fetch-mock';

fetchMock.get('/_/subscriptions', () => [
  {
    name: 'Sheriff Config 1',
    revision: 'rev1',
    bug_labels: ['A', 'B'],
    hotlists: ['C', 'D'],
    bug_components: 'Component1>Subcomponent1',
    bug_priority: 1,
    bug_severity: 2,
    bug_cc_emails: ['abcd@efg.com', '1234@567.com'],
    contact_email: 'test@owner.com',
  },
  {
    name: 'Sheriff Config 2',
    revision: 'rev2',
    bug_labels: ['1', '2'],
    hotlists: ['3', '4'],
    bug_components: 'Component2>Subcomponent2',
    bug_priority: 1,
    bug_severity: 2,
    bug_cc_emails: ['abcd@efg.com', '1234@567.com'],
    contact_email: 'test@owner.com',
  },
  {
    name: 'Sheriff Config 3',
    revision: 'rev3',
    bug_labels: ['1', '2'],
    hotlists: ['3', '4'],
    bug_components: 'Component3>Subcomponent3',
    bug_priority: 1,
    bug_severity: 2,
    bug_cc_emails: ['abcd@efg.com', '1234@567.com'],
    contact_email: 'test@owner.com',
  },
]);

fetchMock.get(`/_/regressions?sub_name=Sheriff%20Config%201&limit=10&offset=0`, [
  {
    id: 'id1',
    commit_number: 1234,
    prev_commit_number: 1236,
    alert_id: 1,
    creation_time: '',
    median_before: 123,
    median_after: 135,
    is_improvement: true,
    cluster_type: 'high',
    frame: {
      dataframe: {
        paramset: {
          bot: ['bot1'],
          benchmark: ['benchmark1'],
          test: ['test1'],
          improvement_direction: ['up'],
        },
        traceset: {},
        header: null,
        skip: 1,
      },
      skps: [1],
      msg: '',
      anomalymap: null,
    },
    high: {
      centroid: null,
      shortcut: 'shortcut 1',
      param_summaries2: null,
      step_fit: {
        status: 'High',
        least_squares: 123,
        regression: 12,
        step_size: 345,
        turning_point: 1234,
      },
      step_point: null,
      num: 156,
      ts: 'test',
    },
  },
]);

fetchMock.get(`/_/regressions?sub_name=Sheriff%20Config%202&limit=10&offset=0`, [
  {
    id: 'id2',
    commit_number: 1235,
    prev_commit_number: 1237,
    alert_id: 1,
    creation_time: '',
    median_before: 123,
    median_after: 135,
    is_improvement: true,
    cluster_type: 'high',
    frame: {
      dataframe: {
        paramset: {
          bot: ['bot1'],
          benchmark: ['benchmark1'],
          test: ['test1'],
          improvement_direction: ['up'],
        },
        traceset: {},
        header: null,
        skip: 1,
      },
      skps: [1],
      msg: '',
      anomalymap: null,
    },
    high: {
      centroid: null,
      shortcut: 'shortcut 1',
      param_summaries2: null,
      step_fit: {
        status: 'High',
        least_squares: 123,
        regression: 12,
        step_size: 345,
        turning_point: 1234,
      },
      step_point: null,
      num: 156,
      ts: 'test',
    },
  },
]);

fetchMock.get(`/_/regressions?sub_name=Sheriff%20Config%203&limit=10&offset=0`, []);

document.querySelector('.component-goes-here')!.innerHTML =
  '<regressions-page-sk></regressions-page-sk>';
