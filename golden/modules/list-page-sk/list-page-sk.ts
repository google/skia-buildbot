/**
 * @module module/list-page-sk
 * @description <h2><code>list-page-sk</code></h2>
 *
 * This page summarizes the outputs of various tests. It shows the amount of digests produced,
 * as well as a few options to configure what range of traces to enumerate.
 *
 * It is a top level element.
 */
import { html } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { $$ } from '../../../infra-sk/modules/dom';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { stateReflector } from '../../../infra-sk/modules/stateReflector';
import { fromObject } from '../../../infra-sk/modules/query';
import { HintableObject } from '../../../infra-sk/modules/hintable';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { clusterPageHref, sendBeginTask, sendEndTask, sendFetchError } from '../common';
import { defaultCorpus } from '../settings';

import '../corpus-selector-sk';
import '../pagination-sk';
import '../query-dialog-sk';
import '../sort-toggle-sk';
import '../../../elements-sk/modules/checkbox-sk';
import '../../../elements-sk/modules/icons/group-work-icon-sk';
import '../../../elements-sk/modules/icons/tune-icon-sk';
import { SearchCriteriaToHintableObject } from '../search-controls-sk';
import { QueryDialogSk } from '../query-dialog-sk/query-dialog-sk';
import { SortToggleSk } from '../sort-toggle-sk/sort-toggle-sk';
import { SearchCriteria } from '../search-controls-sk/search-controls-sk';
import { ListTestsResponse, ParamSet, TestSummary } from '../rpc_types';
import { PaginationSkPageChangedEventDetail } from '../pagination-sk/pagination-sk';

const searchQuery = (corpus: string, query: string): string => {
  if (!query) {
    return `source_type=${corpus}`;
  }
  return `source_type=${corpus}, \n${query.split('&').join(',\n')}`;
};

/** We extend TestSummary with the test name to facilitate sorting with SortToggleSk. */
interface TestSummaryWithTestName extends TestSummary {
  name: string;
}

export class ListPageSk extends ElementSk {
  private static paginationTemplate = (el: ListPageSk, cssClass: string) => {
    return html`
      <pagination-sk
        class="${cssClass}"
        offset="${el.offset || 0}"
        page_size="${el.pageSize || 50}"
        total="${el.totalResults}"
        @page-changed=${el.onPageChange}>
      </pagination-sk>
    `;
  };

  private static template = (ele: ListPageSk) => html`
    <div>
      <corpus-selector-sk
        .corpora=${ele.corpora}
        .selectedCorpus=${ele.currentCorpus}
        @corpus-selected=${ele.currentCorpusChanged}>
      </corpus-selector-sk>

      <div class="query_params">
        <button class="show_query_dialog" @click=${ele.showQueryDialog}>
          <tune-icon-sk></tune-icon-sk>
        </button>
        <pre>${searchQuery(ele.currentCorpus, ele.currentQuery)}</pre>
        <checkbox-sk
          label="Disregard Ignore Rules"
          class="ignore_rules"
          ?checked=${ele.disregardIgnoreRules}
          @click=${ele.toggleIgnoreRules}></checkbox-sk>
      </div>
    </div>

    ${ListPageSk.paginationTemplate(ele, 'top')}
    <!-- lit-html (or maybe html in general) doesn't like sort-toggle-sk to go inside the table.-->
    <sort-toggle-sk id="sort_table" .data=${ele.byTestCounts} @sort-changed=${ele._render}>
      <table>
        <thead>
          <tr>
            <th data-key="name" data-sort-toggle-sk="up">Test name</th>
            <th data-key="positive_digests">Positive</th>
            <th data-key="negative_digests">Negative</th>
            <th data-key="untriaged_digests">Untriaged</th>
            <th data-key="total_digests">Total</th>
            <th>Cluster View</th>
          </tr>
        </thead>
        <tbody>
          <!-- repeat was tested here; map is about twice as fast as using the repeat directive
               (which moves the existing elements). This is because reusing the existing templates
               is pretty fast because there isn't a lot to change.-->
          ${ele.byTestCounts.map((row) => ListPageSk.testRow(row, ele))}
        </tbody>
      </table>
    </sort-toggle-sk>
    ${ListPageSk.paginationTemplate(ele, 'bottom')}
    <query-dialog-sk @edit=${ele.currentQueryChanged}></query-dialog-sk>
  `;

  private static testRow = (row: TestSummaryWithTestName, ele: ListPageSk) => {
    interface MakeSearchCriteriaOpts {
      positive: boolean;
      negative: boolean;
      untriaged: boolean;
    }

    // Returns a HintableObject for building the GET parameters to the search page.
    const makeSearchCriteria = (opts: MakeSearchCriteriaOpts): Partial<SearchCriteria> => ({
      corpus: ele.currentCorpus,
      leftHandTraceFilter: { name: [row.name] },
      includePositiveDigests: opts.positive,
      includeNegativeDigests: opts.negative,
      includeUntriagedDigests: opts.untriaged,
      includeIgnoredDigests: ele.disregardIgnoreRules,
      includeDigestsNotAtHead: false,
    });

    const searchPageHref = (opts: MakeSearchCriteriaOpts) => {
      const searchCriteria = makeSearchCriteria(opts);
      const queryParameters = fromObject(
        SearchCriteriaToHintableObject(searchCriteria) as HintableObject
      );
      return `/search?${queryParameters}`;
    };

    const clusterPageSearchCriteria: Partial<SearchCriteria> = {
      ...makeSearchCriteria({
        positive: true,
        negative: true,
        untriaged: true,
      }),
    };

    return html`
      <tr>
        <td>
          <a
            href="${searchPageHref({
              positive: true,
              negative: true,
              untriaged: true,
            })}"
            target="_blank"
            rel="noopener">
            ${row.name}
          </a>
        </td>
        <td class="center">
          <a
            href="${searchPageHref({
              positive: true,
              negative: false,
              untriaged: false,
            })}"
            target="_blank"
            rel="noopener">
            ${row.positive_digests}
          </a>
        </td>
        <td class="center">
          <a
            href="${searchPageHref({
              positive: false,
              negative: true,
              untriaged: false,
            })}"
            target="_blank"
            rel="noopener">
            ${row.negative_digests}
          </a>
        </td>
        <td class="center">
          <a
            href="${searchPageHref({
              positive: false,
              negative: false,
              untriaged: true,
            })}"
            target="_blank"
            rel="noopener">
            ${row.untriaged_digests}
          </a>
        </td>
        <td class="center">
          <a
            href="${searchPageHref({
              positive: true,
              negative: true,
              untriaged: true,
            })}"
            target="_blank"
            rel="noopener">
            ${row.total_digests}
          </a>
        </td>
        <td class="center">
          <a
            href="${clusterPageHref(row.grouping, clusterPageSearchCriteria)}"
            target="_blank"
            rel="noopener">
            <group-work-icon-sk></group-work-icon-sk>
          </a>
        </td>
      </tr>
    `;
  };

  private corpora: string[] = [];

  private paramset: ParamSet = {};

  private currentQuery = '';

  private currentCorpus = '';

  private disregardIgnoreRules = false;

  private pageSize = 50;

  private offset = 0;

  private totalResults = 0;

  private byTestCounts: TestSummaryWithTestName[] = [];

  private readonly stateChanged: () => void;

  // Allows us to abort fetches if we fetch again.
  private fetchController?: AbortController;

  constructor() {
    super(ListPageSk.template);

    this.stateChanged = stateReflector(
      /* getState */ () => ({
        // provide empty values
        disregard_ignores: this.disregardIgnoreRules,
        corpus: this.currentCorpus,
        query: this.currentQuery,
        pageSize: this.pageSize,
        offset: this.offset,
      }),
      /* setState */ (newState) => {
        if (!this._connected) {
          return;
        }
        // default values if not specified.
        this.disregardIgnoreRules = (newState.disregard_ignores as boolean) || false;
        this.currentCorpus = (newState.corpus as string) || defaultCorpus();
        this.currentQuery = (newState.query as string) || '';
        this.pageSize = (newState.pageSize as number) || 50;
        this.offset = (newState.offset as number) || 0;
        this.fetch();
        this._render();
      }
    );
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
  }

  private currentCorpusChanged(e: CustomEvent<string>) {
    e.stopPropagation();
    this.currentCorpus = e.detail;
    this.offset = 0;
    this.stateChanged();
    this._render();
    this.fetch();
  }

  private currentQueryChanged(e: CustomEvent<string>) {
    e.stopPropagation();
    this.currentQuery = e.detail;
    this.offset = 0;
    this.stateChanged();
    this._render();
    this.fetch();
  }

  private onPageChange(e: CustomEvent<PaginationSkPageChangedEventDetail>) {
    this.offset = Math.max(0, this.offset + e.detail.delta * this.pageSize);
    this.stateChanged!();
    this.fetch();
    this._render();
  }

  private fetch() {
    if (this.fetchController) {
      // Kill any outstanding requests
      this.fetchController.abort();
    }

    // Make a fresh abort controller for each set of fetches.
    // They cannot be re-used once aborted.
    this.fetchController = new AbortController();
    const extra = {
      signal: this.fetchController.signal,
    };

    sendBeginTask(this);
    sendBeginTask(this);

    const base = '/json/v2/list';
    let url = `${base}?corpus=${encodeURIComponent(this.currentCorpus)}`;
    if (this.disregardIgnoreRules) {
      url += '&include_ignored_traces=true';
    }
    if (this.currentQuery) {
      url += `&trace_values=${encodeURIComponent(this.currentQuery)}`;
    }

    if (this.pageSize > 0) {
      url += `&page_size=${this.pageSize}`;
    }
    if (this.offset > 0) {
      url += `&offset=${this.offset}`;
    }
    fetch(url, extra)
      .then(jsonOrThrow)
      .then((response: ListTestsResponse) => {
        this.byTestCounts = response.tests
          ? response.tests.map((test: TestSummary) => ({
              ...test,
              name: test.grouping.name,
            }))
          : [];
        this.totalResults = response.total;
        this.stateChanged();
        this._render();
        // By default, sort the data by name in ascending order (to match the direction set
        // above).
        $$<SortToggleSk<TestSummaryWithTestName>>('#sort_table', this)!.sort('name', 'up');
        sendEndTask(this);
      })
      .catch((e) => sendFetchError(this, e, 'list'));

    // TODO(kjlubick) when the search page gets a makeover to have just the params for the given
    //   corpus show up, we should do the same here. First idea is to have a separate corpora
    //   endpoint and then make paramset take a corpus.
    const paramsURL = '/json/v2/paramset';
    fetch(paramsURL, extra)
      .then(jsonOrThrow)
      .then((paramset: ParamSet) => {
        // We split the paramset into a list of corpora...
        this.corpora = paramset.source_type || [];
        // ...and the rest of the keys. This is to make it so the layout is
        // consistent with other pages (e.g. the search page, the by blame page, etc).
        delete paramset.source_type;
        this.paramset = paramset;
        this._render();
        sendEndTask(this);
      })
      .catch((e) => sendFetchError(this, e, 'paramset'));
  }

  private showQueryDialog() {
    $$<QueryDialogSk>('query-dialog-sk')!.open(this.paramset, this.currentQuery);
  }

  private toggleIgnoreRules(e: Event) {
    e.preventDefault();
    this.disregardIgnoreRules = !this.disregardIgnoreRules;
    this.stateChanged();
    this._render();
    this.fetch();
  }
}

define('list-page-sk', ListPageSk);
