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
import {
  TriageDelta, TriageDelta2, TriageLogEntry, TriageLogEntry2, TriageLogResponse, TriageLogResponse2,
} from '../rpc_types';
import { PaginationSkPageChangedEventDetail } from '../pagination-sk/pagination-sk';

export class TriagelogPageSk extends ElementSk {
  private static template = (el: TriagelogPageSk) => html`
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
        ${el.entriesV1.map((entry) => TriagelogPageSk.logEntryTemplate(el, entry))}
        ${el.entriesV2.map((entry) => TriagelogPageSk.logEntryTemplate2(el, entry))}
      </tbody>
    </table>

    <pagination-sk offset=${el.pageOffset}
                   page_size=${el.pageSize}
                   total=${el.totalEntries}
                   @page-changed=${el.pageChanged}>
    </pagination-sk>
  `;

  private static logEntryTemplate = (el: TriagelogPageSk, entry: TriageLogEntry) => html`
    <tr>
      <td class=timestamp>${TriagelogPageSk.toLocalDate(entry.ts)}</td>
      <td class=author>${entry.name}</td>
      <td class=num-changes>${entry.changeCount}</td>
      <td class=actions>
        <button @click=${() => el.undoEntry(entry.id)}
                class=undo>
          Undo
        </button>
      </td>
    </tr>

    ${entry.details ? TriagelogPageSk.detailsTemplate(el, entry) : html``}
  `;

  private static logEntryTemplate2 = (el: TriagelogPageSk, entry: TriageLogEntry2) => html`
    <tr>
      <td class=timestamp>${TriagelogPageSk.toLocalDate(entry.ts)}</td>
      <td class=author>${entry.name}</td>
      <td class=num-changes>${entry.details.length}</td>
      <td class=actions>
        <button @click=${() => el.undoEntry(entry.id)}
                class=undo>
          Undo
        </button>
      </td>
    </tr>

    ${entry.details ? TriagelogPageSk.detailsTemplate2(el, entry) : html``}
  `;

  private static detailsTemplate = (el: TriagelogPageSk, entry: TriageLogEntry) => html`
    <tr class=details>
      <td></td>
      <td><strong>Test name</strong></td>
      <td><strong>Digest</strong></td>
      <td><strong>Label</strong></td>
    </tr>

    ${entry.details?.map((e) => TriagelogPageSk.detailsEntryTemplate(el, e))}

    <tr class="details details-separator"><td colspan="4"></td></tr>
  `;

  private static detailsTemplate2 = (el: TriagelogPageSk, entry: TriageLogEntry2) => html`
    <tr class=details>
      <td></td>
      <td><strong>Test name</strong></td>
      <td><strong>Digest</strong></td>
      <td><strong>Label</strong></td>
    </tr>

    ${entry.details?.map((e) => TriagelogPageSk.detailsEntryTemplate2(el, e))}

    <tr class="details details-separator"><td colspan="4"></td></tr>
  `;

  private static detailsEntryTemplate = (el: TriagelogPageSk, detailsEntry: TriageDelta) => {
    let detailHref = `/detail?test=${detailsEntry.test_name}&digest=${detailsEntry.digest}`;
    if (el.changelistID) {
      detailHref += `&changelist_id=${el.changelistID}&crs=${el.crs}`;
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

  private static detailsEntryTemplate2 = (el: TriagelogPageSk, delta: TriageDelta2) => {
    let detailHref = `/detail?test=${delta.grouping.name}&digest=${delta.digest}`;
    if (el.changelistID) {
      detailHref += `&changelist_id=${el.changelistID}&crs=${el.crs}`;
    }
    return html`
      <tr class=details>
        <td></td>
        <td class=test-name title="Grouping ${JSON.stringify(delta.grouping)}">${delta.grouping.name}</td>
        <td class=digest>
          <a href=${detailHref} target=_blank rel=noopener>
            ${delta.digest}
          </a>
        </td>
        <td class=label title="${delta.label_before} => ${delta.label_after}">
          ${delta.label_after}
        </td>
      </tr>
    `;
  };

  private entriesV1: TriageLogEntry[] = []; // Log entries fetched from the server.

  private entriesV2: TriageLogEntry2[] = []; // Log entries fetched from the server.

  private pageOffset = 0; // Reflected in the URL.

  private pageSize = 0; // Reflected in the URL.

  private changelistID = ''; // Reflected in the URL.

  private crs = ''; // Code Review System (e.g. 'gerrit', 'github')

  private totalEntries = 0; // Total number of entries in the server.

  private useNewAPI = false;

  private readonly stateChanged: ()=> void;

  private fetchController?: AbortController;

  constructor() {
    super(TriagelogPageSk.template);

    // stateReflector will trigger on DomReady.
    this.stateChanged = stateReflector(
      /* getState */ () => ({
        offset: this.pageOffset,
        page_size: this.pageSize,
        changelist_id: this.changelistID,
        crs: this.crs,
        use_new_api: this.useNewAPI,
      }),
      /* setState */ (newState) => {
        // The stateReflector's lingering popstate event handler will continue
        // to call this function on e.g. browser back button clicks long after
        // this custom element is detached from the DOM.
        if (!this._connected) {
          return;
        }

        this.pageOffset = newState.offset as number || 0;
        this.pageSize = newState.page_size as number || 20;
        this.changelistID = newState.changelist_id as string || '';
        this.crs = newState.crs as string || '';
        this.useNewAPI = (newState.use_new_api as boolean) || false;
        this._render();
        this.fetchEntries();
      },
    );
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
  }

  private pageChanged(e: CustomEvent<PaginationSkPageChangedEventDetail>) {
    this.pageOffset = Math.max(0, this.pageOffset + e.detail.delta * this.pageSize);
    this.stateChanged();
    this._render();
    this.fetchEntries();
  }

  private undoEntry(entryId: string) {
    sendBeginTask(this);
    this.fetchV1(`/json/v1/triagelog/undo?id=${entryId}`, 'POST')
    // The undo RPC returns the first page of results with details hidden.
    // But we always show details, so we need to make another request to
    // fetch the triage log with details from /json/v1/triagelog.
    // TODO(lovisolo): Rethink this after we delete the old triage log page.
      .then(() => this.fetchEntries(/* sendBusyDoneEvents= */ false))
      .then(() => sendEndTask(this))
      .catch((e) => sendFetchError(this, e, 'undo'));
  }

  private fetchEntries(sendBusyDoneEvents = true): Promise<void> {
    if (!this.useNewAPI) {
      let url = `/json/v1/triagelog?details=true&offset=${this.pageOffset}`
          + `&size=${this.pageSize}`;
      if (this.changelistID) {
        url += `&changelist_id=${this.changelistID}&crs=${this.crs}`;
      }
      if (sendBusyDoneEvents) {
        sendBeginTask(this);
      }
      return this.fetchV1(url, 'GET')
        .then(() => {
          this._render();
          if (sendBusyDoneEvents) {
            sendEndTask(this);
          }
        })
        .catch((e) => sendFetchError(this, e, 'triagelog'));
    }
    let url = `/json/v2/triagelog?offset=${this.pageOffset}&size=${this.pageSize}`;
    if (this.changelistID) {
      url += `&changelist_id=${this.changelistID}&crs=${this.crs}`;
    }
    if (sendBusyDoneEvents) {
      sendBeginTask(this);
    }
    return this.fetchV2(url, 'GET')
      .then(() => {
        this._render();
        if (sendBusyDoneEvents) {
          sendEndTask(this);
        }
      })
      .catch((e) => sendFetchError(this, e, 'triagelog'));
  }

  // Both /json/v1/triagelog and /json/v1/triagelog/undo RPCs return the same kind of
  // response, which is a page with triage log entries. Therefore this method is
  // called by both fetchEntries and undoEntry to carry out their
  // corresponding RPCs and handle the server response.
  private fetchV1(url: string, method: 'GET' | 'POST'): Promise<void> {
    // Force only one fetch at a time. Abort any outstanding requests.
    if (this.fetchController) {
      this.fetchController.abort();
    }
    this.fetchController = new AbortController();

    const options: RequestInit = {
      method: method,
      signal: this.fetchController.signal,
    };

    return fetch(url, options)
      .then(jsonOrThrow)
      .then((response: TriageLogResponse) => {
        this.entriesV1 = response.entries || [];
        this.pageOffset = response.offset || 0;
        this.pageSize = response.size || 0;
        this.totalEntries = response.total || 0;
      });
  }

  private fetchV2(url: string, method: 'GET' | 'POST'): Promise<void> {
    // Force only one fetch at a time. Abort any outstanding requests.
    if (this.fetchController) {
      this.fetchController.abort();
    }
    this.fetchController = new AbortController();

    const options: RequestInit = {
      method: method,
      signal: this.fetchController.signal,
    };

    return fetch(url, options)
      .then(jsonOrThrow)
      .then((response: TriageLogResponse2) => {
        this.entriesV2 = response.entries || [];
        this.pageOffset = response.offset || 0;
        this.pageSize = response.size || 0;
        this.totalEntries = response.total || 0;
      });
  }

  private static toLocalDate(timeStampMS: number): string {
    return new Date(timeStampMS).toLocaleString();
  }
}

define('triagelog-page-sk', TriagelogPageSk);
