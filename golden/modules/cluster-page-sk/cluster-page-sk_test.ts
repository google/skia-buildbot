import './index';

import fetchMock from 'fetch-mock';

import { expect } from 'chai';
import { deepCopy } from 'common-sk/modules/object';
import {
  eventPromise,
  setQueryString,
  expectQueryStringToEqual,
  setUpElementUnderTest,
  eventSequencePromise,
} from '../../../infra-sk/modules/test_util';
import { clusterDiffJSON, negativeDigest, positiveDigest } from './test_data';
import { testOnlySetSettings } from '../settings';
import { ClusterPageSk } from './cluster-page-sk';
import { ClusterPageSkPO } from './cluster-page-sk_po';
import {
  DigestComparison, DigestDetails, TriageRequestV3, TriageResponse,
} from '../rpc_types';
import { twoHundredCommits, typicalDetails } from '../digest-details-sk/test_data';
import { groupingsResponse } from '../search-page-sk/demo_data';

describe('cluster-page-sk', () => {
  const newInstance = setUpElementUnderTest<ClusterPageSk>('cluster-page-sk');

  let clusterPageSk: ClusterPageSk;
  let clusterPageSkPO: ClusterPageSkPO;

  beforeEach(async () => {
    testOnlySetSettings({
      defaultCorpus: 'infra',
    });
    // Clear out any query params we might have to not mess with our current state.
    // This page always requires a grouping to be set.
    setQueryString('?grouping=some-test');

    // These are the default RPC calls when the page loads.
    fetchMock.get(
      '/json/v2/clusterdiff?head=true'
        + '&include=false&neg=false&pos=false&query=name%3Dsome-test'
        + '&source_type=infra&unt=false',
      clusterDiffJSON,
    );
    fetchMock.get('/json/v2/paramset', clusterDiffJSON.paramsetsUnion);
    fetchMock.getOnce('/json/v1/groupings', groupingsResponse);

    // Instantiate page; wait for RPCs to complete and for the page to render.
    const endTask = eventSequencePromise(['end-task', 'end-task', 'end-task']);
    clusterPageSk = newInstance();
    clusterPageSkPO = new ClusterPageSkPO(clusterPageSk);
    await endTask;

    // Give the cluster-digests-sk component a chance to render the initial SVG elements.
    await new Promise((resolve) => setTimeout(resolve, 100));

    // Wait for initial page load to finish.
    await fetchMock.flush(true);
  });

  afterEach(async () => {
    // Completely remove the mocking which allows each test
    // to be able to mess with the mocked routes w/o impacting other tests.
    fetchMock.reset();

    // Make sure all subsequent RPC calls happen.
    await fetchMock.flush(true);
  });

  it('shows the paramset', async () => {
    expect(await clusterPageSkPO.paramSetSkPO.getParamSets()).to.deep.equal([{
      ext: ['png'],
      gpu: ['AMD', 'nVidia'],
      name: ['dots-legend-sk_too-many-digests'],
      source_type: ['infra', 'some-other-corpus'],
    }]);
  });

  it('removes corpus and test name from the paramset passed to the search controls', async () => {
    await clusterPageSkPO.searchControlsSkPO.traceFilterSkPO.clickEditBtn();
    const paramset = await clusterPageSkPO
      .searchControlsSkPO
      .traceFilterSkPO
      .queryDialogSkPO
      .querySkPO
      .getParamSet();
    expect(paramset).to.deep.equal({
      ext: ['png'],
      gpu: ['AMD', 'nVidia'],
    });
  });

  it('changes what it fetches based on search controls', async () => {
    // We only spot-check that one field in the search-controls-sk component is correctly wired.
    //
    // The behaviors spanning across SearchControlsSk, SearchCriteria and SearchResponse are
    // thoroughly tested in search-page-sk_tests.ts. There is no need to repeat those tests here.

    fetchMock.get('/json/v2/clusterdiff?head=false'
        + '&include=false&neg=false&pos=false&query=name%3Dsome-test'
        + '&source_type=infra&unt=false',
    clusterDiffJSON);
    await clusterPageSkPO.searchControlsSkPO.clickIncludeDigestsNotAtHeadCheckbox();
    expectQueryStringToEqual('?corpus=infra&grouping=some-test&max_rgba=255&not_at_head=true');
  });

  it('makes an RPC for details when the selection is changed to one digest', async () => {
    fetchMock.get('/json/v2/details?corpus=infra'
      + `&digest=${positiveDigest}&test=some-test`, {
      'these-details': 'do not matter for this test',
    });

    await clusterPageSkPO.clusterDigestsSkPO.clickNode(positiveDigest);
  });

  it('makes an RPC for a diff when the selection is changed to two digests', async () => {
    fetchMock.get('/json/v2/details?corpus=infra'
        + `&digest=${positiveDigest}&test=some-test`, {
      'these-details': 'do not matter for this test',
    });

    await clusterPageSkPO.clusterDigestsSkPO.clickNode(positiveDigest);

    fetchMock.get('/json/v2/diff?corpus=infra'
        + `&left=${positiveDigest}`
        + `&right=${negativeDigest}&test=some-test`, {
      'these-details': 'do not matter for this test',
    });

    await clusterPageSkPO.clusterDigestsSkPO.shiftClickNode(negativeDigest);
  });

  describe('triaging', async () => {
    // These tests exercise the wiring that passes the groupings returned by the /json/v1/groupings
    // RPC to the digest-details-sk element.

    beforeEach(() => {
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
        { status: 200, body: triageResponse },
      );

      const digestDetails: DigestDetails = {
        digest: deepCopy(typicalDetails),
        commits: deepCopy(twoHundredCommits),
      };
      fetchMock.get('glob:/json/v2/details*', digestDetails);
    });

    it('can triage with one digest selected', async () => {
      let endTask = eventPromise('end-task');
      await clusterPageSkPO.clusterDigestsSkPO.clickNode(positiveDigest);
      await endTask;

      endTask = eventPromise('end-task');
      await clusterPageSkPO.digestDetailsSkPO.triageSkPO.clickButton('negative');
      await endTask;
    });

    it('can triage with two digests selected', async () => {
      let endTask = eventPromise('end-task');
      await clusterPageSkPO.clusterDigestsSkPO.clickNode(positiveDigest);
      await endTask;

      const digestComparison: DigestComparison = {
        left: deepCopy(typicalDetails),
        right: deepCopy(typicalDetails.refDiffs?.pos)!,
      };
      fetchMock.get('glob:/json/v2/diff*', digestComparison);

      endTask = eventPromise('end-task');
      await clusterPageSkPO.clusterDigestsSkPO.shiftClickNode(negativeDigest);
      await endTask;

      endTask = eventPromise('end-task');
      await clusterPageSkPO.digestDetailsSkPO.triageSkPO.clickButton('negative');
      await endTask;
    });
  });
});
