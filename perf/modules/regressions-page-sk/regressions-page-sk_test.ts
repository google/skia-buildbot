import './index';
import { assert } from 'chai';
import fetchMock from 'fetch-mock';
import { RegressionsPageSk } from './regressions-page-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';

describe('regressions-page-sk', () => {
  const newInstance = setUpElementUnderTest<RegressionsPageSk>(
    'regressions-page-sk'
  );

  let element: RegressionsPageSk;

  beforeEach(() => {
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

    fetchMock.get(
      `/_/regressions?sub_name=Sheriff%20Config%202&limit=10&offset=0`,
      () => [
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
      ]
    );

    element = newInstance((el: RegressionsPageSk) => {});
  });

  describe('RegressionsPageSK', () => {
    it('Loads associated regressions when subscription selected', async () => {
      const dropdown = document.getElementById('filter');
      // Three options and two additional node: the non-selected option and the Lit comment node
      assert.equal(dropdown?.childNodes.length, 5);
      assert.equal(element.regressions.length, 0);

      await element.filterChange('Sheriff Config 2');
      assert.equal(element.regressions.length, 1);
    });
  });

  describe('isRegressionImprovement', () => {
    it('');
  });
});
