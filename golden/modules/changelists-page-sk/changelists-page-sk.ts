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
import { Changelist, ChangelistsResponse } from '../rpc_types';
import { PaginationSkPageChangedEventDetail } from '../pagination-sk/pagination-sk';

export class ChangelistsPageSk extends ElementSk {
  private static template = (ele: ChangelistsPageSk) => html`
    <div class=controls>
      <pagination-sk page_size=${ele.pageSize} offset=${ele.offset}
                     total=${ele.total} @page-changed=${ele.pageChanged}>
      </pagination-sk>

      <checkbox-sk label="show all" ?checked=${ele.showAll}
                   @click=${ele.toggleShowAll}></checkbox-sk>
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
      ${ele.cls.map(ChangelistsPageSk.changelist)}
    </tbody>
  `;

  private static changelist = (cl: Changelist) => html`
    <tr>
      <td class=id>
        <a title="See codereview in a new window" target=_blank rel=noopener href=${cl.url}
          >${cl.id}</a>
        ${ChangelistsPageSk.statusIcon(cl)}
      </td>
      <td>
        <a href="/cl/${cl.system}/${cl.id}"
           target="_blank" rel="noopener">Triage</a>
      </td>
      <td class=owner>${cl.owner}</td>
      <td title=${cl.updated}>${human.diffDate(cl.updated)} ago</td>
      <td>${cl.subject}</td>
    </tr>
  `;

  private static statusIcon = (cl: Changelist) => {
    if (cl.status === 'Open') {
      return html`<cached-icon-sk title="ChangeList is open"></cached-icon-sk>`;
    } if (cl.status === 'Landed') {
      return html`<done-icon-sk title="ChangeList was landed"></done-icon-sk>`;
    }
    return html`<block-icon-sk title="ChangeList was abandoned"></block-icon-sk>`;
  };

  // Set empty values to allow empty rendering while we wait for
  // stateReflector (which triggers on DomReady). Additionally, these values
  // help stateReflector with types.
  private cls: Changelist[] = [];
  private offset = 0;
  private pageSize = 0;
  private total = 0;
  private showAll = false;

  private readonly stateChanged: () => void;

  // Allows us to abort fetches if a user pages.
  private fetchController?: AbortController;

  constructor() {
    super(ChangelistsPageSk.template);
    this.stateChanged = stateReflector(
      /* getState */() => ({
        // provide empty values
        offset: this.offset,
        page_size: this.pageSize,
        show_all: this.showAll,
      }), /* setState */(newState) => {
        if (!this._connected) {
          return;
        }

        // default values if not specified.
        this.offset = newState.offset as number || 0;
        this.pageSize =
            newState.page_size as number || +this.getAttribute('page_size')! || 50;
        this.showAll = newState.show_all as boolean || false;
        this.fetch();
        this._render();
      },
    );
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  // Returns a promise that resolves when all outstanding requests resolve
  // or null if none were made. This promise makes unit tests a little more concise.
  private fetch(): Promise<void> {
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
    let u = `/json/v1/changelists?offset=${this.offset}&size=${this.pageSize}`;
    if (!this.showAll) {
      u += '&active=true';
    }
    return fetch(u, extra)
      .then(jsonOrThrow)
      .then((response: ChangelistsResponse) => {
        this.cls = response.changelists || [];
        this.offset = response.offset;
        this.total = response.total;
        this._render();
        sendEndTask(this);
      })
      .catch((e) => sendFetchError(this, e, 'changelists'));
  }

  private pageChanged(e: CustomEvent<PaginationSkPageChangedEventDetail>) {
    const d = e.detail;
    this.offset += d.delta * this.pageSize;
    if (this.offset < 0) {
      this.offset = 0;
    }
    this.stateChanged();
    this._render();
    this.fetch();
  }

  private toggleShowAll(e: Event) {
    e.preventDefault();
    this.showAll = !this.showAll;
    this.stateChanged();
    this._render();
    this.fetch();
  }
}

define('changelists-page-sk', ChangelistsPageSk);
