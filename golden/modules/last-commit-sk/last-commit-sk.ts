/**
 * @module module/last-commit-sk
 * @description <h2><code>last-commit-sk</code></h2>
 *
 * This element polls /json/v2/trstatus every 3 seconds and displays the last commit that had data
 * ingested for it. If there are any network errors, it will log them and retry.
 *
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { baseRepoURL } from '../settings';
import { StatusResponse } from '../rpc_types';

const reloadInterval = 3000;

export class LastCommitSk extends ElementSk {
  private static template = (ele: LastCommitSk) => html`
    <div class=last_commit>
      <a href="${LastCommitSk.commitURL(ele.status)}">
      Last Commit: ${LastCommitSk.lastCommitText(ele.status)}
      </a>
    <div>
  `;

  private static commitURL = (status: StatusResponse | null): string => {
    const hash = status?.lastCommit?.hash;
    if (!hash) {
      return '';
    }
    const url = baseRepoURL();
    if (url.indexOf('github.com') !== -1) {
      return `${url}/commit/${hash}`;
    }
    return `${url}/+show/${hash}`;
  };

  private static lastCommitText = (status: StatusResponse | null): string => {
    const hash = status?.lastCommit?.hash;
    let author = status?.lastCommit?.author;
    if (!hash || !author) {
      return '';
    }
    // Trim off the email address (usually in parens) if it exists.
    const idx = author.indexOf('(');
    if (idx) {
      author = author.substring(0, idx);
    }
    return `${hash.substring(0, 7)} - ${author}`;
  };

  private status: StatusResponse | null = null;

  constructor() {
    super(LastCommitSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
    // We intentionally do not use setInterval here because we don't want multiple requests to
    // the RPC to pile up if the server gets behind.
    const reload = () => {
      if (!this._connected) {
        return;
      }
      fetch('/json/v2/trstatus')
        .then(jsonOrThrow)
        .then((json) => {
          this.status = json as StatusResponse;
          this._render();
          setTimeout(reload, reloadInterval);
        })
        .catch((e) => {
          console.warn('Error fetching status', e);
          // Keep the old data the same
          setTimeout(reload, reloadInterval);
        });
    };
    reload();
  }
}

define('last-commit-sk', LastCommitSk);
