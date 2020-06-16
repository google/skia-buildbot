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
import { errorMessage } from 'elements-sk/errorMessage';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { SpinnerSk } from 'elements-sk/spinner-sk/spinner-sk';
import { CommitID, CommitDetailsRequest } from '../json';

import 'elements-sk/spinner-sk';
import 'elements-sk/styles/buttons';

export class JSONSourceSk extends ElementSk {
  private _json: string;
  private _cid: CommitID | null = null;
  private _traceid: string;
  private _spinner: SpinnerSk | null = null;

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

  constructor() {
    super(JSONSourceSk.template);
    this._json = '';
    this._traceid = '';
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
    this._spinner = $$('#spinner', this);
  }

  /** @prop cid - The Commit ID. */
  get cid() {
    return this._cid;
  }

  set cid(val) {
    this._cid = val;
    this._json = '';
    this._render();
  }

  /** @prop traceid - The ID of the trace. */
  get traceid() {
    return this._traceid;
  }

  set traceid(val) {
    this._traceid = val;
    this._json = '';
    this._render();
  }

  _loadSource() {
    this._loadSourceImpl(false);
  }

  _loadSourceSmall() {
    this._loadSourceImpl(true);
  }

  _loadSourceImpl(isSmall: boolean) {
    if (this._spinner!.active === true) {
      return;
    }
    if (!this.traceid) {
      return;
    }
    if (!this.cid || this.cid.offset === undefined) {
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
