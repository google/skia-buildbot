/**
 * @module module/last-commit-sk
 * @description <h2><code>last-commit-sk</code></h2>
 *
 * This element polls /json/v1/trstatus every 3 seconds and displays the last commit that had data
 * ingested for it. If there are any network errors, it will log them and retry.
 *
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { baseRepoURL } from '../settings';

const template = (ele) => html`
<div class=last_commit>
  <a href="${commitURL(ele._status)}">
  Last Commit: ${lastCommitText(ele._status)}
  </a>
<div>`;

const commitURL = (status) => {
  const hash = status.lastCommit && status.lastCommit.hash;
  if (!hash) {
    return '';
  }
  const url = baseRepoURL();
  if (url.indexOf('github.com') !== -1) {
    return `${url}/commit/${hash}`;
  }
  return `${url}/+show/${hash}`;
};

const lastCommitText = (status) => {
  const hash = status.lastCommit && status.lastCommit.hash;
  let author = status.lastCommit && status.lastCommit.author;
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

const reloadInterval = 3000;

define('last-commit-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._status = {};
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
      fetch('/json/v1/trstatus')
        .then(jsonOrThrow)
        .then((json) => {
          this._status = json;
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
});
