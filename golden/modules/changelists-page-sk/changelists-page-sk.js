/**
 * @module modules/changelists-page-sk
 * @description <h2><code>changelists-page-sk</code></h2>
 *
 * Allows the user to page through the ChangeLists for which Gold has seen
 * data uploaded via TryJobs.
 *
 */
import * as human from 'common-sk/modules/human'

import { define } from 'elements-sk/define'
import { ElementSk } from '../../../infra-sk/modules/ElementSk'
import { html } from 'lit-html'
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow'
import { stateReflector } from 'common-sk/modules/stateReflector'

import '../pagination-sk'

const _changelist = (cl) => html`
<tr>
  <td>
    <a title="See codereview in a new window" target=_blank rel=noopener href=${cl.url}>
      ${cl.id}
    </a>
  </td>
  <td>
    <a href="/search?issue=${cl.id}&new_clstore=true"
       target="_blank" rel="noopener">Triage</a>
  </td>
  <td>${cl.owner}</td>
  <td title=${cl.updated}>${human.diffDate(cl.updated)} ago</td>
  <td>${cl.subject}</td>
</tr>`;

const template = (ele) => html`
<div>
  <pagination-sk page_size=${ele._page_size} offset=${ele._offset}
                 total=${ele._total} @page-changed=${ele._pageChanged}>
  </pagination-sk>
</div>

<table>
  <thead>
    <tr>
      <th>ChangeList</th>
      <th></th>
      <th>Owner</th>
      <th>Updated</th>
      <th>Subject</th>
    </tr>
  </thead>
  <tbody>
  ${ele._cls.map(_changelist)}
</tbody>`;

define('changelists-page-sk', class extends ElementSk {
  constructor() {
    super(template);

    // Set empty values to allow empty rendering while we wait for
    // stateReflector (which triggers on DomReady). Additionally, these values
    // help stateReflector with types.
    this._cls = [];
    this._offset = 0;
    this._page_size = 0;
    this._total = 0;

    this._urlParamsLoaded = false;
    this._stateChanged = stateReflector(
      /*getState*/() => {
        return {
          // provide empty values
          'offset': this._offset,
          'page_size': this._page_size,
        }
    }, /*setState*/(newState) => {
      // default values if not specified.
      this._offset = newState.offset || 0;
      this._page_size = newState.page_size || +this.getAttribute('page_size') || 50;
      if (!this._urlParamsLoaded) {
        // initial page load/fetch
        this._urlParamsLoaded = true;
        this._fetch();
      }
      this._render();
    });

    // Allows us to abort fetches if a user pages.
    this._fetchController = null;
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  // Returns a promise that resolves when all outstanding requests resolve
  // or null if none were made. This promise makes unit tests a little more concise.
  _fetch() {
    if (!this._urlParamsLoaded) {
      return null;
    }

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

    this._sendBusy();
    return fetch(`/json/changelists?offset=${this._offset}&size=${this._page_size}`, extra)
      .then(jsonOrThrow)
      .then((json) => {
        this._cls = json.data || [];
        this._offset = json.pagination.offset;
        this._total = json.pagination.total;
        this._stateChanged();
        this._render();
        this._sendDone();
      })
      .catch((e) => this._sendFetchError(e, 'changelists'));
  }

  _pageChanged(e) {
    const d = e.detail;
    this._offset += d.delta * this._page_size;
    if (this._offset < 0) {
      this._offset = 0;
    }
    this._stateChanged();
    this._render();
    this._fetch();
  }

  _sendBusy() {
    this.dispatchEvent(new CustomEvent('begin-task', {bubbles: true}));
  }

  _sendDone() {
    this.dispatchEvent(new CustomEvent('end-task', {bubbles: true}));
  }

  _sendFetchError(e, what) {
    this.dispatchEvent(new CustomEvent('fetch-error', { detail: {
      error: e,
      loading: what,
    }, bubbles: true}));
  }
});
