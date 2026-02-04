/**
 * @module modules/json-source-sk
 * @description <h2><code>json-source-sk</code></h2>
 *
 * Displays buttons that, when pressed, retrieve and show the JSON file that
 * was ingested for the point in the trace identified by commit id and trace
 * id.
 *
 */
import { html } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { $$ } from '../../../infra-sk/modules/dom';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { SpinnerSk } from '../../../elements-sk/modules/spinner-sk/spinner-sk';
import { errorMessage } from '../errorMessage';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { CommitDetailsRequest, CommitNumber } from '../json';

import '../../../elements-sk/modules/spinner-sk';
import { validKey } from '../paramtools';

export class JSONSourceSk extends ElementSk {
  private _json: string;

  private _cid: CommitNumber = CommitNumber(-1);

  private _traceid: string;

  private _spinner: SpinnerSk | null = null;

  private showJsonDialog: HTMLDialogElement | null = null;

  constructor() {
    super(JSONSourceSk.template);
    this._json = '';
    this._traceid = '';
  }

  private static template = (ele: JSONSourceSk) => html`
    <div class="controls" ?hidden=${!ele.validTraceID()}>
      <button id="view-source" @click=${ele._loadSource}>View Json File</button>

      <button id="load-source" @click=${ele._loadSourceSmall}>View Short Json File</button>
    </div>

    <dialog id="json-dialog">
      <button id="closeIcon" @click=${ele.closeJsonDialog}>
        <close-icon-sk></close-icon-sk>
      </button>

      <spinner-sk id="spinner"></spinner-sk>

      ${ele.jsonFile()}
    </dialog>
  `;

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
    this._spinner = $$('#spinner', this);
    this.showJsonDialog = this.querySelector('#json-dialog');
  }

  private validTraceID(): boolean {
    return validKey(this._traceid);
  }

  /** @prop cid - The Commit ID. */
  get cid(): CommitNumber {
    return this._cid;
  }

  set cid(val: CommitNumber) {
    this._cid = val;
    this._json = '';
    this._render();
  }

  /** @prop traceid - The ID of the trace. */
  get traceid(): string {
    return this._traceid;
  }

  set traceid(val: string) {
    this._traceid = val;
    this._json = '';
    this._render();
  }

  private async _loadSource() {
    if (await this._loadSourceImpl(false)) {
      this.openJsonDialog();
    }
  }

  private async _loadSourceSmall() {
    if (await this._loadSourceImpl(true)) {
      this.openJsonDialog();
    }
  }

  private async _loadSourceImpl(isSmall: boolean) {
    if (this._spinner!.active === true) {
      return false;
    }
    if (!this.validTraceID()) {
      return false;
    }
    if (this.cid === -1) {
      return false;
    }
    const body: CommitDetailsRequest = {
      cid: this.cid,
      traceid: this.traceid,
    };
    this._spinner!.active = true;
    let url = '/_/details/';
    if (isSmall) {
      url += '?results=false';
    }
    return await fetch(url, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(body),
    })
      .then(jsonOrThrow)
      .then(async (json) => {
        this._json = JSON.stringify(json, null, '  ');
        this._spinner!.active = false;
        this._render();
        return true;
      })
      .catch((e) => {
        this._spinner!.active = false;
        errorMessage(e);
        return false;
      });
  }

  private jsonFile() {
    if (this._json! !== '') {
      return html` <div id="json-source">
        <pre>${this._json}</pre>
      </div>`;
    }
  }

  private openJsonDialog() {
    this._render();
    this.showJsonDialog!.showModal();
  }

  private closeJsonDialog(): void {
    this._json = '';
    this._render();
    this.showJsonDialog!.close();
  }
}

define('json-source-sk', JSONSourceSk);
