/**
 * @module modules/byblame-page-sk
 * @description <h2><code>byblame-page-sk</code></h2>
 *
 * Displays the current untriaged digests, grouped by blame.
 *
 * @attr default-corpus {string} Name of the corpus to use when not specified
 *     in the URL.
 * @attr base-repo-url {string} Base repository URL.
 */

import { define } from 'elements-sk/define'
import { ElementSk } from '../../../infra-sk/modules/ElementSk'
import { html } from 'lit-html'
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { stateReflector } from 'common-sk/modules/stateReflector';
import '../byblameentry-sk';
import '../corpus-selector-sk';

const template = (el) => html`
<corpus-selector-sk
    .selectedCorpus=${el._corpus}
    @corpus-selected=${(e) => el._handleCorpusChange(e)}>
</corpus-selector-sk>

<div class=entries>
  ${(!el._entries || el._entries.length === 0)
      ? (el._loaded ? 'No untriaged digests.' : 'Loading untriaged digests...')
      : el._entries.map((entry) => entryTemplate(el, entry))}
</div>
`;

const entryTemplate = (el, entry) => html`
<byblameentry-sk
    .byBlameEntry=${entry}
    .gitLog=${el._gitLogByGroupID.get(entry.groupID)}
    .corpus=${el._corpus}
    .baseRepoUrl=${el._baseRepoUrl}>
</byblameentry-sk>
`;

define('byblame-page-sk', class extends ElementSk {
  constructor() {
    super(template);

    this._corpus = '';
    // Will hold ByBlameEntry objects returned by /json/byblame for the selected
    // corpus.
    this._entries = [];
    // Maps ByBlameEntry.groupID to the corresponding gitLog object returned by
    // /json/gitlog.
    this._gitLogByGroupID = new Map();
    this._loaded = false;  // False if entries haven't been fetched yet.

    // stateReflector will trigger on DomReady.
    this._stateChanged = stateReflector(
        /* getState */ () => {
          return {
            // Provide empty values.
            'corpus': this._corpus,
          };
        },
        /* setState */ (newState) => {
          this._corpus = newState.corpus || this._defaultCorpus;
          this._render(); // Update corpus selector immediately.
          this._fetchEntries();
        });
  }

  connectedCallback() {
    super.connectedCallback();
    // Show loading indicator while we wait for results from the server.
    this._render();
  }

  get _defaultCorpus() {
    return this.getAttribute('default-corpus');
  }

  get _baseRepoUrl() {
    return this.getAttribute('base-repo-url');
  }

  _handleCorpusChange(event) {
    this._corpus = event.detail.corpus;
    this._stateChanged();
    this._fetchEntries();
  }

  _fetchEntries() {
    // Fetching is done in two steps:
    // 1. ByBlameEntry objects are fetched from /json/byblame.
    // 2. A gitLog object is retrieved from /json/gitlog for each ByBlameEntry.

    const query = encodeURIComponent(`source_type=${this._corpus}`);
    const url = `/json/byblame?query=${query}`;

    // Force only one fetch at a time. Abort any outstanding requests.
    if (this._fetchController) {
      this._fetchController.abort();
    }
    this._fetchController = new AbortController();

    // The /json/byblame and /json/gitlog fetches share the same controller.
    const options = {
      method: 'GET',
      signal: this._fetchController.signal
    };

    this._sendBusy();
    // Step 1: Fetch ByBlameEntry objects from /json/byblame.
    fetch(url, options)
        .then(jsonOrThrow)
        .then((json) => {
          this._entries = json.data || [];

          // TODO(lovisolo): Consider modifying /json/byblame to include
          //                 commit messages in its response so we don't have to
          //                 query the /json/gitlog endpoint.

          const gitLogUrl = (entry) => {
            const startHash = entry.commits[entry.commits.length - 1].hash;
            const endHash = entry.commits[0].hash;
            return `/json/gitlog?start=${startHash}&end=${endHash}`;
          };

          // Step 2: Fetch gitLog objects from /json/gitlog.
          return Promise.all(
              this._entries.map(
                  (entry) =>
                      fetch(gitLogUrl(entry), options)
                          .then(jsonOrThrow)
                          .then(
                              (gitLog) =>
                                  this._gitLogByGroupID.set(
                                      entry.groupID,
                                      gitLog))));
        })
        .then(() => {
          this._loaded = true;
          this._render();
          this._sendDone();
        })
        .catch((e) => this._sendError(e));
  }

  _sendBusy() {
    this.dispatchEvent(new CustomEvent('begin-task', {bubbles: true}));
  }

  _sendDone() {
    this.dispatchEvent(new CustomEvent('end-task', {bubbles: true}));
  }

  _sendError(e) {
    this.dispatchEvent(new CustomEvent('fetch-error', {
      detail: {
        error: e,
        loading: 'by blame page',
      }, bubbles: true
    }));
  }
});
