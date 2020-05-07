/**
 * @module module/blamelist-panel-sk
 * @description <h2><code>blamelist-panel-sk</code></h2>
 *
 * @evt
 *
 * @attr
 *
 * @example
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { diffDate } from 'common-sk/modules/human';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { truncateWithEllipses } from '../common';

const maxCommitsToDisplay = 15;

const template = (ele) => html`
<h2>Commits:</h2>
<table>
  ${ele._commits.slice(0, maxCommitsToDisplay).map(commitRow)}
</table>
<div>
  ${ele._commits.length > maxCommitsToDisplay ? '...and other commits...' : ''}
</div>`;

const commitRow = (c) => html`
<tr>
  <td title=${c.author}>${truncateWithEllipses(c.author, 20)}</td>
  <td title=${new Date(c.commit_time * 1000)}>
   ${diffDate(c.commit_time * 1000)}
  </td>
  <td><a href=${commitHref(c)}>${c.hash && c.hash.substring(0, 8)}</a></td>
  <td title=${c.message}>${truncateWithEllipses(c.message || '', 80)}</td>
</tr>
`;

const repo = document.body.getAttribute('data-repo');

const commitHref = (commit) => {
  if (!repo) {
    return '';
  }
  if (repo.indexOf('github.com') !== -1) {
    return `${repo}/commit/${commit.hash}`;
  }
  return `${repo}/+/${commit.hash}`;
};

define('blamelist-panel-sk', class extends ElementSk {
  constructor() {
    super(template);

    this._commits = [];
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  get commits() { return this._commits; }

  set commits(arr) {
    this._commits = arr;
    this._render();
  }
});
