/**
 * @module modules/commits-data-sk
 * @description An element that manages fetching and processing commits data for Status.
 */

import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { errorMessage } from 'elements-sk/errorMessage';
import { truncateWithEllipses } from '../../../golden/modules/common';

import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import {
  GetIncrementalCommitsResponse,
  Task,
  Comment,
  IncrementalUpdate,
  Branch,
  LongCommit,
  GetStatusService,
  StatusService,
} from '../rpc';
import 'elements-sk/select-sk';
import { defaultRepo } from '../settings';

const VALID_TASK_SPEC_CATEGORIES = ['Build', 'Housekeeper', 'Infra', 'Perf', 'Test', 'Upload'];

const FILTER_ALL = 'all';
const FILTER_DEFAULT = 'default';
const FILTER_INTERESTING = 'interesting';
const FILTER_FAILURES = 'failures';
const FILTER_FAIL_NO_COMMENT = 'nocomment';
const FILTER_COMMENTS = 'comments';
const FILTER_SEARCH = 'search';

const TASK_STATUS_PENDING = '';
const TASK_STATUS_RUNNING = 'RUNNING';
const TASK_STATUS_SUCCESS = 'SUCCESS';
const TASK_STATUS_FAILURE = 'FAILURE';
const TASK_STATUS_MISHAP = 'MISHAP';

const TIME_POINTS = [
  {
    label: '-1h',
    offset: 60 * 60 * 1000,
  },
  {
    label: '-3h',
    offset: 3 * 60 * 60 * 1000,
  },
  {
    label: '-1d',
    offset: 24 * 60 * 60 * 1000,
  },
];

export type CommitHash = string;
export type TaskSpec = string;
export type TaskId = string;

export interface Commit extends LongCommit {
  shortAuthor: string;
  shortHash: string;
  shortSubject: string;
  issue: string;
  patchStorage: string;
  isRevert: boolean;
  isReland: boolean;
  ignoreFailure: boolean;
}

export class CategorySpec {
  taskSpecsBySubCategory: Map<string, Array<TaskSpec>> = new Map();
  colspan: number = 0;
}

export class TaskSpecDetails {
  name: string = '';
  comments: Array<Comment> = [];
  category: string = '';
  subcategory: string = '';
  flaky: boolean = false;
  ignoreFailure: boolean = false;
  // Metadata about the set of tasks in this spec.
  hasSuccess = false;
  hasFailure = false;
  hasTaskComment = false;

  interesting(): boolean {
    return this.hasSuccess && this.hasFailure && !this.ignoreFailure;
  }
  hasFailing(): boolean {
    return this.hasFailure;
  }
  hasFailingNoComment(): boolean {
    return this.hasFailure && (!this.comments || this.comments.length == 0);
  }
  hasComment(): boolean {
    return this.hasTaskComment || (this.comments && this.comments.length > 0);
  }
}

export class CommitsDataSk extends ElementSk {
  // Ordered commits.
  commits: Array<Commit> = [];
  commitsByHash: Map<CommitHash, Commit> = new Map();

  branchHeads: Array<Branch> = [];
  tasks: Map<TaskId, Task> = new Map();
  tasksBySpec: Map<TaskSpec, Map<TaskId, Task>> = new Map();
  tasksByCommit: Map<CommitHash, Map<TaskSpec, Task>> = new Map();
  comments: Map<CommitHash, Map<TaskSpec, Array<Comment>>> = new Map();
  revertedMap: Map<CommitHash, Commit> = new Map();
  relandedMap: Map<CommitHash, Commit> = new Map();

  taskSpecs: Map<TaskSpec, TaskSpecDetails> = new Map();
  categories: Map<string, CategorySpec> = new Map();

  private serverPodId: string = '';
  private client: StatusService = GetStatusService();

  // Private members because we decorate their setters to refetch data.
  private _loggedIn: boolean | null = null;
  private _repo: string = defaultRepo();
  private _numCommits: number = 35;

  constructor() {
    super(() => html``);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();

    this.dispatchEvent(new CustomEvent('begin-task', { bubbles: true }));
    // TODO(westont): Change the proto back to string type for the backend, and use real times,
    // rather than always loading everything with 0 'from' value.
    this.client
      .getIncrementalCommits({
        // Twirp Typescript module doesn't correctly convey the imported timestamp proto type,
        // and just makes these a string, which the backend chokes on.
        // from: new Date(0).toUTCString(),
        // to: new Date(0).toUTCString(),
        n: this._numCommits,
        pod: this.serverPodId,
        // TODO(westont): repo picker.
        repoPath: this._repo,
      })
      .then((json: GetIncrementalCommitsResponse) => {
        // TODO(westont): Use StartOver to know to clear vs update, and actual implement updating.
        const startOver: boolean = json.metadata!.startOver;
        this.serverPodId = json.metadata!.pod;
        this.extractData(json.update!);
        this.processCommits();
      })
      .catch(errorMessage)
      .finally(() => this.dispatchEvent(new CustomEvent('end-task', { bubbles: true })));
  }

  /**
   * extractData takes data from GetIncrementalCommits and adds useful structure, mapping commits
   * by hash, tasks by Id, commits to tasks, and comments by hash and taskSpec.
   * @param update Data from the backend.
   */
  private extractData(update: IncrementalUpdate) {
    this.commits = update.commits as Array<Commit>;
    this.branchHeads = update.branchHeads || this.branchHeads;

    // Map commits by hash.
    this.commits.forEach((commit: Commit) => {
      this.commitsByHash.set(commit.hash, commit);
    });

    // Map task Id to Task
    for (const task of update.tasks || []) {
      this.tasks.set(task.id, task);
    }

    // TODO(westont): Remove old commits.

    // Map commits to tasks
    for (const [, task] of this.tasks) {
      if (task.commits) {
        for (let commit of task.commits) {
          let tasksForCommit = this.tasksByCommit.get(commit);
          if (!tasksForCommit) {
            tasksForCommit = new Map();
            this.tasksByCommit.set(commit, tasksForCommit);
          }
          tasksForCommit.set(task.name, task);
          // TODO(westont): Remove deleted comments.
        }
      }
    }

    // Map comments.
    for (let comment of update.comments || []) {
      comment.taskSpecName = comment.taskSpecName || '';
      comment.commit = comment.commit || '';
      const commentsBySpec = lookupOrInsert<string, Map<TaskSpec, Array<Comment>>>(
        this.comments,
        comment.commit,
        Map
      );
      const comments = lookupOrInsert<TaskSpec, Array<Comment>>(
        commentsBySpec,
        comment.taskSpecName,
        Array
      );
      comments.push(comment);
      // Keep comments sorted by timestamp, if there are multiple.
      comments.sort((a: Comment, b: Comment) => Number(a.timestamp) - Number(b.timestamp));
    }
  }

  /**
   * processCommits adds metadata to commit objects, maps reverts and relands, and gathers
   * taskspecs references by commits.
   */
  private processCommits() {
    for (let commit of this.commits) {
      // Metadata for display/links.
      commit.shortAuthor = shortAuthor(commit.author);
      commit.shortHash = shortCommit(commit.hash);
      commit.shortSubject = shortSubject(commit.subject);
      [commit.issue, commit.patchStorage] = findIssueAndReviewTool(commit);

      this.mapRevertsAndRelands(commit);

      // Check for commit-specific comments with ignoreFailure.
      const commitComments = this.comments.get(commit.hash)?.get('');
      if (
        commitComments &&
        commitComments.length &&
        commitComments[commitComments.length - 1].ignoreFailure
      ) {
        commit.ignoreFailure = true;
      }

      const commitTasks = this.tasksByCommit.get(commit.hash) || [];
      this.processCommitTasks(commitTasks, commit);
      // TODO(westont): Branch tags and time offset tags.
    }
  }

  private processCommitTasks(commitTasks: Map<string, Task> | never[], commit: Commit) {
    for (const [taskSpec, task] of commitTasks) {
      const details = lookupOrInsert(this.taskSpecs, taskSpec, TaskSpecDetails);
      // First time seeing the taskSpec, fill in header data.
      if (!details.name) {
        this.fillTaskSpecDetails(details, taskSpec);
      }
      // Aggregate data about this spec's tasks.
      details.hasSuccess = details.hasSuccess || task.status == TASK_STATUS_SUCCESS;
      // Only count failures we aren't ignoring.
      details.hasFailure =
        details.hasFailure ||
        (!commit.ignoreFailure &&
          (task.status == TASK_STATUS_FAILURE || task.status == TASK_STATUS_MISHAP));
      details.hasTaskComment =
        details.hasTaskComment || (this.comments.get(commit.hash)?.get(taskSpec)?.length || 0) > 0;
      // TODO(westont): Track purple tasks.
    }
  }

  private fillTaskSpecDetails(details: TaskSpecDetails, taskSpec: string) {
    details.name = taskSpec;
    const comments = this.comments.get('')?.get(taskSpec) || [];
    details.comments = comments;

    const split = taskSpec.split('-');
    if (split.length >= 2 && VALID_TASK_SPEC_CATEGORIES.indexOf(split[0]) != -1) {
      details.category = split[0];
      details.subcategory = split[1];
    }
    if (comments.length > 0) {
      details.flaky = comments[comments.length - 1].flaky;
      details.ignoreFailure = comments[comments.length - 1].ignoreFailure;
    }

    const category = details.category || 'Other';
    const categoryDetails = lookupOrInsert(this.categories, category, CategorySpec);
    //this.categoryList.add(category);
    const subcategory = details.subcategory || 'Other';
    lookupOrInsert<string, Array<string>>(
      categoryDetails.taskSpecsBySubCategory,
      subcategory,
      Array
    ).push(taskSpec);
    categoryDetails.colspan++;
  }

  private mapRevertsAndRelands(commit: Commit) {
    commit.isRevert = false;
    var reverted = findRevertedCommit(this.commitsByHash, commit);
    if (reverted) {
      commit.isRevert = true;
      this.revertedMap.set(reverted.hash, commit);
      reverted.ignoreFailure = true;
    }
    commit.isReland = false;
    var relanded = findRelandedCommit(this.commitsByHash, commit);
    if (relanded) {
      commit.isReland = true;
      this.relandedMap.set(relanded.hash, commit);
    }
  }
}

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
  return res.split('@')[0];
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
  const patt = new RegExp('^This reverts commit ([a-f0-9]+)');
  const tokens = patt.exec(commit.body);
  if (tokens) {
    return commits.get(tokens[tokens.length - 1]);
  }
  return null;
}

// Find and return the commit which was relanded by the given commit.
function findRelandedCommit(commits: Map<string, Commit>, commit: Commit) {
  // Relands can take one of two formats. The first is a "direct" reland.
  const patt = new RegExp('^This is a reland of ([a-f0-9]+)');
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

// Helper to get the value associated with a key, but default construct and
// insert it first if not present.  Passing the type as a second arg is
// necessary since types are erased when transcribed to JS.
// Usage:
// const mymap: Map<string, Array<string>> = new Map();
// lookupOrInsert(mymap, 'foo', Array).push('bar')
function lookupOrInsert<K, V>(map: Map<K, V>, key: K, valuetype: { new (): V }): V {
  let maybeValue = map.get(key);
  if (!maybeValue) {
    maybeValue = new valuetype();
    map.set(key, maybeValue);
  }
  return maybeValue;
}
