/**
 * @module modules/triagelog-page-sk
 * @description <h2><code>triagelog-page-sk</code></h2>
 *
 * Allows the user to page through the diff triage logs, and optionally undo
 * labels applied to triaged diffs.
 */

import { define } from 'elements-sk/define'
import 'elements-sk/checkbox-sk'
import { ElementSk } from '../../../infra-sk/modules/ElementSk'
import { html } from 'lit-html'
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { stateReflector } from 'common-sk/modules/stateReflector';
import '../pagination-sk'

const template = (el) => html`
<div class=controls>
  <pagination-sk offset=${el._pageOffset}
                 page_size=${el._pageSize}
                 total=${el._totalEntries}
                 @page-changed=${el._pageChanged}>
  </pagination-sk>

  <checkbox-sk ?checked=${el._details}
               @change=${el._detailsHandler}
               label="Show details"
               class=details-checkbox>
  </checkbox-sk>
</div>

<table>
  <thead>
    <tr>
      <th>Timestamp</th>
      <th>Name</th>
      <th># Changes</th>
      <th>Actions</th>
    </tr>
  </thead>
  <tbody>
    ${el._entries.map((entry) => logEntryTemplate(el, entry))}
  </tbody>
</table>
`;

const logEntryTemplate = (el, entry) => html`
<tr>
  <td class=timestamp>${el._toLocalDate(entry.ts)}</td>
  <td class=author>${entry.name}</td>
  <td class=num-changes>${entry.changeCount}</td>
  <td class=actions>
    <button @click=${() => el._undoEntry(entry.id)}
            class=undo>
      Undo
    </button>
  </td>
</tr>

${entry.details ? detailsTemplate(entry) : html``}
`;

const detailsTemplate = (entry) => html`
<tr class=details>
  <td></td>
  <td><strong>Test name</strong></td>
  <td><strong>Digest</strong></td>
  <td><strong>Label</strong></td>
</tr>

${entry.details.map(detailsEntryTemplate)}

<tr class="details details-separator"><td colspan="4"></td></tr>
`;

const detailsEntryTemplate = (detailsEntry) => html`
<tr class=details>
  <td></td>
  <td class=test-name>${detailsEntry.test_name}</td>
  <td class=digest>
    <a href="/detail?test=${detailsEntry.test_name}&digest=${detailsEntry.digest}"
       target="_blank"
       rel="noopener">
      ${detailsEntry.digest}
    </a>
  </td>
  <td class=label>${detailsEntry.label}</td>
</tr>
`;

define('triagelog-page-sk', class extends ElementSk {
  constructor() {
    super(template);

    this._entries = [];      // Log entries fetched from the server.
    this._details = false;   // Reflected in the URL.
    this._pageOffset = 0;    // Reflected in the URL.
    this._pageSize = 0;      // Reflected in the URL.
    this._totalEntries = 0;  // Total number of entries in the server.
    this._urlParamsLoaded = false;

    // stateReflector will trigger on DomReady.
    this._stateChanged = stateReflector(
        /* getState */ () => this._getState(),
        /* setState */ (newState) => {
          this._pageOffset = newState.offset || 0;
          this._pageSize = newState.page_size || 20;
          this._details = newState.details || false;
          this._render();
          this._fetchEntries();
        });
  }

  _getState() {
    return {
      'offset': this._pageOffset,
      'page_size': this._pageSize,
      'details': this._details,
    };
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  _detailsHandler(e) {
    this._details = e.target.checked;
    this._stateChanged();
    this._render();
    this._fetchEntries();
  }

  _pageChanged(e) {
    this._pageOffset =
        Math.max(0, this._pageOffset + e.detail.delta * this._pageSize);
    this._stateChanged();
    this._render();
    this._fetchEntries();
  }

  _undoEntry(entryId) {
    const oldState = this._getState();
    this._sendBusy();
    this._fetch(`/json/triagelog/undo?id=${entryId}`, 'POST')
        .then(() => {
          // The undo RPC endpoint returns the first page of results with
          // details hidden, so we need to uncheck the "Show details"
          // checkbox before re-rendering.
          // TODO(lovisolo): Rethink this behavior once we delete the old
          //                 triagelog page. This will likely require making
          //                 changes to the RPC endpoint.
          this._details = false;
          if (JSON.stringify(oldState) !== JSON.stringify(this._getState())) {
            this._stateChanged();
          }
          this._render();
          this._sendDone();
        })
        .catch((e) => this._sendFetchError(e));
  }

  _fetchEntries() {
    const url =
        `/json/triagelog?details=${this._details}` +
        `&offset=${this._pageOffset}&size=${this._pageSize}`;
    this._sendBusy();
    this._fetch(url, 'GET')
        .then(() => {
          this._render();
          this._sendDone();
        })
        .catch((e) => this._sendFetchError(e));
  }

  // Both /json/triagelog and /json/triagelog/undo RPCs return the same kind of
  // response, which is a page with triage log entries. Therefore this method is
  // called by both _fetchEntries and _undoEntry to carry out their
  // corresponding RPCs and handle the server response.
  _fetch(url, method) {
    // Force only one fetch at a time. Abort any outstanding requests.
    if (this._fetchController) {
      this._fetchController.abort();
    }
    this._fetchController = new AbortController();

    const options = {
      method: method,
      signal: this._fetchController.signal
    };

    return fetch(url, options)
        .then(jsonOrThrow)
        .then((json) => {
          this._entries = json.data || [];
          this._pageOffset = json.pagination.offset || 0;
          this._pageSize = json.pagination.size || 0;
          this._totalEntries = json.pagination.total || 0;
        });
  }

  _toLocalDate(timeStampMS) {
    return new Date(timeStampMS).toLocaleString();
  }

  _sendBusy() {
    this.dispatchEvent(new CustomEvent('begin-task', {bubbles: true}));
  }

  _sendDone() {
    this.dispatchEvent(new CustomEvent('end-task', {bubbles: true}));
  }

  _sendFetchError(error) {
    this.dispatchEvent(new CustomEvent('fetch-error', {
      detail: {
        error: error,
        loading: 'triagelog',
      }, bubbles: true
    }));
  }
});
