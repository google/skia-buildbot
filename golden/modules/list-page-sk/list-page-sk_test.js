import './index';

import { $, $$ } from 'common-sk/modules/dom';
import { fetchMock } from 'fetch-mock';
import {
  eventPromise,
  expectQueryStringToEqual,
  setUpElementUnderTest,
} from '../../../infra-sk/modules/test_util';
import { sampleByTestList } from './test_data';
import { testOnlySetSettings } from '../settings';

describe('list-page-sk', () => {
  const newInstance = setUpElementUnderTest('list-page-sk');

  let listPageSk;

  beforeEach(async () => {
    // Clear out any query params we might have to not mess with our current state.
    setQueryString('');

    testOnlySetSettings({
      defaultCorpus: 'gm',
    });

    // These will get called on page load.
    fetchMock.get('/json/list?corpus=gm&at_head_only=true', sampleByTestList);
    // We only need a few params to make sure the edit-ignore-rule-dialog works properly and it
    // does not matter really what they are, so we use a small subset of actual params.
    const someParams = {
      alpha_type: ['Opaque', 'Premul'],
      arch: ['arm', 'arm64', 'x86', 'x86_64'],
      source_type: ['canvaskit', 'gm', 'corpus with spaces'],
    };
    fetchMock.get('/json/paramset', someParams);

    const event = eventPromise('end-task');
    listPageSk = newInstance();
    await event;
  });

  afterEach(() => {
    expect(fetchMock.done()).to.be.true; // All mock RPCs called at least once.

    // Completely remove the mocking which allows each test
    // to be able to mess with the mocked routes w/o impacting other tests.
    fetchMock.reset();
  });

  describe('html layout', () => {
    it('should make a table with 2 rows in the body', () => {
      const rows = $('table tbody tr', listPageSk);
      expect(rows).to.have.length(2);
    });

    it('should have 3 corpora loaded in, with the default selected', () => {
      const corpusSelector = $$('corpus-selector-sk', listPageSk);
      expect(corpusSelector.corpora).to.have.length(3);
      expect(corpusSelector.selectedCorpus).to.equal('gm');
    });

    it('does not have source_type (corpus) in the params', () => {
      expect(listPageSk._paramset.source_type).to.be.undefined;
    });

    it('should have links for searching and the cluster view', () => {
      const secondRow = $$('table tbody tr:nth-child(2)', listPageSk);
      const links = $('a', secondRow);
      expect(links).to.have.length(2);
      // First link should be to the search results
      const sharedParams = 'unt=true&neg=true&pos=true&source_type=gm&query=name%3Dthis_is_another_test&head=true&include=false';
      expect(links[0].href).to.contain(`/search?${sharedParams}`);
      // Second link should be to cluster view (with a very similar href)
      expect(links[1].href).to.contain(`/cluster?${sharedParams}`);
    });

    it('updates the links based on toggle positions', () => {
      listPageSk._showAllDigests = true;
      listPageSk._disregardIgnoreRules = true;
      listPageSk._render();
      const secondRow = $$('table tbody tr:nth-child(2)', listPageSk);
      const links = $('a', secondRow);
      expect(links).to.have.length(2);
      // First link should be to the search results
      const sharedParams = 'unt=true&neg=true&pos=true&source_type=gm&query=name%3Dthis_is_another_test&head=false&include=true';
      expect(links[0].href).to.contain(`/search?${sharedParams}`);
      // Second link should be to cluster view (with a very similar href)
      expect(links[1].href).to.contain(`/cluster?${sharedParams}`);
    });
  }); // end describe('html layout')

  describe('RPC calls', () => {
    it('has a checkbox to toggle use of ignore rules', async () => {
      fetchMock.get('/json/list?corpus=gm&at_head_only=true&include_ignored_traces=true', sampleByTestList);

      const checkbox = $$('checkbox-sk.ignore_rules input', listPageSk);
      const event = eventPromise('end-task');
      checkbox.click();
      await event;
      expectQueryStringToEqual('?corpus=gm&disregard_ignores=true');
    });

    it('has a checkbox to toggle measuring at head', async () => {
      fetchMock.get('/json/list?corpus=gm', sampleByTestList);

      const checkbox = $$('checkbox-sk.head_only input', listPageSk);
      const event = eventPromise('end-task');
      checkbox.click();
      await event;
      expectQueryStringToEqual('?all_digests=true&corpus=gm');
    });

    it('changes the corpus based on an event from corpus-selector-sk', async () => {
      fetchMock.get('/json/list?corpus=corpus%20with%20spaces&at_head_only=true', sampleByTestList);

      const corpusSelector = $$('corpus-selector-sk', listPageSk);
      const event = eventPromise('end-task');
      corpusSelector.dispatchEvent(
        new CustomEvent('corpus-selected', {
          detail: 'corpus with spaces',
          bubbles: true,
        }),
      );
      await event;
      expectQueryStringToEqual('?corpus=corpus%20with%20spaces');
    });

    it('changes the search params based on an event from query-dialog-sk', async () => {
      fetchMock.get(
        '/json/list?corpus=gm&at_head_only=true&trace_values=alpha_type%3DOpaque%26arch%3Darm64',
        sampleByTestList,
      );

      const queryDialog = $$('query-dialog-sk', listPageSk);
      const event = eventPromise('end-task');
      queryDialog.dispatchEvent(
        new CustomEvent('edit', {
          detail: 'alpha_type=Opaque&arch=arm64',
          bubbles: true,
        }),
      );
      await event;
      expectQueryStringToEqual('?corpus=gm&query=alpha_type%3DOpaque%26arch%3Darm64');
    });
  });
});

function setQueryString(q) {
  history.pushState(
    null, '', window.location.origin + window.location.pathname + q,
  );
}
