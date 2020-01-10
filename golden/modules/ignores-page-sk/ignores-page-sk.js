/**
 * @module module/ignores-page-sk
 * @description <h2><code>ignores-page-sk</code></h2>
 *
 * Page to view/edit/delete ignore rules.
 */

import * as human from 'common-sk/modules/human'

import { classMap } from 'lit-html/directives/class-map.js';
import { define } from 'elements-sk/define'
import { ElementSk } from '../../../infra-sk/modules/ElementSk'
import { html } from 'lit-html'
import { jsonOrThrow } from "../../../common-sk/modules/jsonOrThrow";

import 'elements-sk/checkbox-sk'
import 'elements-sk/icon/delete-icon-sk'
import 'elements-sk/icon/info-outline-icon-sk'
import 'elements-sk/icon/mode-edit-icon-sk'
import 'elements-sk/styles/buttons'

const template = (ele) => html`
<div class=controls>
  <checkbox-sk label="Only count traces with untriaged digests"
               ?checked=${!ele._countAllTraces} @click=${ele._toggleCountAll}></checkbox-sk>

  <button @click=${ele._newIgnoreRule}>Create new ignore rule</button>
</div>

<table>
  <thead>
    <tr>
      <th colspan=2>Filter</th>
      <th>Note</th>
      <th> Traces matched <br> exclusive/all
        <info-outline-icon-sk class=small-icon
            title="'all' is the number of traces that a given ignore rule applies to. 'exclusive' \
is the number of traces which are matched by the given ignore rule and no other ignore rule of the \
rules in this list. If the checkbox is checked to only count traces with untriaged digests, it \
means 'untriaged digests at head', which is typically an indication of a flaky test/config.">
        </info-outline-icon-sk>
      </th>
      <th>Expires in</th>
      <th>Created by</th>
      <th>Updated by</th>
    </tr>
  </thead>
  <tbody>
  ${ele._rules.map((r) => ruleTemplate(ele, r))}
  </tbody>
</table>`;

const ruleTemplate = (ele, r) => {
  const isExpired = Date.parse(r.expires) < Date.now();
  return html`
<tr class=${classMap({expired: isExpired})}>
  <td class=mutate-icons>
    <mode-edit-icon-sk @click=${() => ele._edit(r)}></mode-edit-icon-sk>
    <delete-icon-sk @click=${() => ele._delete(r)}></delete-icon-sk>
  </td>
  <td class=query><a href=${'/list?include=true&query=' + encodeURIComponent(r.query)}
    >${humanReadableQuery(r.query)}</a></td>
  <td>${r.note || '--'}</td>
  <!--TODO(kjlubick) change this to use the All varients with the checkbox -->
  <td title="these counts are recomputed every few minutes">${r.exclusiveCount} / ${r.count}</td>
  <td class=${classMap({expired: isExpired})}>
    ${isExpired ? 'Expired': human.diffDate(r.expires)}
  </td>
  <td title=${'Originally created by ' + r.name}>${trimEmail(r.name)}</td>
  <td title=${'Last updated by ' + r.updatedBy}>
    ${r.name === r.updatedBy ? '': trimEmail(r.updatedBy)}
  </td>
</tr>`;
};

function humanReadableQuery(queryStr) {
  if (!queryStr) {
    return '';
  }
  return queryStr.split('&').join('\n')
}

function trimEmail(s) {
  return s.split('@')[0] + '@';
}

define('ignores-page-sk', class extends ElementSk {
  constructor() {
    super(template);

    this._rules = [];
    this._countAllTraces = false;
    // TODO(kjlubick) state reflector

    // Allows us to abort fetches if we fetch again.
    this._fetchController = null;
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();

    // Run this on the next microtask to allow for the demo.js to be set up.
    // TODO(kjlubick) remove after state reflector is added.
    setTimeout(()=> {
      this._fetch()
    });
  }

  _delete(rule) {
    // TODO(kjlubick)
    console.log('delete', rule);
  }

  _edit(rule) {
    // TODO(kjlubick)
    console.log('edit', rule);
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

    this._sendBusy();

    return fetch('/json/ignores?counts=1', extra)
        .then(jsonOrThrow)
        .then((arr) => {
          this._rules = arr || [];
          this._render();
          this._sendDone();
        })
        .catch((e) => this._sendFetchError(e, 'ignores'));
  }

  _newIgnoreRule(e) {
    // TODO(kjlubick)
    console.log('new rule');
  }

  _sendBusy() {
    this.dispatchEvent(new CustomEvent('begin-task', {bubbles: true}));
  }

  _sendDone() {
    this.dispatchEvent(new CustomEvent('end-task', {bubbles: true}));
  }

  _sendFetchError(e, what) {
    this.dispatchEvent(new CustomEvent('fetch-error', {
      detail: {
        error: e,
        loading: what,
      }, bubbles: true}));
  }

  _toggleCountAll(e) {
    e.preventDefault();
    this._countAllTraces = !this._countAllTraces;
    this._render();
  }
});