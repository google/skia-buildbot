/**
 * @module module/ignores-page-sk
 * @description <h2><code>ignores-page-sk</code></h2>
 *
 * Page to view/edit/delete ignore rules.
 */

import * as human from 'common-sk/modules/human';
import dialogPolyfill from 'dialog-polyfill';

import { $$ } from 'common-sk/modules/dom';
import { classMap } from 'lit-html/directives/class-map';
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { stateReflector } from 'common-sk/modules/stateReflector';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { escapeAndLinkify } from '../../../infra-sk/modules/linkify';
import {
  humanReadableQuery, sendBeginTask, sendEndTask, sendFetchError,
} from '../common';

import '../../../infra-sk/modules/confirm-dialog-sk';
import '../edit-ignore-rule-sk';
import 'elements-sk/checkbox-sk';
import 'elements-sk/icon/delete-icon-sk';
import 'elements-sk/icon/info-outline-icon-sk';
import 'elements-sk/icon/mode-edit-icon-sk';
import 'elements-sk/styles/buttons';

const template = (ele) => html`
<div class=controls>
  <checkbox-sk label="Only count traces with untriaged digests"
               ?checked=${!ele._countAllTraces} @click=${ele._toggleCountAll}></checkbox-sk>

  <button @click=${ele._newIgnoreRule} class=create>Create new ignore rule</button>
</div>

<confirm-dialog-sk></confirm-dialog-sk>

<dialog id=edit-ignore-rule-dialog>
  <h2>${ele._ruleID ? 'Edit Ignore Rule' : 'Create Ignore Rule'}</h2>
  <edit-ignore-rule-sk .paramset=${ele._paramset}></edit-ignore-rule-sk>
  <button @click=${() => ele._editIgnoreRuleDialog.close()}>Cancel</button>
  <button id=ok class=action @click=${ele._saveIgnoreRule}>
    ${ele._ruleID ? 'Update' : 'Create'}
  </button>
</dialog>

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
<tr class=${classMap({ expired: isExpired })}>
  <td class=mutate-icons>
    <mode-edit-icon-sk title="Edit this rule."
        @click=${() => ele._editIgnoreRule(r)}></mode-edit-icon-sk>
    <delete-icon-sk title="Delete this rule."
        @click=${() => ele._deleteIgnoreRule(r)}></delete-icon-sk>
  </td>
  <td class=query><a href=${`/list?include=true&query=${encodeURIComponent(r.query)}`}
    >${humanReadableQuery(r.query)}</a></td>
  <td>${escapeAndLinkify(r.note) || '--'}</td>
  <td class=matches title="These counts are recomputed every few minutes.">
    ${ele._countAllTraces ? r.exclusiveCountAll : r.exclusiveCount} /
    ${ele._countAllTraces ? r.countAll : r.count}
  </td>
  <td class=${classMap({ expired: isExpired })}>
    ${isExpired ? 'Expired' : human.diffDate(r.expires)}
  </td>
  <td title=${`Originally created by ${r.name}`}>${trimEmail(r.name)}</td>
  <td title=${`Last updated by ${r.updatedBy}`}>
    ${r.name === r.updatedBy ? '' : trimEmail(r.updatedBy)}
  </td>
</tr>`;
};

function trimEmail(s) {
  return `${s.split('@')[0]}@`;
}

define('ignores-page-sk', class extends ElementSk {
  constructor() {
    super(template);

    this._rules = [];
    this._paramset = {};
    this._countAllTraces = false;

    this._stateChanged = stateReflector(
      /* getState */() => ({
        // provide empty values
        count_all: this._countAllTraces,
      }), /* setState */(newState) => {
        if (!this._connected) {
          return;
        }

        // default values if not specified.
        this._countAllTraces = newState.count_all || false;
        this._fetch();
        this._render();
      },
    );
    // Allows us to abort fetches if we fetch again.
    this._fetchController = null;
    // This is the dialog element for creating or editing rules.
    this._editIgnoreRuleDialog = null;
    this._ruleID = '';
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
    this._editIgnoreRuleDialog = $$('#edit-ignore-rule-dialog', this);
    dialogPolyfill.registerDialog(this._editIgnoreRuleDialog);
  }

  _deleteIgnoreRule(rule) {
    const dialog = $$('confirm-dialog-sk', this);
    dialog.open('Are you sure you want to delete '
      + 'this ignore rule?').then(() => {
      sendBeginTask(this);
      fetch(`/json/ignores/del/${rule.id}`, {
        method: 'POST',
      }).then(jsonOrThrow).then(() => {
        this._fetch();
        sendEndTask(this);
      }).catch((e) => sendFetchError(this, e, 'deleting ignore'));
    });
  }

  _editIgnoreRule(rule) {
    const editor = $$('edit-ignore-rule-sk', this);
    editor.reset();
    editor.query = rule.query;
    editor.note = rule.note;
    editor.expires = rule.expires;
    this._ruleID = rule.id;
    this._render();
    this._editIgnoreRuleDialog.showModal();
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

    // We always want the counts of the ignore rules, thus the parameter counts=1.
    fetch('/json/ignores?counts=1', extra)
      .then(jsonOrThrow)
      .then((arr) => {
        this._rules = arr || [];
        this._render();
        sendEndTask(this);
      })
      .catch((e) => sendFetchError(this, e, 'ignores'));

    fetch('/json/paramset', extra)
      .then(jsonOrThrow)
      .then((paramset) => {
        this._paramset = paramset;
        this._render();
        sendEndTask(this);
      })
      .catch((e) => sendFetchError(this, e, 'paramset'));
  }

  _newIgnoreRule() {
    const editor = $$('edit-ignore-rule-sk', this);
    editor.reset();
    this._ruleID = '';
    this._render();
    this._editIgnoreRuleDialog.showModal();
  }

  _saveIgnoreRule() {
    const editor = $$('edit-ignore-rule-sk', this);
    if (editor.verifyFields()) {
      const body = {
        duration: editor.expires,
        filter: editor.query,
        note: editor.note,
      };
      // TODO(kjlubick) remove the / from the json endpoint
      let url = '/json/ignores/add/';
      if (this._ruleID) {
        url = `/json/ignores/save/${this._ruleID}`;
      }

      sendBeginTask(this);
      fetch(url, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(body),
      }).then(jsonOrThrow).then(() => {
        this._fetch();
        sendEndTask(this);
      }).catch((e) => sendFetchError(this, e, 'saving ignore'));

      editor.reset();
      this._editIgnoreRuleDialog.close();
    }
  }

  _toggleCountAll(e) {
    e.preventDefault();
    this._countAllTraces = !this._countAllTraces;
    this._stateChanged();
    this._render();
  }
});
