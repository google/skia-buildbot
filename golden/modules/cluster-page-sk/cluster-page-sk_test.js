import './index';

import { $, $$ } from 'common-sk/modules/dom';
import { fetchMock } from 'fetch-mock';

import {
  eventPromise,
  setQueryString,
  expectQueryStringToEqual,
  setUpElementUnderTest,
} from '../../../infra-sk/modules/test_util';
import { clusterDiffJSON } from './test_data';
import { testOnlySetSettings } from '../settings';

describe('cluster-page-sk', () => {
  const newInstance = setUpElementUnderTest('cluster-page-sk');

  // Instantiate page; wait for RPCs to complete and for the page to render.
  const loadClusterPageSk = async () => {
    const endTask = eventPromise('end-task');
    const instance = newInstance();
    await endTask;
    return instance;
  };

  beforeEach(async () => {
    testOnlySetSettings({
      defaultCorpus: 'infra',
    });
    // Clear out any query params we might have to not mess with our current state.
    // This page always requires a grouping to be set.
    setQueryString('?grouping=some-test');

    // These are the default RPC calls when the page loads.
    fetchMock.get('/json/v1/clusterdiff?head=true'
      + '&include=false&neg=false&pos=false&query=name%3Dsome-test'
      + '&source_type=infra&unt=false', clusterDiffJSON);
    fetchMock.get('/json/v1/paramset', clusterDiffJSON.paramsetsUnion);
  });

  afterEach(() => {
    // Completely remove the mocking which allows each test
    // to be able to mess with the mocked routes w/o impacting other tests.
    fetchMock.reset();
  });

  describe('RPC calls', () => {
    let clusterPageSk;
    beforeEach(async () => {
      clusterPageSk = await loadClusterPageSk();
      // Wait for initial page load to finish.
      await fetchMock.flush(true);
    });

    afterEach(async () => {
      // Make sure all subsequent RPC calls happen.
      await fetchMock.flush(true);
    });

    it('removes corpus and test name from the paramset', () => {
      expect(clusterPageSk._paramset).to.deep.equal({
        ext: ['png'],
        gpu: ['AMD', 'nVidia'],
      });
      expect(clusterPageSk._grouping).to.equal('some-test');
    });

    it('changes what it fetches based on search controls', () => {
      fetchMock.get('/json/v1/clusterdiff?head=false&include=true&neg=true&pos=true&'
        + 'query=gpu%3DAMD%26name%3Dsome-test&source_type=some-other-corpus&unt=true',
      clusterDiffJSON);

      // TODO(kjlubick, lovisolo) use the search-controls-sk PO when that lands.
      clusterPageSk._searchControlsChanged(new CustomEvent('search-controls-sk-change', {
        detail: {
          corpus: 'some-other-corpus',
          leftHandTraceFilter: { gpu: ['AMD'] },
          includePositiveDigests: true,
          includeNegativeDigests: true,
          includeUntriagedDigests: true,
          includeDigestsNotAtHead: true,
          includeIgnoredDigests: true,
        },
      }));

      expectQueryStringToEqual(
        '?corpus=some-other-corpus&grouping=some-test&include_ignored=true'
        + '&left_filter=gpu%3DAMD&negative=true&not_at_head=true&positive=true&untriaged=true',
      );
    });

    it('makes an RPC for details when the selection is changed to one digest', () => {
      fetchMock.get('/json/v1/details?corpus=infra'
        + '&digest=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa&test=some-test', {
        'these-details': 'do not matter for this test',
      });

      // In headless tests, the svg doesn't seem to be rendered, so we can't easily click or
      // trigger organic selection-changed events. Thus, we trigger them artificially.
      clusterPageSk._selectionChanged(new CustomEvent('selection-changed', {
        detail: ['aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'], // valid, arbitrary digest
      }));
    });

    it('makes an RPC for a diff when the selection is changed to two digests', () => {
      fetchMock.get('/json/v1/diff?corpus=infra'
        + '&left=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb'
        + '&right=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa&test=some-test', {
        'these-details': 'do not matter for this test',
      });

      // In headless tests, the svg doesn't seem to be rendered, so we can't easily click or
      // trigger organic selection-changed events. Thus, we trigger them artificially.
      clusterPageSk._selectionChanged(new CustomEvent('selection-changed', {
        detail: ['bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb', 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'],
      }));
    });
  });
});
