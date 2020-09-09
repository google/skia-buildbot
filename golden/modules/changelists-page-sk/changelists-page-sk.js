/**
 * @module modules/changelists-page-sk
 * @description <h2><code>changelists-page-sk</code></h2>
 *
 * Allows the user to page through the ChangeLists for which Gold has seen
 * data uploaded via TryJobs.
 *
 */
import * as human from 'common-sk/modules/human';

import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { stateReflector } from 'common-sk/modules/stateReflector';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

import 'elements-sk/checkbox-sk';
import 'elements-sk/icon/block-icon-sk';
import 'elements-sk/icon/cached-icon-sk';
import 'elements-sk/icon/done-icon-sk';

import '../pagination-sk';
import { sendBeginTask, sendEndTask, sendFetchError } from '../common';

const _statusIcon = (cl) => {
  if (cl.status === 'Open') {
    return html`<cached-icon-sk title="ChangeList is open"></cached-icon-sk>`;
  } if (cl.status === 'Landed') {
    return html`<done-icon-sk title="ChangeList was landed"></done-icon-sk>`;
  }
  return html`<block-icon-sk title="ChangeList was abandoned"></block-icon-sk>`;
};

const _changelist = (cl) => html`
<tr>
  <td class=id>
    <a title="See codereview in a new window" target=_blank rel=noopener href=${cl.url}
      >${cl.id}</a>
    ${_statusIcon(cl)}
  </td>
  <td>
    <a href="/cl/${cl.system}/${cl.id}"
       target="_blank" rel="noopener">Triage</a>
  </td>
  <td class=owner>${cl.owner}</td>
  <td title=${cl.updated}>${human.diffDate(cl.updated)} ago</td>
  <td>${cl.subject}</td>
</tr>`;

const template = (ele) => html`
<div class=controls>
  <pagination-sk page_size=${ele._page_size} offset=${ele._offset}
                 total=${ele._total} @page-changed=${ele._pageChanged}>
  </pagination-sk>

  <checkbox-sk label="show all" ?checked=${ele._showAll}
               @click=${ele._toggleShowAll}></checkbox-sk>
</div>

<table>
  <thead>
    <tr>
      <th colspan=2>ChangeList</th>
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
    this._showAll = false;

    this._stateChanged = stateReflector(
      /* getState */() => ({
        // provide empty values
        offset: this._offset,
        page_size: this._page_size,
        show_all: this._showAll,
      }), /* setState */(newState) => {
        if (!this._connected) {
          return;
        }

        // default values if not specified.
        this._offset = newState.offset || 0;
        this._page_size = newState.page_size || +this.getAttribute('page_size') || 50;
        this._showAll = newState.show_all || false;
        this._fetch();
        this._render();
      },
    );

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
    let u = `/json/v1/changelists?offset=${this._offset}&size=${this._page_size}`;
    if (!this._showAll) {
      u += '&active=true';
    }
    return fetch(u, extra)
      .then(jsonOrThrow)
      .then((json) => {
        this._cls = json.data || [];
        this._offset = json.pagination.offset;
        this._total = json.pagination.total;
        this._render();
        sendEndTask(this);
      })
      .catch((e) => sendFetchError(this, e, 'changelists'));
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

  _toggleShowAll(e) {
    e.preventDefault();
    this._showAll = !this._showAll;
    this._stateChanged();
    this._render();
    this._fetch();
  }
});
