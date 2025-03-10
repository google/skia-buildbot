/**
 * @module modules/commit-detail-sk
 * @description <h2><code>commit-detail-sk</code></h2>
 *
 * An element to display information around a single commit.
 *
 */
import { html } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { upgradeProperty } from '../../../elements-sk/modules/upgradeProperty';
import { diffDate } from '../../../infra-sk/modules/human';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { Commit, CommitNumber } from '../json';

import '@material/web/button/outlined-button.js';

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
    <div class="linkish">
      <pre>
${ele.cid.hash.slice(0, 8)} - ${ele.cid.author} - ${diffDate(ele.cid.ts * 1000)} - ${ele.cid
          .message}</pre
      >
      <div class="tip">
        <md-outlined-button @click=${() => ele.openLink(`/g/e/${ele.cid.hash}`)}>
          Explore
        </md-outlined-button>
        <md-outlined-button @click=${() => ele.openLink(`/g/c/${ele.cid.hash}`)}>
          Cluster
        </md-outlined-button>
        <md-outlined-button @click=${() => ele.openLink(`/g/t/${ele.cid.hash}`)}>
          Triage
        </md-outlined-button>
        <md-outlined-button @click=${() => ele.openLink(ele.cid.url)}> Commit </md-outlined-button>
      </div>
    </div>
  `;

  connectedCallback(): void {
    super.connectedCallback();
    upgradeProperty(this, 'cid');
    this._render();
  }

  private openLink(link: string): void {
    window.open(link);
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
