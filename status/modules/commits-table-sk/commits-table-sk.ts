/**
 * @module modules/commits-table-sk
 * @description An element that displays task and commit data for Status.
 *
 * @property displayCommitSubject - Render truncated commit subjects, rather than authors in the
 * table.
 * @property filter - One of 'interesting', 'all', 'failure', 'nocomment', 'comments, or
 * 'search'. To filter taskSpecs displayed.
 * @property search - a regex string with which to filter taskSpecs against for
 * display. Only used if filter == 'search'.
 *
 * @event repo-changed - Occurs when user selects a repo. Event has {detail: '<new repo>'}
 */

import { $, $$, DomReady } from 'common-sk/modules/dom';
import { fromObject } from 'common-sk/modules/query';
import { stateReflector } from 'common-sk/modules/stateReflector';
import { HintableObject } from 'common-sk/modules/hintable';
import { define } from 'elements-sk/define';
import { html, TemplateResult } from 'lit-html';
import { styleMap } from 'lit-html/directives/style-map';
import { classMap } from 'lit-html/directives/class-map';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

import 'elements-sk/radio-sk';
import 'elements-sk/tabs-sk';
import 'elements-sk/select-sk';
import 'elements-sk/icon/add-icon-sk';
import 'elements-sk/icon/autorenew-icon-sk';
import 'elements-sk/icon/block-icon-sk';
import 'elements-sk/icon/comment-icon-sk';
import 'elements-sk/icon/help-icon-sk';
import 'elements-sk/icon/redo-icon-sk';
import 'elements-sk/icon/texture-icon-sk';
import 'elements-sk/icon/undo-icon-sk';
import 'elements-sk/styles/select';
import '../branches-sk';
import '../details-dialog-sk';
import {
  Branch,
  Comment,
  GetIncrementalCommitsRequest,
  GetIncrementalCommitsResponse,
  IncrementalUpdate,
  LongCommit,
  StatusService,
  Task,
} from '../rpc/status';
import { DetailsDialogSk } from '../details-dialog-sk/details-dialog-sk';
import { errorMessage } from 'elements-sk/errorMessage';
import { truncateWithEllipses } from '../../../golden/modules/common';
import { GetStatusService } from '../rpc';
import { BranchesSk } from '../branches-sk/branches-sk';
import { defaultRepo, repos, taskSchedulerUrl } from '../settings';

const CONTROL_START_ROW = 1;
const CATEGORY_START_ROW = CONTROL_START_ROW + 1;
const SUBCATEGORY_START_ROW = CATEGORY_START_ROW + 1;
const TASKSPEC_START_ROW = SUBCATEGORY_START_ROW + 1;
const COMMIT_START_ROW = TASKSPEC_START_ROW + 1;

const BRANCH_START_COL = 1;
const COMMIT_START_COL = BRANCH_START_COL + 1;
const TASK_START_COL = COMMIT_START_COL + 1;

const REVERT_HIGHLIGHT_CLASS = 'highlight-revert';
const RELAND_HIGHLIGHT_CLASS = 'highlight-reland';
const VALID_TASK_SPEC_CATEGORIES = ['Build', 'Housekeeper', 'Infra', 'Perf', 'Test', 'Upload'];

const TASK_STATUS_SUCCESS = 'SUCCESS';
const TASK_STATUS_FAILURE = 'FAILURE';
const TASK_STATUS_MISHAP = 'MISHAP';

// Makes some maps more self-documenting.
export type CommitHash = string;
export type TaskSpec = string;
export type TaskId = string;

// Commit with added metadata we compute that aid in displaying and associating it with other data.
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

// Describes the subcategories and taskspecs within a category.
export class CategorySpec {
  taskSpecsBySubCategory: Map<string, Array<TaskSpec>> = new Map();
  // Sum of the above taskspec array value lengths.
  colspan: number = 0;
}

// Generated data to describe a taskspec and the tasks within it.
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

type Filter = 'Interesting' | 'Failures' | 'All' | 'Nocomment' | 'Comments' | 'Search';

interface FilterInfo {
  text: string;
  title: string;
}

const FILTER_INFO: Map<Filter, FilterInfo> = new Map([
  [
    'Interesting',
    {
      text: 'Interesting',
      title: 'Tasks which have both successes and failures within the visible commit window.',
    },
  ],
  [
    'Failures',
    {
      text: 'Failures',
      title: 'Tasks which have failures within the visible commit window.',
    },
  ],
  [
    'Comments',
    {
      text: 'Comments',
      title: 'Tasks which have comments.',
    },
  ],
  [
    'Nocomment',
    {
      text: 'Failing w/o comment',
      title: 'Tasks which have failures within the visible commit window but have no comments.',
    },
  ],
  [
    'All',
    {
      text: 'All',
      title: 'Display all tasks.',
    },
  ],
  [
    'Search',
    {
      text: '_',
      title:
        'Enter a search string. Substrings and regular expressions may be used, per the Javascript String match() rules.',
    },
  ],
]);
// Used to translate tabs-sk event indices to Filters.
const FILTER_INDEX = Array.from(FILTER_INFO).map(([filter, _]) => filter);

// An internal class used to keep the fetching and preprocessing of data untangled from the logic
// to render the table itself.
class Data {
  // Outputs - Data to be used by the CommitsTableSk class. Derived from calling the
  // GetIncrementalCommits API.
  commits: Array<Commit> = []; // Commits in reverse chronoligcal order.
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
  // Internal state.
  private serverPodId: string = '';
  private client: StatusService = GetStatusService();
  // Used to detect changes of these values between calls, to know when to load from scratch.
  private repo: string = '';
  private numCommits: number = -1;

  update(repo: string, numCommits: number, lastLoaded?: Date) {
    const req: GetIncrementalCommitsRequest = {
      n: numCommits,
      pod: this.serverPodId,
      repoPath: repo,
    };
    if (lastLoaded && repo === this.repo && numCommits === this.numCommits) {
      // We incrementally update if this is the same repo and numCommits as the
      // previous call, and we have a starting point.
      req.from = lastLoaded.toISOString();
    }
    this.repo = repo;
    this.numCommits = numCommits;
    return this.client
      .getIncrementalCommits(req)
      .then((json: GetIncrementalCommitsResponse) => {
        if (json.metadata!.startOver) {
          this.clearData();
        }
        this.serverPodId = json.metadata!.pod;
        this.extractData(json.update!);
        // We clear this derived data, as it may have changed with incremental updates.
        this.taskSpecs = new Map();
        this.categories = new Map();

        this.processCommits();
      })
      .catch(errorMessage);
  }

  private clearData() {
    this.commits = [];
    this.commitsByHash = new Map();
    this.branchHeads = [];
    this.tasks = new Map();
    this.tasksBySpec = new Map();
    this.tasksByCommit = new Map();
    this.comments = new Map();
    this.revertedMap = new Map();
    this.relandedMap = new Map();
    this.taskSpecs = new Map();
    this.categories = new Map();
  }

  /**
   * extractData takes data from GetIncrementalCommits and adds useful structure, mapping commits
   * by hash, tasks by Id, commits to tasks, and comments by hash and taskSpec.
   * @param update Data from the backend.
   */
  private extractData(update: IncrementalUpdate) {
    const newCommits = ((update.commits || []) as Array<Commit>).filter(
      // In a pathological case, a commit that the backend becomes aware of between when the client
      // calculates 'from' and when the backend gets the client's request, could end up being sent
      // twice. Dedup it.
      (commit) => !this.commitsByHash.has(commit.hash)
    );
    const sliceIdx = this.numCommits - newCommits.length;
    const keep = this.commits.slice(0, sliceIdx);
    const remove = this.commits.slice(sliceIdx, this.commits.length);
    this.commits = newCommits.concat(keep);
    this.validateCommits();

    if (update.branchHeads && update.branchHeads.length > 0) {
      this.branchHeads = update.branchHeads;
    }

    // Map commits by hash.
    this.commits.forEach((commit: Commit) => {
      this.commitsByHash.set(commit.hash, commit);
    });

    // Map task Id to Task
    for (const task of update.tasks || []) {
      this.tasks.set(task.id, task);
    }

    // Remove too-old tasks.
    for (let commit of remove) {
      this.tasksByCommit.delete(commit.hash);
      for (let [id, task] of this.tasks) {
        if (task.revision == commit.hash) {
          this.tasks.delete(id);
        }
      }
    }

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
        }
      }
    }

    // TODO(westont): Remove deleted comments. This is broken at the backend already delete
    // comments are only deleted in incremental updates if there is another comment in the
    // same category(e.g. A commit comment won't be recognized as deleted unless another
    // commit comment exists somewhere, to populate the commit_comments update field.)
    // For now this just means deleted comments aren't conveyed to clients until they or the
    // backend forces a full update.

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

  // Returns true if c is a child of possibleAncestor.
  private childOf(c: Commit | undefined, possibleAncestor: Commit) {
    let curr = c;
    while (curr) {
      if (curr.parents!.includes(possibleAncestor.hash)) {
        return true;
      }
      const parentHash = curr.parents!.length > 0 ? curr.parents![0] : '';
      curr = this.commitsByHash.get(parentHash);
    }
    return false;
  }

  private validateCommits() {
    this.commits.sort((a, b) => {
      const diff = new Date(b.timestamp!).valueOf() - new Date(a.timestamp!).valueOf();
      if (diff !== 0) {
        return diff;
      }
      // Timestamps are the same, attempt to sort by lineage.
      if (this.childOf(a, b)) {
        return -1;
      }
      return 1;
    });
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

/**
 * RequestLimiter is a helper class that manages keeping a single async call live at once.
 * Async calls should only be triggered if a call to beginUpdate returns true.  After async calls
 * resolve, call endUpdate, if it returns true, one or more calls to beginUpdate occured before
 * the initial call resolved, these can honored by retriggering the async call.
 */
class RequestLimiter {
  private awaitingResponse: boolean = false;
  private updateRequested: boolean = true;

  // beginUpdate returns true if a request should be sent.
  beginUpdate(): boolean {
    if (this.awaitingResponse) {
      this.updateRequested = true;
      return false;
    }
    this.awaitingResponse = true;
    return true;
  }

  // endUpdate returns true if multiple beginUpdate calls have occured consecutively (without
  // paired finishUpdate calls).
  endUpdate(): boolean {
    this.awaitingResponse = false;
    if (this.updateRequested) {
      this.updateRequested = false;
      return true;
    }
    return false;
  }
}

class State {
  filter: Filter = 'Interesting';
  search: string = '';
  displayCommitSubject: boolean = false;
  repo: string = defaultRepo();
}

export class CommitsTableSk extends ElementSk {
  private _displayCommitSubject: boolean = false;
  private _filter: Filter = 'Interesting';
  private _search: string = '';
  private lastLoaded?: Date;
  private lastColumn: number = 1;
  private mishapTasks: Array<Task> = [];
  private refreshHandle?: number;
  private requestLimiter: RequestLimiter = new RequestLimiter();
  private stateHasChanged: () => void = () => {};

  private data: Data = new Data();

  private static template = (el: CommitsTableSk) => html`<div class="commitsTableContainer">
    <div
      class="legend"
      style=${el.gridLocation(CATEGORY_START_ROW, COMMIT_START_COL, COMMIT_START_ROW)}
    >
      <comment-icon-sk class="tiny"></comment-icon-sk>Comments<br />
      <texture-icon-sk class="tiny"></texture-icon-sk>Flaky<br />
      <block-icon-sk class="tiny"></block-icon-sk>Ignore Failure<br />
      <undo-icon-sk class="tiny fill-red"></undo-icon-sk>Revert<br />
      <redo-icon-sk class="tiny fill-green"></redo-icon-sk>Reland<br />
    </div>
    <div class="tasksTable">${el.fillTableTemplate()}</div>
    <div
      class="reloadControls"
      style=${el.gridLocation(CONTROL_START_ROW, BRANCH_START_COL, COMMIT_START_ROW)}
    >
      <div id="repoContainer">
        <div id="repoLabel">Repo:</div>
        <select
          id="repoSelector"
          @change=${(e: Event) => {
            el.dispatchEvent(
              new CustomEvent('repo-changed', { bubbles: true, detail: (e.target as any).value })
            );
            el.stateHasChanged();
            el.update();
          }}
        >
          ${repos().map((r) => html`<option value=${r}>${r}</option>`)}
        </select>
      </div>
      <div class="refresh">
        <input-sk
          type="number"
          textPrefix="Reload (s):&nbsp"
          id="reloadInput"
          @change=${() => el.update()}
        ></input-sk>
        <input-sk
          type="number"
          textPrefix="Commits:&nbsp&nbsp&nbsp"
          id="commitsInput"
          @change=${() => el.update()}
        >
        </input-sk>
        <div class="lastLoaded">
          ${el.lastLoaded ? `Loaded ${el.lastLoaded.toLocaleTimeString()}` : '(Not yet loaded)'}
        </div>
      </div>
    </div>
    <branches-sk
      style=${el.gridLocation(
        COMMIT_START_ROW,
        BRANCH_START_COL,
        COMMIT_START_ROW + el.data.commits.length
      )}
    ></branches-sk>
    <div
      class="controls"
      style=${el.gridLocation(
        CONTROL_START_ROW,
        COMMIT_START_COL,
        CONTROL_START_ROW + 1,
        // We render this after the table so we know our last column.
        el.lastColumn
      )}
    >
      <div class="horizontal">
        <div class="commitLabelSelector">
          ${['Author', 'Subject'].map(
            (label, i) => html` <radio-sk
              class="tiny"
              label=${label}
              name="commitLabel"
              ?checked=${!!i === el.displayCommitSubject}
              @change=${el.toggleCommitLabel}
            ></radio-sk>`
          )}
        </div>

        <div class="horizontal">
          <tabs-sk
            @tab-selected-sk=${(e: CustomEvent) => (el.filter = FILTER_INDEX[e.detail.index])}
          >
            ${Array.from(FILTER_INFO).map(([filter, info]) =>
              filter === 'Search'
                ? html``
                : html`<button title=${info.title} class=${el._filter === filter ? 'selected' : ''}>
                    ${info.text}
                    <help-icon-sk class="tiny"></help-icon-sk>
                  </button> `
            )}
          </tabs-sk>
          <input-sk
            id="searchInput"
            class=${el.filter === 'Search' ? 'selected' : ''}
            label="Filter task spec"
            @change=${el.searchFilter}
          >
          </input-sk>
          <a href="${taskSchedulerUrl()}/trigger" target="_blank" rel="noopener">
            <button>
              <add-icon-sk></add-icon-sk>
              Trigger a Job
            </button>
          </a>
          <a href=${el.reRunMishapsUrl()} target="_blank" rel="noopener">
            <button>
              <autorenew-icon-sk></autorenew-icon-sk>
              Re-Run Purple Jobs
            </button>
          </a>
        </div>
      </div>
    </div>

    <details-dialog-sk .repo=${el.repo}></details-dialog-sk>
  </div>`;

  constructor() {
    super(CommitsTableSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    document.addEventListener('click', this.onClick);
    this._render();
    // input-sk value is backed by its <inputs>'s value directly, so set after render.
    (<HTMLInputElement>$$('#reloadInput', this)).value = '60';
    (<HTMLInputElement>$$('#commitsInput', this)).value = '35';
    (<HTMLSelectElement>$$('#repoSelector', this)).value = defaultRepo();

    this.stateHasChanged = stateReflector(
      () => this.getState(),
      (fromUrl) => this.setState(fromUrl)
    );
    // Now that we set the default object, use the real getState.
    this.getState = () => this.getCurrentState();
    // Update the Url with the real values after the stateReflector has applied any settings from
    // the url.
    DomReady.then(() => this.stateHasChanged());

    this.update();
  }

  // We provide this to stateReflector initially, to give it a typed but empty object to create
  // deltas from.  This allows our 'default' values to still be reflected in the url.
  private getState = () => {
    const hintableSettings: HintableObject = {
      filter: '',
      search: '',
      repo: '',
      displayCommitSubject: false,
    };
    return hintableSettings;
  };

  private getCurrentState(): HintableObject {
    const state: State = {
      filter: this.filter,
      search: this.search,
      displayCommitSubject: this.displayCommitSubject,
      repo: $$<HTMLSelectElement>('#repoSelector', this)!.value,
    };
    return (state as unknown) as HintableObject;
  }

  private setState(fromUrl: HintableObject) {
    let state = (fromUrl as unknown) as State;
    // Using empty default values in the default State object (so all values, including our true
    // defaults are reflected in the url) means the initial load will try to set filter and
    // repo to the empty string, prevent this.
    if (state.filter) {
      this._filter = state.filter;
    }
    if (state.repo) {
      $$<HTMLSelectElement>('#repoSelector', this)!.value = state.repo;
    }
    this._search = state.search;
    $$<HTMLInputElement>('#searchInput', this)!.value = this._search;
    this._displayCommitSubject = state.displayCommitSubject;
    this.draw();
  }

  disconnectedCallback() {
    document.removeEventListener('click', this.onClick);
  }

  get displayCommitSubject() {
    return this._displayCommitSubject;
  }

  set displayCommitSubject(v: boolean) {
    this._displayCommitSubject = v;
    this.stateHasChanged();
    $('.commit-text').forEach((el, i) => {
      if (v) {
        el.innerHTML = this.data.commits[i].shortSubject;
        el.setAttribute('title', this.data.commits[i].shortAuthor);
      } else {
        el.innerHTML = this.data.commits[i].shortAuthor;
        el.setAttribute('title', this.data.commits[i].shortSubject);
      }
    });
  }

  get filter(): Filter {
    return this._filter;
  }

  set filter(v: Filter) {
    this._filter = v;
    this.stateHasChanged();
    this.draw();
  }

  get search(): string {
    return this._search;
  }

  set search(v: string) {
    this._search = v;
    this.stateHasChanged();
    this.draw();
  }

  get repo(): string {
    return ($$('#repoSelector', this) as HTMLSelectElement)?.value || defaultRepo();
  }

  private searchFilter(e: Event) {
    this._filter = 'Search'; // Use the private member to avoid double-render
    this.search = (<HTMLInputElement>e.target).value;
  }

  // Arrow notation to allow for reference of same function in removeEventListener.
  private onClick = (event: Event) => {
    const target = event.target as HTMLElement;
    const dialog = $$('details-dialog-sk', this) as DetailsDialogSk;
    if (target.classList.contains('task-spec')) {
      const spec = target.getAttribute('title') || '';
      const comments = this.data.taskSpecs.get(spec)?.comments!;
      if (spec !== '' && comments !== undefined) {
        dialog.displayTaskSpec(spec, comments);
      }
    } else if (target.classList.contains('commit')) {
      const commit = this.data.commits[Number(target.dataset.commitIndex)]!;
      const comments = this.data.comments.get(commit.hash)?.get('') || [];
      dialog.displayCommit(commit, comments);
    } else if (target.hasAttribute('data-task-id')) {
      const task = this.data.tasks.get(target.dataset.taskId!)!;
      const comments = this.data.comments.get(task.revision)?.get(task.name) || [];
      dialog.displayTask(task, comments, this.data.commitsByHash);
    } else {
      dialog.close();
    }
  };

  private reRunMishapsUrl() {
    const jobStrings: { job: Array<string> } = {
      job: this.mishapTasks.map((task) => {
        // Jobs are named after task, test, or perf tasks, but not
        // uploads. If this is an upload, trim the prefix.
        var jobName = task.name;
        if (jobName.startsWith('Upload-')) {
          jobName = jobName.substring('Upload-'.length);
        }
        return `${jobName}@${task.revision}`;
      }),
    };
    return `${taskSchedulerUrl()}/trigger?${fromObject(jobStrings)}`;
  }

  private toggleCommitLabel() {
    this.displayCommitSubject = !this.displayCommitSubject;
  }
  /**
   * gridLocation returns a lit StyleMap Part to inline on an element to place it between the
   * provided css grid row and column tracks.
   */
  private gridLocation(
    rowStart: number,
    colStart: number,
    rowEnd: number = rowStart + 1,
    colEnd: number = colStart + 1
  ) {
    // RowStart / ColStart / RowEnd / ColEnd
    return styleMap({ gridArea: `${rowStart} / ${colStart} / ${rowEnd} / ${colEnd}` });
  }

  /**
   * includeTaskSpec checks the spec against the filter type currently set for the table and
   * returns true if the taskspec should be displayed.
   * @param taskSpec The taskSpec name to check against the filter.
   * @param searchRegex Regex to search with, must be set if this._filter === "Search".
   */
  private includeTaskSpec(taskSpec: string, searchRegex?: RegExp): boolean {
    const specDetails = this.data.taskSpecs.get(taskSpec);
    if (!specDetails) {
      return true;
    }
    switch (this._filter) {
      case 'All':
        return true;
      case 'Comments':
        return specDetails.hasComment();
      case 'Nocomment':
        return specDetails.hasFailingNoComment();
      case 'Failures':
        return specDetails.hasFailing();
      case 'Interesting':
        return specDetails.interesting();
      case 'Search':
        return searchRegex!.test(taskSpec);
    }
  }

  /**
   * taskSpecIcons returns any needed comment related icons for a task spec.
   * @param taskSpec The taskSpec to assess.
   */
  private taskSpecIcons(taskSpec: string): Array<TemplateResult> {
    const res: Array<TemplateResult> = [];
    const task = this.data.taskSpecs.get(taskSpec)!;
    if (task.comments.length > 0) {
      res.push(html`<comment-icon-sk class="tiny"></comment-icon-sk>`);
    }
    if (task.flaky) {
      res.push(html`<texture-icon-sk class="tiny"></texture-icon-sk>`);
    }
    if (task.ignoreFailure) {
      res.push(html`<block-icon-sk class="tiny"></block-icon-sk>`);
    }
    return res;
  }

  /**
   * taskIcon returns any needed comment icon for a task.
   * @param task The task to assess.
   */
  private taskIcon(task: Task): TemplateResult {
    return task.commits?.every((c) => {
      return !this.data.comments.get(c)?.get(task.name);
    })
      ? html``
      : html`<comment-icon-sk class="tiny"></comment-icon-sk>`;
  }

  /**
   * commitIcons returns any needed comment, revert, and reland related icons for a commit.
   * @param commit The commit to assess.
   */
  private commitIcons(commit: Commit): Array<TemplateResult> {
    const res: Array<TemplateResult> = [];

    const relanded = this.data.relandedMap.get(commit.hash);
    if (relanded && relanded.timestamp! > commit.timestamp!) {
      res.push(html`<redo-icon-sk
        class="tiny fill-green"
        @mouseenter=${() => this.highlightAssociatedCommit(relanded.hash, false)}
        @mouseleave=${() => this.highlightAssociatedCommit(relanded.hash, false)}
      >
      </redo-icon-sk>`);
    }
    const reverted = this.data.revertedMap.get(commit.hash);
    if (reverted && reverted.timestamp! > commit.timestamp!) {
      res.push(html`<undo-icon-sk
        class="tiny fill-red"
        @mouseenter=${() => this.highlightAssociatedCommit(reverted.hash, true)}
        @mouseleave=${() => this.highlightAssociatedCommit(reverted.hash, true)}
      >
      </undo-icon-sk>`);
    }
    if (commit.ignoreFailure) {
      res.push(html`<block-icon-sk class="tiny"></block-icon-sk>`);
    }
    if (this.data.comments.get(commit.hash)?.get('')?.length || 0 > 0) {
      res.push(html`<comment-icon-sk class="tiny"></comment-icon-sk>`);
    }
    return res;
  }

  /**
   * highlightAssociatedCommit toggles a class on the relevant commit's div when the mouse enters
   * or leaves a revert or reland icon.
   * @param hash Hash of the commit reverting/relanding this CL, which is also the commit div's id.
   * @param revert Use the revert highlight class instead of reland highlight class.
   */
  private highlightAssociatedCommit(hash: string, revert: boolean) {
    $$(`.${this.attributeStringFromHash(hash)}`, this)?.classList.toggle(
      revert ? REVERT_HIGHLIGHT_CLASS : RELAND_HIGHLIGHT_CLASS
    );
  }

  /**
   * attributeStringFromHash pads a hash with the string 'commit-' to avoid confusing JS with
   * leading digits, which fail querySelector.
   * @param hash The hash being padded.
   */
  private attributeStringFromHash(hash: string) {
    return `commit-${hash}`;
  }

  private addTaskHeaders(res: Array<TemplateResult>): Map<TaskSpec, number> {
    const taskSpecStartCols: Map<TaskSpec, number> = new Map();
    let categoryStartCol = TASK_START_COL;
    // We compile our regex once, rather than on ever taskspec.
    const searchRegex = this._filter === 'Search' ? new RegExp(this._search, 'i') : undefined;
    // We walk category/subcategory/taskspec info 'depth-first' so filtered out taskspecs can
    // correctly filter out unnecessary subcategories, etc.
    this.data.categories.forEach((categoryDetails: CategorySpec, categoryName: string) => {
      let subcategoryStartCol = categoryStartCol;
      categoryDetails.taskSpecsBySubCategory.forEach(
        (taskSpecs: Array<string>, subcategoryName: string) => {
          let taskSpecStartCol = subcategoryStartCol;
          taskSpecs
            .filter((ts) => this.includeTaskSpec(ts, searchRegex))
            .forEach((taskSpec: string) => {
              taskSpecStartCols.set(taskSpec, taskSpecStartCol);
              res.push(
                html`<div
                  class="category task-spec"
                  style=${this.gridLocation(TASKSPEC_START_ROW, taskSpecStartCol++)}
                  title=${taskSpec}
                >
                  ${this.taskSpecIcons(taskSpec)}
                </div>`
              );
            });
          if (taskSpecStartCol != subcategoryStartCol) {
            // Added at least one TaskSpec in this subcategory, so add a Subcategory header.
            const subcategoryEndCol = taskSpecStartCol;
            res.push(
              html`<div
                class="category"
                style=${this.gridLocation(
                  SUBCATEGORY_START_ROW,
                  subcategoryStartCol,
                  SUBCATEGORY_START_ROW + 1,
                  subcategoryEndCol
                )}
              >
                ${subcategoryName}
              </div>`
            );
            subcategoryStartCol = subcategoryEndCol;
          }
        }
      );
      if (subcategoryStartCol != categoryStartCol) {
        // Added at least one Subcategory in this category, so add a Category header.
        const categoryEndCol = subcategoryStartCol;
        res.push(
          html`<div
            class="category"
            style=${this.gridLocation(
              CATEGORY_START_ROW,
              categoryStartCol,
              CATEGORY_START_ROW + 1,
              categoryEndCol
            )}
          >
            ${categoryName}
          </div>`
        );
        categoryStartCol = categoryEndCol;
      }
    });
    return taskSpecStartCols;
  }

  private multiCommitTaskSlots(
    displayTaskRows: Array<boolean>,
    rowStart: number,
    task: Task
  ): Array<TemplateResult> {
    let currRow = rowStart;
    // Convert the array of bools describing which slots are covered to an array of templates,
    // where 'true's are styled, normal task divs that have dashed tops / bottoms when bordering
    // 'false's, and 'false's are hidden divs.
    // TODO(westont): Consider further optimizing for minimal divs for broken tasks
    // (combine the contiguous rows).
    return displayTaskRows.map((display, index) => {
      let ret: TemplateResult = display
        ? html` <div
            class=${taskClasses(task, ...this.getDashedBorderClasses(displayTaskRows, index))}
            style=${this.gridLocation(currRow - rowStart + 1, 1)}
            data-task-id=${task.id}
          >
            ${index === 0 ? this.taskIcon(task) : ''}
          </div>`
        : // On holes we just drop a hidden div.
          // TODO(westont): What if the other branch has jobs? Perhaps we should sort out
          // styling for an empty template, or reduce z index.
          html`<div
            class="hidden ${taskClasses(task)}"
            style=${this.gridLocation(currRow - rowStart + 1, 1)}
          ></div>`;
      currRow++;
      return ret;
    });
  }

  private addTasks(
    tasksBySpec: Map<string, Task>,
    taskSpecStartCols: Map<string, number>,
    rowStart: number,
    commitIndex: number,
    tasksAddedToTemplate: Set<string>,
    res: Array<TemplateResult>
  ) {
    if (tasksBySpec) {
      tasksBySpec.forEach((task: Task, name: TaskSpec) => {
        if (tasksAddedToTemplate.has(task.id)) {
          // We already added this task since it also covered a later commit.
          return;
        }
        const colStart = taskSpecStartCols.get(name);
        if (!colStart) {
          // This taskSpec wasn't added, must be filtered, skip it.
          return;
        }
        // We mark tasks as added, since the first time we see multi-commit
        // tasks we add them in their entirety.
        tasksAddedToTemplate.add(task.id);
        if (task.status === TASK_STATUS_MISHAP) {
          this.mishapTasks.push(task);
        }
        const displayTaskRows = this.displayTaskRows(task, commitIndex);
        if (displayTaskRows.every(Boolean)) {
          // The task bubble is contiguous, just draw a single div over that span.
          res.push(
            html`<div
              class=${taskClasses(task, 'grow')}
              style=${this.gridLocation(rowStart, colStart, rowStart + displayTaskRows.length)}
              title=${taskTitle(task)}
              data-task-id=${task.id}
              @mouseenter=${() => this.taskMouseInOut(task)}
              @mouseleave=${() => this.taskMouseInOut(task)}
            >
              ${this.taskIcon(task)}
            </div>`
          );
        } else {
          // A commit on another branch interrupted the task, draw mutiple divs to represent the
          // break. This looks like e.g. [true, false, true] for a task covering two
          // commits that have a single branch commit between them.
          res.push(
            html`<div
              class="multicommit-task grow"
              @mouseenter=${() => this.taskMouseInOut(task)}
              @mouseleave=${() => this.taskMouseInOut(task)}
              style=${this.gridLocation(rowStart, colStart, rowStart + displayTaskRows.length)}
            >
              ${this.multiCommitTaskSlots(displayTaskRows, rowStart, task)}
            </div>`
          );
        }
      });
    }
  }

  private taskMouseInOut(task: Task) {
    task.commits!.forEach((hash) => {
      $$<HTMLDivElement>(`.${this.attributeStringFromHash(hash)}`, this)!.classList.toggle(
        `task-emphasize-${task.status.toLowerCase()}`
      );
    });
  }
  // Return a time label if one should be used for the commit at the given index.
  private timeLabel(commits: Commit[], index: number, timePoints: { label: string; time: Date }[]) {
    if (index === commits.length - 1) {
      return null;
    }

    const curr = new Date(commits[index].timestamp!);
    const next = new Date(commits[index + 1].timestamp!);
    let ret = null;
    for (const moment of timePoints) {
      if (moment.time <= curr && moment.time > next) {
        ret = html`<span class="time-label">${moment.label}</span>`;
      }
    }
    return ret;
  }
  /**
   * fillTableTemplate returns an array of templates (containing headers, commits, tasks, etc),
   * each styled with 'grid-area' to place them inside a css-grid element, covering one or more
   * cells.
   */
  private fillTableTemplate(): Array<TemplateResult> {
    // Elements, each styled to cover one or more cells of a css grid element.
    // E.g.includes divs for commits and taskspecs that are single - row / column headings, but
    // also divs for tasks that may cover multiple commits(rows) and divs for category headings
    // that may span multiple columns.
    const res: Array<TemplateResult> = [];
    // Add headers and get grid column number of each TaskSpec.
    const taskSpecStartCols: Map<TaskSpec, number> = this.addTaskHeaders(res);
    // We use lastColumn to ensure our controls panel and row underlay covers all columns, always
    // at least 1 more than the commits panel, even if we have no tasks displayed.
    this.lastColumn = Math.max(taskSpecStartCols.size + TASK_START_COL, TASK_START_COL + 1);
    this.mishapTasks = [];
    const taskStartRow = COMMIT_START_ROW;
    const tasksAddedToTemplate: Set<TaskId> = new Set();
    const now = Date.now();
    // Explicitly privide 'now' in case we patched it out for testing.
    const today = new Date(now);
    today.setHours(0, 0, 0, 0);
    const yesterday = new Date(today.valueOf());
    yesterday.setDate(yesterday.getDate() - 1);
    const timePoints = [
      { label: '-1h', time: new Date(now - 60 * 60 * 1000) },
      { label: '-3h', time: new Date(now - 3 * 60 * 60 * 1000) },
      { label: 'today', time: today },
      { label: 'yesterday', time: yesterday },
    ];
    timePoints.sort((a, b) => b.time.valueOf() - a.time.valueOf());
    // Commits are ordered newest to oldest, so the first commit is visually near the top.
    for (const [i, commit] of this.data.commits.entries()) {
      const rowStart = taskStartRow + i;
      const title = this.displayCommitSubject ? commit.shortAuthor : commit.shortSubject;
      const text = !this.displayCommitSubject ? commit.shortAuthor : commit.shortSubject;
      const timeLabel = this.timeLabel(this.data.commits, i, timePoints);

      const tasksBySpec = this.data.tasksByCommit.get(commit.hash);
      if (tasksBySpec) {
        this.addTasks(tasksBySpec, taskSpecStartCols, rowStart, i, tasksAddedToTemplate, res);
      }
      // Draw commits last so the span.highlight-row naturally renders above the tasks.
      res.push(
        html`
          <div class="commit-container" style=${this.gridLocation(rowStart, COMMIT_START_COL)}>
            <div class="time-spacer">${timeLabel}</div>
            <div
              class="commit ${this.attributeStringFromHash(commit.hash)}"
              title=${title}
              data-commit-index=${i}
            >
              <span class="nowrap commit-text">${text}</span>
              <span class="nowrap icons">${this.commitIcons(commit)}</span>
            </div>
            ${timeLabel ? html`<span class="time-underline"></span>` : html``}
          </div>
          <span
            class="highlight-row"
            style=${this.gridLocation(
              rowStart,
              COMMIT_START_COL + 1,
              rowStart + 1,
              this.lastColumn
            )}
          ></span>
        `
      );
    }
    // Add a single div covering the grid, behind everything, that highlights alternate rows.
    let row = taskStartRow;
    const nextRowDiv = () => html` <div
      style=${this.gridLocation(row, 1, ++row, this.lastColumn)}
    ></div>`;
    res.push(html` <div class="rowUnderlay">
      ${Array(this.data.commits.length).fill(1).map(nextRowDiv)}
    </div>`);
    return res;
  }

  /**
   * getDashedBorderClasses provides classes for styling the borders against 'gaps' where commits
   * on different branches lie between commits covered by a task.
   *
   * @param displayTaskRows Value returned from displayTaskRows.
   * @param index Index in displayTaskRows that we're assessing.
   */
  private getDashedBorderClasses(displayTaskRows: Array<boolean>, index: number) {
    const ret: Array<string> = [];
    if (index > 0 && !displayTaskRows[index - 1]) {
      ret.push('dashed-top');
    }
    if (index < displayTaskRows.length - 1 && !displayTaskRows[index + 1]) {
      ret.push('dashed-bottom');
    }
    return ret;
  }

  /**
   * displayTaskRows returns an array describing which of the next N commits the task covers.
   * e.g. for Tasks covering contiguous commits (no commits on other branches), return will
   * be [true, ...[true]].  For tasks covering commits with interstitial other-branch commits,
   * return will be e.g. [true, false, true, true].
   * @param task The task being assessed.
   * @param latestCommitIndex: The index of the top/most recent commit covered by the task.
   */
  private displayTaskRows(task: Task, latestCommitIndex: number) {
    // Only a single commit, or the last shown commit, obviously contiguous.
    if (task.commits!.length < 2 || latestCommitIndex >= this.data.commits.length - 1) {
      return [true];
    }
    const thisTaskOverCommits: Array<boolean> = [true];
    // Check for parental gaps. Commits may be sorted, but we don't assume that.
    let displayCommitsCount = 1;
    // We update this as we 'walk backward' through the commits this task covers.
    let currentCommitInTask = this.data.commits[latestCommitIndex];
    // Follow the ancestory up to the penultimate commit, since we look ahead by 1.
    // Earlier here means visually below.
    for (
      let earlierCommitIndex = latestCommitIndex + 1;
      earlierCommitIndex < this.data.commits.length;
      earlierCommitIndex++
    ) {
      // Exit if we know we've account for all commits in the task, to avoid an extra 'false' at
      // the end of the returned array.
      if (displayCommitsCount === task.commits!.length) break;

      let earlierCommit = this.data.commits[earlierCommitIndex];
      if (currentCommitInTask.parents!.indexOf(earlierCommit.hash) === -1) {
        // Branch leaves a gap.
        thisTaskOverCommits.push(false);
      } else {
        // This is expected to be true, since this task covers at least one more commit, and the
        // next oldest commit is our current commits parent.
        if (task.commits!.indexOf(earlierCommit.hash) !== -1) {
          thisTaskOverCommits.push(true);
          displayCommitsCount++;
          currentCommitInTask = earlierCommit;
        }
      }
    }
    return thisTaskOverCommits;
  }

  private update() {
    if (!this.requestLimiter.beginUpdate()) {
      // There is already an outstanding request, we'll re-update once that resolves.
      return;
    }
    const refreshSeconds = Number((<HTMLInputElement>$$('#reloadInput', this)).value);
    const numCommits = Number((<HTMLInputElement>$$('#commitsInput', this)).value);
    window.clearTimeout(this.refreshHandle);
    this.refreshHandle = undefined;
    this.dispatchEvent(new CustomEvent('begin-task', { bubbles: true }));
    const previousLoad = this.lastLoaded ? new Date(this.lastLoaded.getTime()) : undefined;
    this.lastLoaded = new Date();
    this.data.update(this.repo, numCommits, previousLoad).finally(() => {
      this.draw();
      const branchesSk = $$('branches-sk', this) as BranchesSk;
      branchesSk.commits = this.data.commits;
      branchesSk.branchHeads = this.data.branchHeads;
      this.dispatchEvent(new CustomEvent('end-task', { bubbles: true }));
      // If an additional update was requested, start it, otherwise schedule it.
      if (this.requestLimiter.endUpdate()) {
        this.update();
      } else {
        this.refreshHandle = window.setTimeout(() => this.update(), refreshSeconds * 1000);
      }
    });
  }

  private draw() {
    console.time('render');
    this._render();
    console.timeEnd('render');
  }
}

define('commits-table-sk', CommitsTableSk);

function taskClasses(task: Task, ...classes: Array<string>) {
  const map: Record<string, any> = { task: true };
  map[`bg-${(task.status || 'PENDING').toLowerCase()}`] = true;
  classes.forEach((c) => (map[c] = true));
  return classMap(map);
}

function taskTitle(task: Task) {
  return `${task.name} @${task.commits!.length > 1 ? '\n' : ' '}${task.commits!.join(',\n')}`;
}

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
