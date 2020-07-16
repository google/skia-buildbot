import './index';

import { $, $$ } from 'common-sk/modules/dom';
import { fetchMock } from 'fetch-mock';
import {
  eventPromise,
  expectQueryStringToEqual,
  setQueryString,
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
      expect(links).to.have.length(6);
      // First link should be to the search results for all digests.
      const paramsForAllDigests = 'query=name%3Dthis_is_another_test%26source_type%3Dgm&head=true&include=false&unt=true&neg=true&pos=true';
      expect(links[0].href).to.contain(`/search?${paramsForAllDigests}`);
      // Second through Fourth links are for just positive, negative, untriaged
      expect(links[1].href).to.contain('pos=true&neg=false&unt=false');
      expect(links[2].href).to.contain('pos=false&neg=true&unt=false');
      expect(links[3].href).to.contain('pos=false&neg=false&unt=true');
      // Fifth link is the total count, which is the same as the first link.
      expect(links[4].href).to.contain(`/search?${paramsForAllDigests}`);
      // Sixth link should be to cluster view (with a very similar href)
      expect(links[5].href).to.contain(`/cluster?${paramsForAllDigests}`);
    });

    it('updates the links based on toggle positions', () => {
      listPageSk._showAllDigests = true;
      listPageSk._disregardIgnoreRules = true;
      listPageSk._render();
      const secondRow = $$('table tbody tr:nth-child(2)', listPageSk);
      const links = $('a', secondRow);
      expect(links).to.have.length(6);
      // First link should be to the search results
      const paramsForAllDigests = 'query=name%3Dthis_is_another_test%26source_type%3Dgm&head=false&include=true&unt=true&neg=true&pos=true';
      expect(links[0].href).to.contain(`/search?${paramsForAllDigests}`);
      // Second through Fourth links are for just positive, negative, untriaged
      expect(links[1].href).to.contain('pos=true&neg=false&unt=false');
      expect(links[2].href).to.contain('pos=false&neg=true&unt=false');
      expect(links[3].href).to.contain('pos=false&neg=false&unt=true');
      // Fifth link is the total count, which is the same as the first link.
      expect(links[4].href).to.contain(`/search?${paramsForAllDigests}`);
      // Sixth link should be to cluster view (with a very similar href)
      expect(links[5].href).to.contain(`/cluster?${paramsForAllDigests}`);
    });

    it('updates the sort order by clicking on sort-toggle-sk', async () => {
      let firstRow = $$('table tbody tr:nth-child(1)', listPageSk);
      expect($$('td', firstRow).innerText).to.equal('this_is_a_test');

      // After first click, it will be sorting in descending order by number of negatives.
      clickOnNegativeHeader(listPageSk);

      firstRow = $$('table tbody tr:nth-child(1)', listPageSk);
      expect($$('td', firstRow).innerText).to.equal('this_is_another_test');

      // After second click, it will be sorting in ascending order by number of negatives.
      clickOnNegativeHeader(listPageSk);

      firstRow = $$('table tbody tr:nth-child(1)', listPageSk);
      expect($$('td', firstRow).innerText).to.equal('this_is_a_test');
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

function clickOnNegativeHeader(ele) {
  $$('table > thead > tr > th:nth-child(3)', ele).click();
}
