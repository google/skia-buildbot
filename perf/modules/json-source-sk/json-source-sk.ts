/**
 * @module modules/json-source-sk
 * @description <h2><code>json-source-sk</code></h2>
 *
 * Displays buttons that, when pressed, retrieve and show the JSON file that
 * was ingested for the point in the trace identified by commit id and trace
 * id.
 *
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { $$ } from 'common-sk/modules/dom';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { SpinnerSk } from 'elements-sk/spinner-sk/spinner-sk';
import { errorMessage } from '../errorMessage';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { CommitDetailsRequest, CommitNumber } from '../json/all';

import 'elements-sk/spinner-sk';
import 'elements-sk/styles/buttons';

export class JSONSourceSk extends ElementSk {
  private _json: string;

  private _cid: CommitNumber = -1;

  private _traceid: string;

  private _spinner: SpinnerSk | null = null;

  constructor() {
    super(JSONSourceSk.template);
    this._json = '';
    this._traceid = '';
  }

  private static template = (ele: JSONSourceSk) => html`
    <div id="controls">
      <button @click=${ele._loadSource}>View Source File</button>
      <button @click=${ele._loadSourceSmall}
        >View Source File Without Results</button
      >
      <spinner-sk id="spinner"></spinner-sk>
    </div>
    <pre>${ele._json}</pre>
  `;

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
    this._spinner = $$('#spinner', this);
  }

  /** @prop cid - The Commit ID. */
  get cid(): number {
    return this._cid;
  }

  set cid(val: number) {
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

  private _loadSource() {
    this._loadSourceImpl(false);
  }

  private _loadSourceSmall() {
    this._loadSourceImpl(true);
  }

  private _loadSourceImpl(isSmall: boolean) {
    if (this._spinner!.active === true) {
      return;
    }
    if (!this.traceid) {
      return;
    }
    if (this.cid === -1) {
      return;
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
    fetch(url, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(body),
    })
      .then(jsonOrThrow)
      .then((json) => {
        this._spinner!.active = false;
        this._json = JSON.stringify(json, null, '  ');
        this._render();
      })
      .catch((e) => {
        this._spinner!.active = false;
        errorMessage(e);
      });
  }
}

define('json-source-sk', JSONSourceSk);
