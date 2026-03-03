/**
 * @module modules/json-source-sk
 * @description <h2><code>json-source-sk</code></h2>
 *
 * Displays buttons that, when pressed, retrieve and show the JSON file that
 * was ingested for the point in the trace identified by commit id and trace
 * id.
 *
 */
import { html, LitElement, nothing } from 'lit';
import { customElement, property, state, query } from 'lit/decorators.js';
import { Task } from '@lit/task';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { errorMessage } from '../errorMessage';
import { CommitDetailsRequest, CommitNumber } from '../json';

import '../../../elements-sk/modules/spinner-sk';
import { validKey } from '../paramtools';

import '@material/web/button/outlined-button.js';

@customElement('json-source-sk')
export class JSONSourceSk extends LitElement {
  @property({ type: Number })
  cid: CommitNumber = CommitNumber(-1);

  @property({ type: String })
  traceid: string = '';

  @state()
  private _fetchParams: { isSmall: boolean } | null = null;

  @state()
  private _dialogOpen = false;

  @query('.json-dialog')
  private dialog!: HTMLDialogElement;

  private _fetchCommitDetailsTask = new Task(this, {
    task: async ([params], { signal }) => {
      if (!params || !this.validTraceID() || this.cid === -1) {
        // Return null or initial state if we shouldn't fetch yet
        return null;
      }

      try {
        const body: CommitDetailsRequest = {
          cid: this.cid,
          traceid: this.traceid,
        };

        let url = '/_/details/';
        if (params.isSmall) {
          url += '?results=false';
        }

        const response = await fetch(url, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
          },
          body: JSON.stringify(body),
          signal,
        });

        const json = await jsonOrThrow(response);
        return JSON.stringify(json, null, '  ');
      } catch (e) {
        errorMessage(e as any);
        throw e;
      }
    },
    args: () => [this._fetchParams] as const,
  });

  createRenderRoot() {
    return this;
  }

  // Handle opening/closing the modal dialog based on state changes.
  updated(changedProperties: Map<string, any>) {
    if (changedProperties.has('_dialogOpen')) {
      if (this._dialogOpen) {
        if (!this.dialog.open) {
          this.dialog.showModal();
        }
      } else {
        if (this.dialog.open) {
          this.dialog.close();
        }
      }
    }
  }

  private validTraceID(): boolean {
    return validKey(this.traceid);
  }

  private _loadSource() {
    if (!this.validTraceID() || this.cid === -1) {
      return;
    }
    this._fetchParams = { isSmall: false };
    this._dialogOpen = true;
  }

  private _loadSourceSmall() {
    if (!this.validTraceID() || this.cid === -1) {
      return;
    }
    this._fetchParams = { isSmall: true };
    this._dialogOpen = true;
  }

  private _closeJsonDialog() {
    this._dialogOpen = false;
    this._fetchParams = null;
  }

  render() {
    return html`
      <div class="controls" ?hidden=${!this.validTraceID()}>
        <button class="view-source" @click=${this._loadSource}>View Json File</button>
        <button class="load-source" @click=${this._loadSourceSmall}>View Short Json File</button>
      </div>
      <dialog class="json-dialog">
        <button class="closeIcon" @click=${this._closeJsonDialog} aria-label="Close">
          <close-icon-sk></close-icon-sk>
        </button>

        ${this._fetchCommitDetailsTask.render({
          pending: () => html`<spinner-sk active class="spinner"></spinner-sk>`,
          complete: (json) => {
            if (json === null) return nothing;
            return html`
              <div class="json-source">
                <pre>${json}</pre>
              </div>
            `;
          },
          error: () => html`<div class="error">Error loading JSON</div>`,
        })}
      </dialog>
    `;
  }
}

// Global declaration for TS
declare global {
  interface HTMLElementTagNameMap {
    'json-source-sk': JSONSourceSk;
  }
}
