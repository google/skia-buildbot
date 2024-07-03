import './index';
import { assert } from 'chai';
import fetchMock from 'fetch-mock';
import { RevisionInfoSk } from './revision-info-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { GraphConfig, RevisionInfo } from '../json';

describe('revision-info-sk', () => {
  const newInstance = setUpElementUnderTest<RevisionInfoSk>('revision-info-sk');

  let element: RevisionInfoSk;
  beforeEach(() => {
    element = newInstance((el: RevisionInfoSk) => {
      // Place here any code that must run after the element is instantiated but
      // before it is attached to the DOM (e.g. property setter calls,
      // document-level event listeners, etc.).
    });
  });

  describe('Send Requests', () => {
    it('Single RevInfo', async () => {
      const revId = '12345';

      const response: RevisionInfo[] = [
        {
          benchmark: 'b1',
          bot: 'bot1',
          bug_id: '111',
          end_revision: 456,
          start_revision: 123,
          start_time: 1712026352,
          end_time: 1712285552,
          explore_url: 'https://url',
          is_improvement: false,
          master: 'm1',
          test: 't1',
          query: 'master=m1&bot=bot1&benchmark=b1&test=t1',
        },
      ];

      fetchMock.get(`/_/revision/?rev=${revId}`, response);
      element.revisionId!.value = revId;
      await element.getRevisionInfo();

      assert.deepEqual(element.revisionInfos, response);
    });
  });

  describe('Multigraph View', () => {
    it('getGraphConfigs', () => {
      const revisionInfos: RevisionInfo[] = [
        {
          benchmark: 'b1',
          bot: 'bot1',
          bug_id: '111',
          end_revision: 456,
          start_revision: 123,
          start_time: 1712026352,
          end_time: 1712285552,
          explore_url: 'https://url',
          is_improvement: false,
          master: 'm1',
          test: 't1/t2/t3',
          query:
            'master=m1&bot=bot1&benchmark=b1&test=t1&subtest_1=t2&subtest_2=t3',
        },
        {
          benchmark: 'b1',
          bot: 'bot1',
          bug_id: '111',
          end_revision: 456,
          start_revision: 123,
          start_time: 1713235952,
          end_time: 1713408752,
          explore_url: 'https://url',
          is_improvement: false,
          master: 'm1',
          test: 't5',
          query: 'master=m1&bot=bot1&benchmark=b1&test=t5',
        },
      ];

      const resp: GraphConfig[] = [
        {
          queries: [
            'master=m1&bot=bot1&benchmark=b1&test=t1&subtest_1=t2&subtest_2=t3',
          ],
          formulas: [],
          keys: '',
        },
        {
          queries: ['master=m1&bot=bot1&benchmark=b1&test=t5'],
          formulas: [],
          keys: '',
        },
      ];

      const graphConfigs = element.getGraphConfigs(revisionInfos);
      assert.deepEqual(resp, graphConfigs);
    });

    it('getMultiGraphUrl', async () => {
      const revisionInfos: RevisionInfo[] = [
        {
          benchmark: 'b1',
          bot: 'bot1',
          bug_id: '111',
          end_revision: 456,
          start_revision: 123,
          start_time: 1712026352,
          end_time: 1712285552,
          explore_url: 'https://url',
          is_improvement: false,
          master: 'm1',
          test: 't1/t2/t3',
          query:
            'master=m1&bot=bot1&benchmark=b1&test=t1&subtest_1=t2&subtest_2=t3',
        },
        {
          benchmark: 'b1',
          bot: 'bot1',
          bug_id: '111',
          end_revision: 456,
          start_revision: 123,
          start_time: 1713235952,
          end_time: 1713408752,
          explore_url: 'https://url',
          is_improvement: false,
          master: 'm1',
          test: 't5',
          query: 'master=m1&bot=bot1&benchmark=b1&test=t5',
        },
      ];

      fetchMock.post(`/_/shortcut/update`, { id: '1234567' });

      const url = await element.getMultiGraphUrl(revisionInfos);
      const expected =
        'begin=1712026352&end=1713408752&shortcut=1234567&totalGraphs=2';

      assert.include(url, expected);
    });
  });
});
