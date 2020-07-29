/**
 * @module modules/triagelog-page-sk
 * @description <h2><code>triagelog-page-sk</code></h2>
 *
 * Allows the user to page through the diff triage logs, and optionally undo
 * labels applied to triaged diffs.
 */

import { define } from 'elements-sk/define';
import 'elements-sk/checkbox-sk';
import { html } from 'lit-html';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { stateReflector } from 'common-sk/modules/stateReflector';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import '../pagination-sk';
import { sendBeginTask, sendEndTask, sendFetchError } from '../common';

const template = (el) => html`
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

<pagination-sk offset=${el._pageOffset}
               page_size=${el._pageSize}
               total=${el._totalEntries}
               @page-changed=${el._pageChanged}>
</pagination-sk>
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

${entry.details ? detailsTemplate(el, entry) : html``}
`;

const detailsTemplate = (el, entry) => html`
<tr class=details>
  <td></td>
  <td><strong>Test name</strong></td>
  <td><strong>Digest</strong></td>
  <td><strong>Label</strong></td>
</tr>

${entry.details.map((e) => detailsEntryTemplate(el, e))}

<tr class="details details-separator"><td colspan="4"></td></tr>
`;

const detailsEntryTemplate = (el, detailsEntry) => {
  let detailHref = `/detail?test=${detailsEntry.test_name}&digest=${detailsEntry.digest}`;
  if (el._changelistID) {
    detailHref += `&changelist_id=${el._changelistID}&crs=${el._crs}`;
  }
  return html`
<tr class=details>
  <td></td>
  <td class=test-name>${detailsEntry.test_name}</td>
  <td class=digest>
    <a href=${detailHref} target=_blank rel=noopener>
      ${detailsEntry.digest}
    </a>
  </td>
  <td class=label>${detailsEntry.label}</td>
</tr>
`;
};

define('triagelog-page-sk', class extends ElementSk {
  constructor() {
    super(template);

    this._entries = []; // Log entries fetched from the server.
    this._pageOffset = 0; // Reflected in the URL.
    this._pageSize = 0; // Reflected in the URL.
    this._changelistID = ''; // Reflected in the URL.
    this._crs = ''; // Code Review System (e.g. 'gerrit', 'github')
    this._totalEntries = 0; // Total number of entries in the server.

    // stateReflector will trigger on DomReady.
    this._stateChanged = stateReflector(
      /* getState */ () => ({
        offset: this._pageOffset,
        page_size: this._pageSize,
        changelist_id: this._changelistID,
        crs: this._crs,
      }),
      /* setState */ (newState) => {
        // The stateReflector's lingering popstate event handler will continue
        // to call this function on e.g. browser back button clicks long after
        // this custom element is detached from the DOM.
        if (!this._connected) {
          return;
        }

        this._pageOffset = newState.offset || 0;
        this._pageSize = newState.page_size || 20;
        this._changelistID = newState.changelist_id || '';
        this._crs = newState.crs || '';
        this._render();
        this._fetchEntries();
      },
    );
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  _pageChanged(e) {
    this._pageOffset = Math.max(0, this._pageOffset + e.detail.delta * this._pageSize);
    this._stateChanged();
    this._render();
    this._fetchEntries();
  }

  _undoEntry(entryId) {
    sendBeginTask(this);
    this._fetch(`/json/triagelog/undo?id=${entryId}`, 'POST')
    // The undo RPC returns the first page of results with details hidden.
    // But we always show details, so we need to make another request to
    // fetch the triage log with details from /json/triagelog.
    // TODO(lovisolo): Rethink this after we delete the old triage log page.
      .then(() => this._fetchEntries(/* sendBusyDoneEvents= */ false))
      .then(() => sendEndTask(this))
      .catch((e) => sendFetchError(this, e, 'undo'));
  }

  _fetchEntries(sendBusyDoneEvents = true) {
    let url = `/json/triagelog?details=true&offset=${this._pageOffset}`
        + `&size=${this._pageSize}`;
    if (this._changelistID) {
      url += `&changelist_id=${this._changelistID}&crs=${this._crs}`;
    }
    if (sendBusyDoneEvents) {
      sendBeginTask(this);
    }
    return this._fetch(url, 'GET')
      .then(() => {
        this._render();
        if (sendBusyDoneEvents) {
          sendEndTask(this);
        }
      })
      .catch((e) => sendFetchError(this, e, 'triagelog'));
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
      signal: this._fetchController.signal,
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
});
