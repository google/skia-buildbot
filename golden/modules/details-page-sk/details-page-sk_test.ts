import './index';
import fetchMock from 'fetch-mock';
import { expect } from 'chai';
import { deepCopy } from '../../../infra-sk/modules/object';
import {
  eventPromise,
  eventSequencePromise,
  setQueryString,
  setUpElementUnderTest,
} from '../../../infra-sk/modules/test_util';
import { DigestDetails, TriageRequestV3, TriageResponse } from '../rpc_types';
import { DetailsPageSk } from './details-page-sk';
import { DetailsPageSkPO } from './details-page-sk_po';
import { groupingsResponse } from '../search-page-sk/demo_data';
import {
  twoHundredCommits,
  typicalDetails,
} from '../digest-details-sk/test_data';

describe('details-page-sk', () => {
  const newInstance = setUpElementUnderTest<DetailsPageSk>('details-page-sk');

  let detailsPageSk: DetailsPageSk;
  let detailsPageSkPO: DetailsPageSkPO;

  const initialize = async (queryString: string) => {
    setQueryString(queryString);

    detailsPageSk = newInstance();
    detailsPageSkPO = new DetailsPageSkPO(detailsPageSk);

    fetchMock.getOnce('/json/v1/groupings', () => deepCopy(groupingsResponse));
    const digestDetails: DigestDetails = {
      digest: deepCopy(typicalDetails),
      commits: deepCopy(twoHundredCommits),
    };
    fetchMock.post('/json/v2/details', digestDetails);

    // Wait for the above RPCs to complete.
    await eventSequencePromise(['end-task', 'end-task']);
  };

  const addTests = () => {
    it('renders', async () => {
      expect(await detailsPageSkPO.digestDetailsSkPO.getTestName()).to.equal(
        'Test: dots-legend-sk_too-many-digests'
      );
    });

    it('can triage', async () => {
      // This tests the wiring that passes the groupings returned by the /json/v1/groupings RPC to
      // the digest-details-sk element.
      const triageRequest: TriageRequestV3 = {
        deltas: [
          {
            grouping: {
              source_type: 'infra',
              name: 'dots-legend-sk_too-many-digests',
            },
            digest: '6246b773851984c726cb2e1cb13510c2',
            label_before: 'positive',
            label_after: 'negative',
          },
        ],
        changelist_id: '12353',
        crs: 'gerrit-internal',
      };
      const triageResponse: TriageResponse = { status: 'ok' };
      fetchMock.post(
        { url: '/json/v3/triage', body: triageRequest },
        { status: 200, body: triageResponse }
      );

      const endTask = eventPromise('end-task');
      await detailsPageSkPO.digestDetailsSkPO.triageSkPO.clickButton(
        'negative'
      );
      await endTask;
    });

    it('should show overview layout by default', async () => {
      expect(await detailsPageSkPO.digestDetailsSkPO.hasClassName('overview'))
        .to.be.true;
    });

    it('should change to full width view if image size is toggled', async () => {
      await detailsPageSkPO.digestDetailsSkPO.imageCompareSkPO.clickImage(0);
      expect(await detailsPageSkPO.digestDetailsSkPO.hasClassName('overview'))
        .to.be.false;
    });
  };

  afterEach(() => {
    expect(fetchMock.done()).to.be.true; // All mock RPCs called at least once.
    fetchMock.reset();
  });

  describe('with grouping in URL', () => {
    beforeEach(async () => {
      await initialize(
        '?digest=6246b773851984c726cb2e1cb13510c2&' +
          'grouping=name%3Ddots-legend-sk_too-many-digests%26source_type%3Dinfra&' +
          'changelist_id=12353&crs=gerrit-internal'
      );
    });

    addTests();
  });
});
