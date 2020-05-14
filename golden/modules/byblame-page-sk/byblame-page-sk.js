/**
 * @module modules/byblame-page-sk
 * @description <h2><code>byblame-page-sk</code></h2>
 *
 * Displays the current untriaged digests, grouped by the commits that may have caused them
 * (i.e. the blamelist or blame, for short).
 *
 */

import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { stateReflector } from 'common-sk/modules/stateReflector';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import '../byblameentry-sk';
import '../corpus-selector-sk';
import { sendBeginTask, sendEndTask, sendFetchError } from '../common';
import { defaultCorpus } from '../settings';

const template = (ele) => html`
<div class=top-container>
  <corpus-selector-sk
      .corpora=${ele._corpora}
      .selectedCorpus=${ele._corpus}
      .corpusRendererFn=${corpusRendererFn}
      @corpus-selected=${ele._handleCorpusChange}>
  </corpus-selector-sk>
</div>

<div class=entries>
  ${(!ele._entries || ele._entries.length === 0)
    ? noEntries(ele) : ele._entries.map((entry) => entryTemplate(ele, entry))}
</div>
`;

const entryTemplate = (ele, entry) => html`
<byblameentry-sk
    .byBlameEntry=${entry}
    .corpus=${ele._corpus}>
</byblameentry-sk>
`;

const noEntries = (ele) => {
  if (!ele._loaded) {
    return 'Loading untriaged digests...';
  }
  return `No untriaged digests for corpus ${ele._corpus}.`;
};

const corpusRendererFn = (corpus) => {
  if (corpus.untriagedCount) {
    return `${corpus.name} (${corpus.untriagedCount})`;
  }
  return corpus.name;
};

define('byblame-page-sk', class extends ElementSk {
  constructor() {
    super(template);

    this._corpora = [];
    this._corpus = '';
    // Will hold ByBlameEntry objects returned by /json/byblame for the selected
    // corpus.
    this._entries = [];
    this._loaded = false; // False if entries haven't been fetched yet.

    // stateReflector will trigger on DomReady.
    this._stateChanged = stateReflector(
      /* getState */ () => ({
        // Provide empty values.
        corpus: this._corpus,
      }),
      /* setState */ (newState) => {
        // The stateReflector's lingering popstate event handler will continue
        // to call this function on e.g. browser back button clicks long after
        // this custom element is detached from the DOM.
        if (!this._connected) {
          return;
        }

        this._corpus = newState.corpus || defaultCorpus();
        this._render(); // Update corpus selector immediately.
        this._fetch();
      },
    );
  }

  connectedCallback() {
    super.connectedCallback();
    // Show loading indicator while we wait for results from the server.
    this._render();
  }

  _handleCorpusChange(event) {
    this._corpus = event.detail.corpus;
    this._stateChanged();
    this._fetch();
  }

  _fetch() {
    // Force only one fetch at a time. Abort any outstanding requests.
    if (this._fetchController) {
      this._fetchController.abort();
    }
    this._fetchController = new AbortController();

    const options = {
      method: 'GET',
      signal: this._fetchController.signal,
    };

    const query = encodeURIComponent(`source_type=${this._corpus}`);
    const byBlameURL = `/json/byblame?query=${query}`;

    sendBeginTask(this);
    fetch(byBlameURL, options)
      .then(jsonOrThrow)
      .then((json) => {
        this._entries = json.data || [];
        this._loaded = true;
        this._render();
        sendEndTask(this);
      })
      .catch((e) => sendFetchError(this, e, 'byblamepage_entries'));

    sendBeginTask(this);
    fetch('/json/trstatus', options)
      .then(jsonOrThrow)
      .then((json) => {
        this._corpora = json.corpStatus;
        this._render();
        sendEndTask(this);
      })
      .catch((e) => sendFetchError(this, e, 'byblamepage_trstatus'));
  }
});
