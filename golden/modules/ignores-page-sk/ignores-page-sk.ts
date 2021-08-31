/**
 * @module module/ignores-page-sk
 * @description <h2><code>ignores-page-sk</code></h2>
 *
 * Page to view/edit/delete ignore rules.
 */

import * as human from 'common-sk/modules/human';
import dialogPolyfill from 'dialog-polyfill';

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
import {
  IgnoreRule, IgnoreRuleBody, IgnoresResponse, ParamSet,
} from '../rpc_types';
import { EditIgnoreRuleSk } from '../edit-ignore-rule-sk/edit-ignore-rule-sk';
import { ConfirmDialogSk } from '../../../infra-sk/modules/confirm-dialog-sk/confirm-dialog-sk';

import '../../../infra-sk/modules/confirm-dialog-sk';
import '../edit-ignore-rule-sk';
import 'elements-sk/checkbox-sk';
import 'elements-sk/icon/delete-icon-sk';
import 'elements-sk/icon/info-outline-icon-sk';
import 'elements-sk/icon/mode-edit-icon-sk';
import 'elements-sk/styles/buttons';

function trimEmail(s: string) {
  return `${s.split('@')[0]}@`;
}

export class IgnoresPageSk extends ElementSk {
  private static template = (ele: IgnoresPageSk) => html`
    <div class=controls>
      <checkbox-sk label="Only count traces with untriaged digests"
                   ?checked=${!ele.countAllTraces} @click=${ele.toggleCountAll}></checkbox-sk>

      <button @click=${ele.newIgnoreRule} class=create>Create new ignore rule</button>
    </div>

    <confirm-dialog-sk></confirm-dialog-sk>

    <dialog id=edit-ignore-rule-dialog>
      <h2>${ele.ruleID ? 'Edit Ignore Rule' : 'Create Ignore Rule'}</h2>
      <edit-ignore-rule-sk .paramset=${ele.paramset}></edit-ignore-rule-sk>
      <button @click=${() => ele.editIgnoreRuleDialog?.close()}>Cancel</button>
      <button id=ok class=action @click=${ele.saveIgnoreRule}>
        ${ele.ruleID ? 'Update' : 'Create'}
      </button>
    </dialog>

    <table>
      <thead>
        <tr>
          <th colspan=2>Filter</th>
          <th>Note</th>
          <th> Traces matched <br> exclusive/all
            <info-outline-icon-sk class=small-icon
                title="'all' is the number of traces that a given ignore rule applies to. \
    'exclusive' is the number of traces which are matched by the given ignore rule and no other \
    ignore rule of the rules in this list. If the checkbox is checked to only count traces with \
    untriaged digests, it means 'untriaged digests at head', which is typically an indication of \
    a flaky test/config.">
            </info-outline-icon-sk>
          </th>
          <th>Expires in</th>
          <th>Created by</th>
          <th>Updated by</th>
        </tr>
      </thead>
      <tbody>
      ${ele.rules.map((r) => IgnoresPageSk.ruleTemplate(ele, r))}
      </tbody>
    </table>
  `;

  private static ruleTemplate = (ele: IgnoresPageSk, r: IgnoreRule) => {
    const isExpired = Date.parse(r.expires) < Date.now();
    return html`
      <tr class=${classMap({ expired: isExpired })}>
        <td class=mutate-icons>
          <mode-edit-icon-sk title="Edit this rule."
              @click=${() => ele.editIgnoreRule(r)}></mode-edit-icon-sk>
          <delete-icon-sk title="Delete this rule."
              @click=${() => ele.deleteIgnoreRule(r)}></delete-icon-sk>
        </td>
        <td class=query><a href=${`/list?include=true&query=${encodeURIComponent(r.query)}`}
          >${humanReadableQuery(r.query)}</a></td>
        <td>${escapeAndLinkify(r.note) || '--'}</td>
        <td class=matches title="These counts are recomputed every few minutes.">
          ${ele.countAllTraces ? r.exclusiveCountAll : r.exclusiveCount} /
          ${ele.countAllTraces ? r.countAll : r.count}
        </td>
        <td class=${classMap({ expired: isExpired })}>
          ${isExpired ? 'Expired' : human.diffDate(r.expires)}
        </td>
        <td title=${`Originally created by ${r.name}`}>${trimEmail(r.name)}</td>
        <td title=${`Last updated by ${r.updatedBy}`}>
          ${r.name === r.updatedBy ? '' : trimEmail(r.updatedBy)}
        </td>
      </tr>
    `;
  };

  private rules: IgnoreRule[] = [];

  private paramset: ParamSet = {};

  private countAllTraces = false;

  private useOldAPI = false;

  private ruleID = '';

  private editIgnoreRuleDialog?: HTMLDialogElement; // Dialog for creating or editing rules.

  private editIgnoreRuleSk?: EditIgnoreRuleSk;

  private confirmDialogSk?: ConfirmDialogSk;

  private readonly stateChanged: ()=> void;

  private fetchController?: AbortController; // Allows us to abort fetches if we fetch again.

  constructor() {
    super(IgnoresPageSk.template);

    this.stateChanged = stateReflector(
      /* getState */() => ({
        // provide empty values
        count_all: this.countAllTraces,
        use_old_api: this.useOldAPI,
      }), /* setState */(newState) => {
        if (!this._connected) {
          return;
        }

        // default values if not specified.
        this.countAllTraces = newState.count_all as boolean || false;
        this.useOldAPI = (newState.use_old_api === 'true') || false;
        this.fetch();
        this._render();
      },
    );
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
    this.editIgnoreRuleDialog = this.querySelector<HTMLDialogElement>('#edit-ignore-rule-dialog')!;
    dialogPolyfill.registerDialog(this.editIgnoreRuleDialog);
    this.editIgnoreRuleSk = this.querySelector<EditIgnoreRuleSk>('edit-ignore-rule-sk')!;
    this.confirmDialogSk = this.querySelector<ConfirmDialogSk>('confirm-dialog-sk')!;
  }

  private deleteIgnoreRule(rule: IgnoreRule) {
    this.confirmDialogSk!.open('Are you sure you want to delete '
      + 'this ignore rule?').then(() => {
      sendBeginTask(this);
      fetch(`/json/v1/ignores/del/${rule.id}`, {
        method: 'POST',
      }).then(jsonOrThrow).then(() => {
        this.fetch();
        sendEndTask(this);
      }).catch((e) => sendFetchError(this, e, 'deleting ignore'));
    });
  }

  private editIgnoreRule(rule: IgnoreRule) {
    this.editIgnoreRuleSk!.reset();
    this.editIgnoreRuleSk!.query = rule.query;
    this.editIgnoreRuleSk!.note = rule.note;
    this.editIgnoreRuleSk!.expires = rule.expires;
    this.ruleID = rule.id;
    this._render();
    this.editIgnoreRuleDialog!.showModal();
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

    // We always want the counts of the ignore rules, thus the parameter counts=1.
    // The v2 API does this by default
    const url = this.useOldAPI ? '/json/v1/ignores?counts=1' : '/json/v2/ignores';
    fetch(url, extra)
      .then(jsonOrThrow)
      .then((response: IgnoresResponse) => {
        this.rules = response.rules || [];
        this._render();
        sendEndTask(this);
      })
      .catch((e) => sendFetchError(this, e, 'ignores'));

    const paramsUrl = this.useOldAPI ? '/json/v1/paramset' : '/json/v2/paramset';
    fetch(paramsUrl, extra)
      .then(jsonOrThrow)
      .then((paramset: ParamSet) => {
        this.paramset = paramset;
        this._render();
        sendEndTask(this);
      })
      .catch((e) => sendFetchError(this, e, 'paramset'));
  }

  private newIgnoreRule() {
    this.editIgnoreRuleSk!.reset();
    this.ruleID = '';
    this._render();
    this.editIgnoreRuleDialog!.showModal();
  }

  private saveIgnoreRule() {
    if (this.editIgnoreRuleSk!.verifyFields()) {
      const body: IgnoreRuleBody = {
        duration: this.editIgnoreRuleSk!.expires,
        filter: this.editIgnoreRuleSk!.query,
        note: this.editIgnoreRuleSk!.note,
      };
      // TODO(kjlubick) remove the / from the json endpoint
      let url = '/json/v1/ignores/add/';
      if (this.ruleID) {
        url = `/json/v1/ignores/save/${this.ruleID}`;
      }

      sendBeginTask(this);
      fetch(url, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(body),
      }).then(jsonOrThrow).then(() => {
        this.fetch();
        sendEndTask(this);
      }).catch((e) => sendFetchError(this, e, 'saving ignore'));

      this.editIgnoreRuleSk!.reset();
      this.editIgnoreRuleDialog!.close();
    }
  }

  private toggleCountAll(e: Event) {
    e.preventDefault();
    this.countAllTraces = !this.countAllTraces;
    this.stateChanged();
    this._render();
  }
}

define('ignores-page-sk', IgnoresPageSk);
