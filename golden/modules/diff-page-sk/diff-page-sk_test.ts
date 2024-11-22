import './index';
import fetchMock from 'fetch-mock';
import { expect } from 'chai';
import { deepCopy } from '../../../infra-sk/modules/object';
import {
  eventPromise,
  eventSequencePromise,
  setUpElementUnderTest,
} from '../../../infra-sk/modules/test_util';
import {
  DigestComparison,
  TriageRequestV3,
  TriageResponse,
} from '../rpc_types';
import { groupingsResponse } from '../search-page-sk/demo_data';
import { typicalDetails } from '../digest-details-sk/test_data';
import { DiffPageSk } from './diff-page-sk';
import { DiffPageSkPO } from './diff-page-sk_po';

describe('diff-page-sk', () => {
  const newInstance = setUpElementUnderTest<DiffPageSk>('diff-page-sk');

  let diffPageSk: DiffPageSk;
  let diffPageSkPO: DiffPageSkPO;

  beforeEach(async () => {
    diffPageSk = newInstance();
    diffPageSkPO = new DiffPageSkPO(diffPageSk);

    fetchMock.getOnce('/json/v1/groupings', () => deepCopy(groupingsResponse));
    const digestComparison: DigestComparison = {
      left: deepCopy(typicalDetails),
      right: deepCopy(typicalDetails.refDiffs?.pos)!,
    };
    fetchMock.post('glob:/json/v2/diff*', digestComparison);

    // Wait for the above RPCs to complete.
    await eventSequencePromise(['end-task', 'end-task']);
  });

  afterEach(() => {
    expect(fetchMock.done()).to.be.true; // All mock RPCs called at least once.
    fetchMock.reset();
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
    };
    const triageResponse: TriageResponse = { status: 'ok' };
    fetchMock.post(
      { url: '/json/v3/triage', body: triageRequest },
      { status: 200, body: triageResponse }
    );

    const endTask = eventPromise('end-task');
    await diffPageSkPO.digestDetailsSkPO.triageSkPO.clickButton('negative');
    await endTask;
  });

  it('should show overview layout by default', async () => {
    expect(await diffPageSkPO.digestDetailsSkPO.hasClassName('overview')).to.be
      .true;
  });

  it('should change to full width view if image size is toggled', async () => {
    await diffPageSkPO.digestDetailsSkPO.imageCompareSkPO.clickImage(0);
    expect(await diffPageSkPO.digestDetailsSkPO.hasClassName('overview')).to.be
      .false;
  });
});
