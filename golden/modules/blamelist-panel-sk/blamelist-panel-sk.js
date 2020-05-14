/**
 * @module module/blamelist-panel-sk
 * @description <h2><code>blamelist-panel-sk</code></h2>
 *
 * A list of commits and authors. If the list is too long, the first several will be shown.
 *
 * This should typically go into some sort of dialog to show the user.
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { diffDate } from 'common-sk/modules/human';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { truncateWithEllipses } from '../common';
import { baseRepoURL } from '../settings';

const maxCommitsToDisplay = 15;

const template = (ele) => html`
<h2>Commits:</h2>
<table>
  ${ele._commits.slice(0, maxCommitsToDisplay).map(commitRow)}
</table>
<div>
  ${ele._commits.length > maxCommitsToDisplay ? '...and other commits.' : ''}
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

const commitHref = (commit) => {
  // TODO(kjlubick) deduplicate with by-blame-sk
  const repo = baseRepoURL();
  if (!repo) {
    throw new DOMException('repo not set in settings');
  }
  if (repo.indexOf('github.com') !== -1) {
    return `${repo}/commit/${commit.hash}`;
  }
  return `${repo}/+show/${commit.hash}`;
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

  /**
   * @prop commits {Array<Object>} the commits to show. The objects should have string fields:
   *   author, message, hash and a field commit_time that is the number of seconds since the epoch.
   *   See frontend.Commit on the server side for more.
   */
  get commits() { return this._commits; }

  set commits(arr) {
    this._commits = arr;
    this._render();
  }
});
