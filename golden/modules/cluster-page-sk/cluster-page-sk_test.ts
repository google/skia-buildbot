import './index';

import fetchMock from 'fetch-mock';

import {
  eventPromise,
  setQueryString,
  expectQueryStringToEqual,
  setUpElementUnderTest,
} from '../../../infra-sk/modules/test_util';
import {clusterDiffJSON, negativeDigest, positiveDigest} from './test_data';
import { testOnlySetSettings } from '../settings';
import {ClusterPageSk} from './cluster-page-sk';
import { expect } from 'chai';
import {ClusterPageSkPO} from './cluster-page-sk_po';

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
        '/json/v1/clusterdiff?head=true'
        + '&include=false&neg=false&pos=false&query=name%3Dsome-test'
        + '&source_type=infra&unt=false',
        clusterDiffJSON);
    fetchMock.get('/json/v1/paramset', clusterDiffJSON.paramsetsUnion);

    // Instantiate page; wait for RPCs to complete and for the page to render.
    const endTask = eventPromise('end-task');
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
  })

  it('removes corpus and test name from the paramset passed to the search controls', async () => {
    await clusterPageSkPO.searchControlsSkPO.traceFilterSkPO.clickEditBtn();
    const paramset =
        await clusterPageSkPO
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

    fetchMock.get('/json/v1/clusterdiff?head=false'
        + '&include=false&neg=false&pos=false&query=name%3Dsome-test'
        + '&source_type=infra&unt=false',
        clusterDiffJSON);
    await clusterPageSkPO.searchControlsSkPO.clickIncludeDigestsNotAtHeadCheckbox();
    expectQueryStringToEqual("?corpus=infra&grouping=some-test&max_rgba=255&not_at_head=true");
  });

  it('makes an RPC for details when the selection is changed to one digest', async () => {
    fetchMock.get('/json/v1/details?corpus=infra'
      + `&digest=${positiveDigest}&test=some-test`, {
      'these-details': 'do not matter for this test',
    });

    await clusterPageSkPO.clusterDigestsSkPO.clickNode(positiveDigest);
  });

  it('makes an RPC for a diff when the selection is changed to two digests', async () => {
    fetchMock.get('/json/v1/details?corpus=infra'
        + `&digest=${positiveDigest}&test=some-test`, {
      'these-details': 'do not matter for this test',
    });

    await clusterPageSkPO.clusterDigestsSkPO.clickNode(positiveDigest);

    fetchMock.get('/json/v1/diff?corpus=infra'
        + `&left=${positiveDigest}`
        + `&right=${negativeDigest}&test=some-test`, {
      'these-details': 'do not matter for this test',
    });

    await clusterPageSkPO.clusterDigestsSkPO.shiftClickNode(negativeDigest);
  });
});
