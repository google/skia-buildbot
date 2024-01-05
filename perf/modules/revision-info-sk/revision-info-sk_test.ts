import './index';
import { assert } from 'chai';
import fetchMock from 'fetch-mock';
import { RevisionInfoSk } from './revision-info-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { RevisionInfo } from '../json';

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
          explore_url: 'https://url',
          is_improvement: false,
          master: 'm1',
          test: 't1',
        },
      ];

      fetchMock.get(`/_/revision/?rev=${revId}`, response);
      element.revisionId!.value = revId;
      await element.getRevisionInfo();

      assert.deepEqual(element.revisionInfos, response);
    });
  });
});
