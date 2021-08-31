import './index';

import { $, $$ } from 'common-sk/modules/dom';
import fetchMock from 'fetch-mock';
import { expect } from 'chai';
import {
  eventPromise,
  expectQueryStringToEqual,
  setQueryString,
  setUpElementUnderTest,
} from '../../../infra-sk/modules/test_util';
import { sampleByTestList } from './test_data';
import { testOnlySetSettings } from '../settings';
import { ListPageSk } from './list-page-sk';
import { CorpusSelectorSk } from '../corpus-selector-sk/corpus-selector-sk';
import { QueryDialogSk } from '../query-dialog-sk/query-dialog-sk';
import { QueryDialogSkPO } from '../query-dialog-sk/query-dialog-sk_po';
import { CorpusSelectorSkPO } from '../corpus-selector-sk/corpus-selector-sk_po';

describe('list-page-sk', () => {
  const newInstance = setUpElementUnderTest<ListPageSk>('list-page-sk');

  let listPageSk: ListPageSk;
  let queryDialogSkPO: QueryDialogSkPO;
  let corpusSelectorSkPO: CorpusSelectorSkPO;

  beforeEach(async () => {
    // Clear out any query params we might have to not mess with our current state.
    setQueryString('');

    testOnlySetSettings({
      defaultCorpus: 'gm',
    });

    // These will get called on page load.
    fetchMock.get('/json/v2/list?corpus=gm', sampleByTestList);
    // We only need a few params to make sure the edit-ignore-rule-dialog works properly and it
    // does not matter really what they are, so we use a small subset of actual params.
    const someParams = {
      alpha_type: ['Opaque', 'Premul'],
      arch: ['arm', 'arm64', 'x86', 'x86_64'],
      source_type: ['canvaskit', 'gm', 'corpus with spaces'],
    };
    fetchMock.get('/json/v2/paramset', someParams);

    const event = eventPromise('end-task');
    listPageSk = newInstance();
    await event;

    queryDialogSkPO = new QueryDialogSkPO(listPageSk.querySelector<QueryDialogSk>('query-dialog-sk')!);
    corpusSelectorSkPO = new CorpusSelectorSkPO(
            listPageSk.querySelector<CorpusSelectorSk<string>>('corpus-selector-sk')!,
    );
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

    it('should have 3 corpora loaded in, with the default selected', async () => {
      expect(await corpusSelectorSkPO.getCorpora()).to.have.length(3);
      expect(await corpusSelectorSkPO.getSelectedCorpus()).to.equal('gm');
    });

    it('does not have source_type (corpus) in the params', () => {
      // Field "paramset" is private, thus the cast to any. Is this test really necessary?
      expect((listPageSk as any).paramset.source_type).to.be.undefined;
    });

    const expectedSearchPageHref = (opts: {
          positive: boolean,
          negative: boolean,
          untriaged: boolean,
          disregardIgnoreRules: boolean
        }): string => `/search?${[
      'corpus=gm',
      `include_ignored=${opts.disregardIgnoreRules}`,
      'left_filter=name%3Dthis_is_another_test',
      'max_rgba=0',
      'min_rgba=0',
      `negative=${opts.negative}`,
      'not_at_head=false',
      `positive=${opts.positive}`,
      'reference_image_required=false',
      'right_filter=',
      'sort=descending',
      `untriaged=${opts.untriaged}`,
    ].join('&')}`;

    const expectedClusterPageHref = (opts: {disregardIgnoreRules: boolean}): string => `/cluster?${[
      'corpus=gm',
      'grouping=this_is_another_test',
      `include_ignored=${opts.disregardIgnoreRules}`,
      'left_filter=',
      'max_rgba=0',
      'min_rgba=0',
      'negative=true',
      'not_at_head=false',
      'positive=true',
      'reference_image_required=false',
      'right_filter=',
      'sort=descending',
      'untriaged=true',
    ].join('&')}`;

    it('should have links for searching and the cluster view', () => {
      const secondRow = $$<HTMLTableRowElement>('table tbody tr:nth-child(2)', listPageSk)!;
      const links = $<HTMLAnchorElement>('a', secondRow)!;
      expect(links).to.have.length(6);

      // First link should be to the search results for all digests.
      expect(links[0].getAttribute('href')).to.equal(expectedSearchPageHref({
        positive: true,
        negative: true,
        untriaged: true,
        disregardIgnoreRules: false,
      }));

      // Second link should be just positive digests.
      expect(links[1].getAttribute('href')).to.equal(expectedSearchPageHref({
        positive: true,
        negative: false,
        untriaged: false,
        disregardIgnoreRules: false,
      }));

      // Third link should be just negative digests.
      expect(links[2].getAttribute('href')).to.equal(expectedSearchPageHref({
        positive: false,
        negative: true,
        untriaged: false,
        disregardIgnoreRules: false,
      }));

      // Fourth link should be just untriaged digests.
      expect(links[3].getAttribute('href')).to.equal(expectedSearchPageHref({
        positive: false,
        negative: false,
        untriaged: true,
        disregardIgnoreRules: false,
      }));

      // Fifth link is the total count, and should be the same as the first link.
      expect(links[4].getAttribute('href')).to.equal(expectedSearchPageHref({
        positive: true,
        negative: true,
        untriaged: true,
        disregardIgnoreRules: false,
      }));

      // Sixth link should be to cluster view
      expect(links[5].getAttribute('href')).to.equal(
        expectedClusterPageHref({ disregardIgnoreRules: false }),
      );
    });

    it('updates the links based on toggle positions', async () => {
      fetchMock.get('/json/v2/list?corpus=gm&include_ignored_traces=true', sampleByTestList);

      await clickDisregardIgnoreRulesCheckbox(listPageSk);

      const secondRow = $$<HTMLTableRowElement>('table tbody tr:nth-child(2)', listPageSk)!;
      const links = $('a', secondRow);
      expect(links).to.have.length(6);

      // First link should be to the search results for all digests.
      expect(links[0].getAttribute('href')).to.equal(expectedSearchPageHref({
        positive: true,
        negative: true,
        untriaged: true,
        disregardIgnoreRules: true,
      }));

      // Second link should be just positive digests.
      expect(links[1].getAttribute('href')).to.equal(expectedSearchPageHref({
        positive: true,
        negative: false,
        untriaged: false,
        disregardIgnoreRules: true,
      }));

      // Third link should be just negative digests.
      expect(links[2].getAttribute('href')).to.equal(expectedSearchPageHref({
        positive: false,
        negative: true,
        untriaged: false,
        disregardIgnoreRules: true,
      }));

      // Fourth link should be just untriaged digests.
      expect(links[3].getAttribute('href')).to.equal(expectedSearchPageHref({
        positive: false,
        negative: false,
        untriaged: true,
        disregardIgnoreRules: true,
      }));

      // Fifth link is the total count, and should be the same as the first link.
      expect(links[4].getAttribute('href')).to.equal(expectedSearchPageHref({
        positive: true,
        negative: true,
        untriaged: true,
        disregardIgnoreRules: true,
      }));

      // Sixth link should be to cluster view
      expect(links[5].getAttribute('href')).to.equal(
        expectedClusterPageHref({ disregardIgnoreRules: true }),
      );
    });

    it('updates the sort order by clicking on sort-toggle-sk', async () => {
      let firstRow = $$<HTMLTableRowElement>('table tbody tr:nth-child(1)', listPageSk)!;
      expect($$<HTMLTableDataCellElement>('td', firstRow)!.innerText).to.equal('this_is_a_test');

      // After first click, it will be sorting in descending order by number of negatives.
      clickOnNegativeHeader(listPageSk);

      firstRow = $$<HTMLTableRowElement>('table tbody tr:nth-child(1)', listPageSk)!;
      expect($$<HTMLTableDataCellElement>('td', firstRow)!.innerText)
        .to.equal('this_is_another_test');

      // After second click, it will be sorting in ascending order by number of negatives.
      clickOnNegativeHeader(listPageSk);

      firstRow = $$<HTMLTableRowElement>('table tbody tr:nth-child(1)', listPageSk)!;
      expect($$<HTMLTableDataCellElement>('td', firstRow)!.innerText).to.equal('this_is_a_test');
    });
  }); // end describe('html layout')

  describe('RPC calls', () => {
    it('has a checkbox to toggle use of ignore rules', async () => {
      fetchMock.get(
        '/json/v2/list?corpus=gm&include_ignored_traces=true',
        sampleByTestList,
      );

      await clickDisregardIgnoreRulesCheckbox(listPageSk);
      expectQueryStringToEqual('?corpus=gm&disregard_ignores=true');
    });

    it('changes the corpus based on an event from corpus-selector-sk', async () => {
      fetchMock.get(
        '/json/v2/list?corpus=corpus%20with%20spaces', sampleByTestList,
      );

      const event = eventPromise('end-task');
      await corpusSelectorSkPO.clickCorpus('corpus with spaces');
      await event;

      expectQueryStringToEqual('?corpus=corpus%20with%20spaces');
    });

    it('changes the search params based on an event from query-dialog-sk', async () => {
      fetchMock.get(
        '/json/v2/list?'
          + 'corpus=gm&trace_values=alpha_type%3DOpaque%26arch%3Darm64',
        sampleByTestList,
      );

      const event = eventPromise('end-task');
      $$<HTMLButtonElement>('.show_query_dialog', listPageSk)!.click();
      await queryDialogSkPO.setSelection({ alpha_type: ['Opaque'], arch: ['arm64'] });
      await queryDialogSkPO.clickShowMatchesBtn();
      await event;

      expectQueryStringToEqual('?corpus=gm&query=alpha_type%3DOpaque%26arch%3Darm64');
    });
  });
});

function clickOnNegativeHeader(ele: ListPageSk) {
  $$<HTMLTableHeaderCellElement>('table > thead > tr > th:nth-child(3)', ele)!.click();
}

async function clickDisregardIgnoreRulesCheckbox(listPageSk: ListPageSk) {
  const checkbox = $$<HTMLInputElement>('checkbox-sk.ignore_rules input', listPageSk)!;
  const event = eventPromise('end-task');
  checkbox.click();
  await event;
}
