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

const template = (ele: BlamelistPanelSk) => html`
<h2>Commits:</h2>
<table>
  ${ele.commits.slice(0, maxCommitsToDisplay).map(commitRow)}
</table>
<div>
  ${ele.commits.length > maxCommitsToDisplay ? '...and other commits.' : ''}
</div>`;

const commitRow = (c: Commit) => html`
<tr>
  <td title=${c.author}>${truncateWithEllipses(c.author, 20)}</td>
  <td title=${new Date(c.commit_time * 1000)}>
   ${diffDate(c.commit_time * 1000)}
  </td>
  <td><a href=${commitHref(c)}>${c.hash && c.hash.substring(0, 8)}</a></td>
  <td title=${c.message}>${truncateWithEllipses(c.message || '', 80)}</td>
</tr>
`;

const commitHref = (commit: Commit) => {
  // TODO(kjlubick): Deduplicate with by-blame-sk.
  const repo = baseRepoURL();
  if (!repo) {
    throw new DOMException('repo not set in settings');
  }
  if (repo.indexOf('github.com') !== -1) {
    return `${repo}/commit/${commit.hash}`;
  }
  return `${repo}/+show/${commit.hash}`;
};

/**
 * Represents a Git commit.
 * 
 * Client-side equivalent of frontend.Commit Go type.
 */
export interface Commit {
  readonly hash: string;
  readonly author: string;
  readonly message: string;
  readonly commit_time: number;
};

export class BlamelistPanelSk extends ElementSk {
  private _commits: Commit[] = [];

  constructor() {
    super(template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  get commits(): Commit[] { return this._commits; }

  set commits(commits: Commit[]) {
    this._commits = commits;
    this._render();
  }
};

define('blamelist-panel-sk', BlamelistPanelSk);
