/**
 * @module module/list-page-sk
 * @description <h2><code>list-page-sk</code></h2>
 *
 * This page summarizes the outputs of various tests. It shows the amount of digests produced,
 * as well as a few options to configure what range of traces to enumerate.
 *
 * It is a top level element.
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { $$ } from 'common-sk/modules/dom';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { stateReflector } from 'common-sk/modules/stateReflector';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { sendBeginTask, sendEndTask, sendFetchError } from '../common';
import { defaultCorpus } from '../settings';

import '../corpus-selector-sk';
import '../query-dialog-sk';
import '../sort-toggle-sk';
import 'elements-sk/checkbox-sk';
import 'elements-sk/icon/group-work-icon-sk';
import 'elements-sk/icon/tune-icon-sk';
import { SearchCriteriaToHintableObject } from '../search-controls-sk';
import { fromObject } from 'common-sk/modules/query';

const template = (ele) => html`
<div>
  <corpus-selector-sk .corpora=${ele._corpora}
      .selectedCorpus=${ele._currentCorpus} @corpus-selected=${ele._currentCorpusChanged}>
  </corpus-selector-sk>

  <div class=query_params>
    <button class=show_query_dialog @click=${ele._showQueryDialog}>
      <tune-icon-sk></tune-icon-sk>
    </button>
    <pre>${searchQuery(ele._currentCorpus, ele._currentQuery)}</pre>
    <checkbox-sk label="Digests at Head Only" class=head_only
             ?checked=${!ele._showAllDigests} @click=${ele._toggleAllDigests}></checkbox-sk>
    <checkbox-sk label="Disregard Ignore Rules" class=ignore_rules
             ?checked=${ele._disregardIgnoreRules} @click=${ele._toggleIgnoreRules}></checkbox-sk>
  </div>
</div>

<!-- lit-html (or maybe html in general) doesn't like sort-toggle-sk to go inside the table.-->
<sort-toggle-sk id=sort_table .data=${ele._byTestCounts} @sort-changed=${ele._render}>
  <table>
     <thead>
         <tr>
          <th data-key=name data-sort-toggle-sk=up>Test name</th>
          <th data-key=positive_digests>Positive</th>
          <th data-key=negative_digests>Negative</th>
          <th data-key=untriaged_digests>Untriaged</th>
          <th data-key=total_digests>Total</th>
          <th>Cluster View</th>
        </tr>
    </thead>
    <tbody>
      <!-- repeat was tested here; map is about twice as fast as using the repeat directive
           (which moves the existing elements). This is because reusing the existing templates
           is pretty fast because there isn't a lot to change.-->
      ${ele._byTestCounts.map((row) => testRow(row, ele))}
    </tbody>
  </table>
</sort-toggle-sk>

<query-dialog-sk @edit=${ele._currentQueryChanged}></query-dialog-sk>
`;

const testRow = (row, ele) => {
  // TODO(kjlubick) update this to use helpers in search-controls-sk.
  const allDigests = 'unt=true&neg=true&pos=true';
  const query = encodeURIComponent(`name=${row.name}&source_type=${ele._currentCorpus}`);
  const searchParams = `query=${query}`
    + `&head=${ele._showAllDigests ? 'false' : 'true'}`
    + `&include=${ele._disregardIgnoreRules ? 'true' : 'false'}`;

  const searchCriteria = {
    corpus: ele._currentCorpus,
    includePositiveDigests: true,
    includeNegativeDigests: true,
    includeUntriagedDigests: true,
    includeDigestsNotAtHead: ele._showAllDigests,
    includeIgnoredDigests: ele._disregardIgnoreRules,
  };
  const clusterState = SearchCriteriaToHintableObject(searchCriteria);
  clusterState.grouping = row.name;

  return html`
<tr>
  <td>
    <a href="/search?${searchParams}&${allDigests}" target=_blank rel=noopener>
      ${row.name}
    </a>
  </td>
  <td class=center>
    <a href="/search?${searchParams}&pos=true&neg=false&unt=false" target=_blank rel=noopener>
     ${row.positive_digests}
    </a>
  </td>
  <td class=center>
    <a href="/search?${searchParams}&pos=false&neg=true&unt=false" target=_blank rel=noopener>
     ${row.negative_digests}
    </a>
  </td>
  <td class=center>
    <a href="/search?${searchParams}&pos=false&neg=false&unt=true" target=_blank rel=noopener>
     ${row.untriaged_digests}
    </a>
  </td>
  <td class=center>
    <a href="/search?${searchParams}&${allDigests}" target=_blank rel=noopener>
     ${row.total_digests}
    </a>
  </td>
  <td class=center>
    <a href="/cluster?${fromObject(clusterState)}" target=_blank rel=noopener>
      <group-work-icon-sk></group-work-icon-sk>
    </a>
  </td>
</tr>`;
};

const searchQuery = (corpus, query) => {
  if (!query) {
    return `source_type=${corpus}`;
  }
  return `source_type=${corpus}, \n${query.split('&').join(',\n')}`;
};

define('list-page-sk', class extends ElementSk {
  constructor() {
    super(template);

    this._corpora = [];
    this._paramset = {};

    this._currentQuery = '';
    this._currentCorpus = '';

    this._showAllDigests = false;
    this._disregardIgnoreRules = false;

    this._stateChanged = stateReflector(
      /* getState */() => ({
        // provide empty values
        all_digests: this._showAllDigests,
        disregard_ignores: this._disregardIgnoreRules,
        corpus: this._currentCorpus,
        query: this._currentQuery,
      }), /* setState */(newState) => {
        if (!this._connected) {
          return;
        }
        // default values if not specified.
        this._showAllDigests = newState.all_digests || false;
        this._disregardIgnoreRules = newState.disregard_ignores || false;
        this._currentCorpus = newState.corpus || defaultCorpus();
        this._currentQuery = newState.query || '';
        this._fetch();
        this._render();
      },
    );

    this._byTestCounts = [];

    // Allows us to abort fetches if we fetch again.
    this._fetchController = null;
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  _currentCorpusChanged(e) {
    e.stopPropagation();
    this._currentCorpus = e.detail;
    this._stateChanged();
    this._render();
    this._fetch();
  }

  _currentQueryChanged(e) {
    e.stopPropagation();
    this._currentQuery = e.detail;
    this._stateChanged();
    this._render();
    this._fetch();
  }

  _fetch() {
    if (this._fetchController) {
      // Kill any outstanding requests
      this._fetchController.abort();
    }

    // Make a fresh abort controller for each set of fetches.
    // They cannot be re-used once aborted.
    this._fetchController = new AbortController();
    const extra = {
      signal: this._fetchController.signal,
    };

    sendBeginTask(this);
    sendBeginTask(this);

    let url = `/json/v1/list?corpus=${encodeURIComponent(this._currentCorpus)}`;
    if (!this._showAllDigests) {
      url += '&at_head_only=true';
    }
    if (this._disregardIgnoreRules) {
      url += '&include_ignored_traces=true';
    }
    if (this._currentQuery) {
      url += `&trace_values=${encodeURIComponent(this._currentQuery)}`;
    }
    fetch(url, extra)
      .then(jsonOrThrow)
      .then((jsonList) => {
        this._byTestCounts = jsonList;
        this._byTestCounts.forEach((row) => {
          row.total_digests = row.positive_digests + row.negative_digests + row.untriaged_digests;
        });
        this._render();
        // By default, sort the data by name in ascending order (to match the direction set above).
        $$('#sort_table', this).sort('name', 'up');
        sendEndTask(this);
      })
      .catch((e) => sendFetchError(this, e, 'list'));

    // TODO(kjlubick) when the search page gets a makeover to have just the params for the given
    //   corpus show up, we should do the same here. First idea is to have a separate corpora
    //   endpoint and then make paramset take a corpus.
    fetch('/json/v1/paramset', extra)
      .then(jsonOrThrow)
      .then((paramset) => {
        // We split the paramset into a list of corpora...
        this._corpora = paramset.source_type || [];
        // ...and the rest of the keys. This is to make it so the layout is
        // consistent with other pages (e.g. the search page, the by blame page, etc).
        delete paramset.source_type;
        this._paramset = paramset;
        this._render();
        sendEndTask(this);
      })
      .catch((e) => sendFetchError(this, e, 'paramset'));
  }

  _showQueryDialog() {
    $$('query-dialog-sk').open(this._paramset, this._currentQuery);
  }

  _toggleAllDigests(e) {
    e.preventDefault();
    this._showAllDigests = !this._showAllDigests;
    this._stateChanged();
    this._render();
    this._fetch();
  }

  _toggleIgnoreRules(e) {
    e.preventDefault();
    this._disregardIgnoreRules = !this._disregardIgnoreRules;
    this._stateChanged();
    this._render();
    this._fetch();
  }
});
