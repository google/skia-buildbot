/**
 * @module modules/commit-detail-sk
 * @description <h2><code>commit-detail-sk</code></h2>
 *
 * An element to display information around a single commit.
 *
 */
import { html } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { $$ } from '../../../infra-sk/modules/dom';
import { upgradeProperty } from '../../../elements-sk/modules/upgradeProperty';
import { diffDate } from '../../../infra-sk/modules/human';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { Commit, CommitNumber } from '../json';

export class CommitDetailSk extends ElementSk {
  private _cid: Commit;

  constructor() {
    super(CommitDetailSk.template);
    this._cid = {
      author: '',
      message: '',
      url: '',
      ts: 0,
      hash: '',
      offset: CommitNumber(0),
      body: '',
    };
  }

  private static template = (ele: CommitDetailSk) => html`
    <div @click=${() => ele._click()} class="linkish">
      <pre>
${ele.cid.hash.slice(0, 8)} - ${diffDate(ele.cid.ts * 1000)} - ${ele.cid
          .message} - ${ele.cid.author}</pre
      >
    </div>
    <div class="tip hidden">
      <a href="/g/e/${ele.cid.hash}">Explore</a>
      <a href="/g/c/${ele.cid.hash}">Cluster</a>
      <a href="/g/t/${ele.cid.hash}">Triage</a>
      <a href="${ele.cid.url}">Commit</a>
    </div>
  `;

  connectedCallback(): void {
    super.connectedCallback();
    upgradeProperty(this, 'cid');
    this._render();
  }

  private _click() {
    $$('.tip', this)!.classList.toggle('hidden');
  }

  /** The details about a commit. */
  get cid(): Commit {
    return this._cid;
  }

  set cid(val: Commit) {
    this._cid = val;
    this._render();
  }
}

define('commit-detail-sk', CommitDetailSk);
