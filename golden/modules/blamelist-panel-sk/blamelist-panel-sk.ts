/**
 * @module module/blamelist-panel-sk
 * @description <h2><code>blamelist-panel-sk</code></h2>
 *
 * A list of commits and authors. If the list is too long, the first several will be shown.
 * The last commit in the list will not be shown, as it is interpreted to be "the last good commit",
 * or the last commit for which Gold had data before the first commit in the list.
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

const commitRow = (c: Commit) => html`
<tr>
  <td title=${c.author}>${truncateWithEllipses(c.author, 20)}</td>
  <td title=${new Date(c.commit_time * 1000)}>
   ${diffDate(c.commit_time * 1000)}
  </td>
  <td>
    <a href=${commitHref(c)} target=_blank rel=noopener>
      ${c.hash?.substring(0, 8)}
    </a>
  </td>
  <td title=${c.message}>${truncateWithEllipses(c.message || '', 80)}</td>
</tr>
`;

// lastGoodCommit is the last commit that Gold had data before newest commit. When we create the
// range below, the next commit in the repo's history after the oldest commit will be the first to
// show up. We need to do this because Gold removes commits that have no data (to make the data
// "dense") and we don't want to construct a blamelist that is missing commits.
const commitRange = (commits: Commit[], lastGoodCommit: Commit) => {
  if (!commits.length) {
    return '';
  }
  // If we have CL "commit", it is not in the repo we are going to link to (at least not reachable
  // by CL id). Therefore, we skip a CL "commit" when constructing the blamelist. "commit" here is
  // in quotes because we re-use the Commit structure to pass in information about a CL for the
  // purpose of drawing a trace.
  let newestCommit = commits[0];
  if (newestCommit.cl_url) {
    newestCommit = commits[1];
  }

  // If we have exactly one commit or one commit and one CL "commit", we can't show a range.
  if (!newestCommit || lastGoodCommit.hash === newestCommit.hash) {
    return '';
  }

  const repo = baseRepoURL();
  if (!repo) {
    throw new DOMException('repo not set in settings');
  }
  if (repo.indexOf('github.com') !== -1) {
    return `${repo}/compare/${lastGoodCommit.hash}...${newestCommit.hash}`;
  }
  return `${repo}/+log/${lastGoodCommit.hash}..${newestCommit.hash}`;
};

const commitHref = (commit: Commit) => {
  if (commit.cl_url) {
    return commit.cl_url;
  }
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
  readonly hash: string; // For CLs, this is the CL ID.
  readonly author: string;
  readonly message: string;
  readonly commit_time: number;
  readonly cl_url: string;
}

export class BlamelistPanelSk extends ElementSk {
  static template = (ele: BlamelistPanelSk) => {
    const commitRangeHref = commitRange(ele.commits, ele._lastGoodCommit);
    return html`
<h2 ?hidden=${!commitRangeHref} class=full_range>
    <a href=${commitRangeHref} target=_blank rel=noopener>View Full Range</a>
</h2>

<h2>Commits for which Gold saw data:</h2>
<table>
  ${ele.commits.slice(0, maxCommitsToDisplay).map(commitRow)}
</table>
<div>
  ${ele.commits.length > maxCommitsToDisplay ? '...and other commits.' : ''}
</div>`;
  };

  private _commits: Commit[] = [];
  private _lastGoodCommit: Commit = {
    hash: '', author: '', message: '', commit_time: 0, cl_url: ''
  };

  constructor() {
    super(BlamelistPanelSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  get commits(): Commit[] { return this._commits; }

  set commits(commits: Commit[]) {
    // This can happen if clicking on the oldest commit.
    if (commits.length === 1) {
      this._commits = commits;
      this._lastGoodCommit = commits[0];
    } else {
      // Slice off the last good commit.
      this._lastGoodCommit = commits.pop() || this._lastGoodCommit;
      this._commits = commits;
    }

    this._render();
  }
}

define('blamelist-panel-sk', BlamelistPanelSk);
