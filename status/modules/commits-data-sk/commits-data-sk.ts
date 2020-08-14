/**
 * @module modules/commits-data-sk
 * @description An element that manages fetching and processing commits data for Status.
 */

import { $$ } from 'common-sk/modules/dom';
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { errorMessage } from 'elements-sk/errorMessage';
import { truncateWithEllipses } from '../../../golden/modules/common';


import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import {
    IncrementalCommitsResponse, Task, Comment, IncrementalUpdate,
    Branch, LongCommit, IncrementalCommitsRequest, ResponseMetadata
} from '../rpc/statusFe'
import 'elements-sk/select-sk';

const VALID_TASK_SPEC_CATEGORIES = ["Build", "Housekeeper", "Infra", "Perf", "Test", "Upload"];

const FILTER_ALL = "all";
const FILTER_DEFAULT = "default";
const FILTER_INTERESTING = "interesting";
const FILTER_FAILURES = "failures";
const FILTER_FAIL_NO_COMMENT = "nocomment";
const FILTER_COMMENTS = "comments";
const FILTER_SEARCH = "search";

const TASK_STATUS_PENDING = "";
const TASK_STATUS_RUNNING = "RUNNING";
const TASK_STATUS_SUCCESS = "SUCCESS";
const TASK_STATUS_FAILURE = "FAILURE";
const TASK_STATUS_MISHAP = "MISHAP";

const TIME_POINTS = [
    {
        label: "-1h",
        offset: 60 * 60 * 1000,
    },
    {
        label: "-3h",
        offset: 3 * 60 * 60 * 1000,
    },
    {
        label: "-1d",
        offset: 24 * 60 * 60 * 1000,
    },
];

const template = () => html`
<div class=tr-container>
  
</div>
`;

type CommitHash = string;
type TaskSpec = string;
type TaskId = string;


export interface Commit extends LongCommit {
  shortAuthor: string;
  shortHash: string;
  shortSubject: string;
  issue: string;
  patchStorage: string;
  isRevert: boolean;
  isReland: boolean;
}

export class CommitsDataSk extends ElementSk {
  // Ordered commits, I think.
  commits: Array<Commit> = [];
  commitsByHash: Map<CommitHash, Commit> = new Map();
  
  branch_heads: Array<Branch> = [];
  swarming_url: string | null = null;
  task_scheduler_url: string | null = null;
  // taskId -> Task
  tasks: Map<TaskId, Task> = new Map();
  // taskName -> (taskId -> Task)
  tasksBySpec: Map<TaskSpec, Map<TaskId, Task>> = new Map();
  // commitHash -> (taskName -> Task)
  tasksByCommit: Map<CommitHash, Map<TaskSpec, Task>> = new Map(); // task_details?
  // commitHash -> (taskName -> Comment)
  comments: Map<CommitHash, Map<TaskSpec, Comment>> = new Map();
  // commitHash -> Commit that reverted it
  reverted_map: Map<CommitHash, Commit> = new Map();
  // commitHash -> Commit that relanded it
  relanded_map: Map<CommitHash, Commit> = new Map();

  // todo categories, task specs, etc

  task_details: object | null = null;
  task_specs: object | null = null;
  categories: object | null = null;
  category_list: Array<string> = [];
  commits_map: object | null = null;
  logged_in: boolean | null = null;
  repo: string | null = null;
  repo_base: string | null = null;

  // TODO
  serverPodId: string | null = 'ashfadshaqafda';
  data: IncrementalCommitsResponse | null = null;



  private static template = (ele: CommitsDataSk) => html`<div>Hello World!</div>`;
  constructor() {
      super(CommitsDataSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
    const repo = "skia";
    const count = "30";
    let url = `/json/{repo}/incremental?n={count}`;
    url += `&pod={this.serverPodId}`;
    fetch('/json/skia/incremental?n=10', { method: 'GET' })
      .then(jsonOrThrow)
      .then((json: IncrementalCommitsResponse) => {
        const update: IncrementalUpdate = json.update;

        const startOver: boolean = update.startover;
        this.commits = update.commits as Array<Commit>;
        this.branch_heads = update.branchheads || this.branch_heads;
        this.swarming_url = update.swarmingurl || this.swarming_url;
        this.task_scheduler_url = update.taskschedulerurl || this.task_scheduler_url;

        // Map commits by hash.
        this.commits.forEach((commit: Commit) => {
          this.commitsByHash.set(commit.hash, commit)
        })

        // Map task Id to Task
        if (update.tasks) {
          for (let i = 0; i < update.tasks.length; i++) {
            const task = update.tasks[i];
            this.tasks.set(task.id, task);
          }
        }

        // TODO: remove old commits?

        // Map commits to tasks
        for (let [, task] of this.tasks) {
          if (task.commits) {
            for (let commit of task.commits) {
              let tasksForCommit = this.tasksByCommit.get(commit);
              if (!tasksForCommit) {
                tasksForCommit = new Map();
                this.tasksByCommit.set(commit, tasksForCommit);
              }
              tasksForCommit.set(task.name, task);
            }
          }
        }


        // Map comments.
        for (let comment of update.comments) {
          comment.taskspecname = comment.taskspecname || "";
          comment.commithash = comment.commithash || "";
          let commentsForCommit = this.comments.get(comment.commithash);
          if (!commentsForCommit) {
            commentsForCommit = new Map();
            this.comments.set(comment.commithash, commentsForCommit);
          }
          commentsForCommit.set(comment.taskspecname, comment);
        }

        // Process commits.
        for (let commit of this.commits) {
          commit.shortAuthor = shortAuthor(commit.author);
          commit.shortHash = shortCommit(commit.hash);
          commit.shortSubject = shortSubject(commit.subject);
          
          [commit.issue, commit.patchStorage] = findIssueAndReviewTool(commit);

          commit.isRevert = false;
          var reverted = findRevertedCommit(this.commitsByHash, commit);
          if (reverted) {
            commit.isRevert = true;
            this.reverted_map.set(reverted.hash,commit);
          }

          commit.isReland = false;
          var relanded = findRelandedCommit(this.commitsByHash, commit);
          if (relanded) {
            commit.isReland = true;
            this.relanded_map.set(relanded.hash, commit);
          }
        }
      })
      .catch(errorMessage);;
  }
};

define('commits-data-sk', CommitsDataSk);

// shortCommit returns the first 7 characters of a commit hash.
function shortCommit(commit: string): string {
    return commit.substring(0, 7);
}

// shortAuthor shortens the commit author field by returning the
// parenthesized email address if it exists. If it does not exist, the
// entire author field is used.
function shortAuthor(author: string): string {
    const re: RegExp = /.*\((.+)\)/;
    const match = re.exec(author);
    let res = author;
    if (match) {
        res = match[1];
    }
    return res.split("@")[0];
}

// shortSubject truncates a commit subject line to 72 characters if needed.
// If the text was shortened, the last three characters are replaced by
// ellipsis.
function shortSubject(subject: string): string {
    return truncateWithEllipses(subject, 72);
}

// findIssueAndReviewTool returns [issue, patchStorage]. patchStorage will
// be either Gerrit or empty, and issue will be the CL number or empty.
// If an issue cannot be determined then an empty string is returned for
// both issue and patchStorage.
function findIssueAndReviewTool(commit: LongCommit): [string, string] {
  // See if it is a Gerrit CL.
  var gerritRE = /(.|[\r\n])*Reviewed-on:.*\/([0-9]*)/g;
  var gerritTokens = gerritRE.exec(commit.body);
  if (gerritTokens) {
    return [gerritTokens[gerritTokens.length - 1], 'gerrit'];
  }
  // Could not find a CL number return an empty string.
  return ['', ''];
}

// Find and return the commit which was reverted by the given commit.
function findRevertedCommit(commits: Map<string, Commit>, commit: Commit) {
  const patt = new RegExp("^This reverts commit ([a-f0-9]+)");
  const tokens = patt.exec(commit.body);
  if (tokens) {
    return commits.get(tokens[tokens.length - 1]);
  }
  return null;
}

// Find and return the commit which was relanded by the given commit.
function findRelandedCommit(commits: Map<string, Commit>, commit: Commit) {
  // Relands can take one of two formats. The first is a "direct" reland.
  const patt = new RegExp("^This is a reland of ([a-f0-9]+)");
  const tokens = patt.exec(commit.body) as RegExpExecArray;
  if (tokens) {
    return commits.get(tokens[tokens.length - 1]);
  }

  // The second is a revert of a revert.
  var revert = findRevertedCommit(commits, commit);
  if (revert) {
    return findRevertedCommit(commits, revert);
  }
  return null;
}