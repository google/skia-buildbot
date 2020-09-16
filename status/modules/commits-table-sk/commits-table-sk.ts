/**
 * @module modules/commits-table-sk
 * @description An element that manages fetching and processing commits data for Status.
 */

import { $, $$ } from 'common-sk/modules/dom';
import { define } from 'elements-sk/define';
import { html, TemplateResult, Part, Template } from 'lit-html';
import { styleMap, StyleInfo } from 'lit-html/directives/style-map';
import { classMap, ClassInfo } from 'lit-html/directives/class-map';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { errorMessage } from 'elements-sk/errorMessage';
import { truncateWithEllipses } from '../../../golden/modules/common';


import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import {
  GetIncrementalCommitsResponse, Task, Comment, IncrementalUpdate,
  Branch, LongCommit, GetIncrementalCommitsRequest, ResponseMetadata
} from '../rpc/status'
import 'elements-sk/select-sk';
import 'elements-sk/icon/comment-icon-sk'
import 'elements-sk/icon/texture-icon-sk'
import 'elements-sk/icon/block-icon-sk'
import 'elements-sk/icon/undo-icon-sk'
import 'elements-sk/icon/redo-icon-sk'

import '../commits-data-sk';
import { CommitsDataSk, CategorySpec, TaskSpecDetails } from '../commits-data-sk/commits-data-sk';

const TASK_STATUS_PENDING = "";
const TASK_STATUS_RUNNING = "RUNNING";
const TASK_STATUS_SUCCESS = "SUCCESS";
const TASK_STATUS_FAILURE = "FAILURE";
const TASK_STATUS_MISHAP = "MISHAP";

type CommitHash = string;
type TaskSpec = string;
type TaskId = string;

const CATEGORY_START_ROW = 1;
const SUBCATEGORY_START_ROW = 2;
const TASKSPEC_START_ROW = 3;

enum Filter {
  "interesting",
  "failures",
  "all",
  "nocomment",
  "comments",
  "search"
}


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

export class CommitsTableSk extends ElementSk {
  private _displayCommitSubject: boolean = false;
  private _filter: Filter = Filter.interesting;
  private _search: RegExp = new RegExp('');


  private static template = (el: CommitsTableSk) => html`
${el.data() ? CommitsTableSk.tableTemplate(el) : ''}
`;
  // Break the template in two so we can conditionally use the commits-data-sk, we have to render once to make it exist, then we re render to fetch its data and fill in.
  private static tableTemplate = (el: CommitsTableSk) => html`
<div id=commitsTableContainer>
  <div id=legend style=${el.gridLocation(1, 1, 4)}>
    <comment-icon-sk class=tiny></comment-icon-sk>Comments<br/>
    <texture-icon-sk class=tiny></texture-icon-sk>Flaky<br/>
    <block-icon-sk class=tiny></block-icon-sk>Flaky<br/>
    <undo-icon-sk class="tiny fill-red"></undo-icon-sk>Revert<br/>
    <redo-icon-sk class="tiny fill-green"></redo-icon-sk>Reland<br/>
  </div>
  <div id=tasksTable>
    ${el.addTasks()}
  </div>
</div>`;

  constructor() {
    super(CommitsTableSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
    document.addEventListener('end-task', () => this._render());
    document.addEventListener("click", (event) => {
      // TaskSpec clicks open a dialog, all others close it.
      const target = event.target as HTMLElement;
      if (target.classList.contains('task-spec')) {
        console.log(`will show dialog for ${target.getAttribute('title')}`);
      } else {
        console.log('will close dialog');
      }

    })
  }

  get displayCommitSubject() {
    return this._displayCommitSubject;
  }
  set displayCommitSubject(v: boolean) {
    this._displayCommitSubject = v;
    $('.commit').forEach((el, i) => {
      if (v) {
        el.innerHTML = this.data().commits[i].shortSubject;
        el.setAttribute('title', this.data().commits[i].shortAuthor);
      } else {
        el.innerHTML = this.data().commits[i].shortAuthor;
        el.setAttribute('title', this.data().commits[i].shortSubject);

      }
    });
  }
  get filter(): string {
    return Filter[this._filter];
  }
  set filter(v: string) {
    this._filter = (<any>Filter)[v] || Filter.interesting;
    this.draw();
  }
  get search(): string {
    return this._search.toString();
  }
  set search(v: string) {
    this._search = new RegExp(v, 'i');
    this.draw();
  }

  gridLocation(rowStart: number, colStart: number, rowEnd: number = rowStart + 1, colEnd: number = colStart + 1) {
    // RowStart / ColStart / RowEnd / ColEnd
    return styleMap({ 'gridArea': `${rowStart} / ${colStart} / ${rowEnd} / ${colEnd}` })
  }

  taskSpecComments(): Map<TaskSpec, Array<Comment>> | undefined {
    return this.data().comments.get('');
  }

  includeTaskSpec(taskSpec: string): boolean {
    const specDetails = this.data().taskSpecs.get(taskSpec);
    if (!specDetails) {
      return true;
    }
    switch (this._filter) {
      case Filter.all: return true;
      case Filter.comments: return specDetails.hasComment();
      case Filter.nocomment: return specDetails.hasFailingNoComment();
      case Filter.failures: return specDetails.hasFailing();
      case Filter.interesting: return specDetails.interesting();
      case Filter.search: return this._search.test(taskSpec);
    }
  }

  taskSpecIcons(taskSpec: string): Array<TemplateResult> {
    const res: Array<TemplateResult> = [];
    const task = this.data().taskSpecs.get(taskSpec)!;
    if (task.comments.length > 0) {
      res.push(html`<comment-icon-sk class=tiny></comment-icon-sk>`);
    }
    if (task.flaky) {
      res.push(html`<texture-icon-sk class=tiny></texture-icon-sk>`);
    }
    if (task.ignoreFailure) {
      res.push(html`<block-icon-sk class=tiny></block-icon-sk>`);
    }
    return res;
  }

  taskIcon(task: Task): TemplateResult {
    return task.commits?.every((c) => {
      return !this.data().comments.get(c)?.get(task.name);
    }) ? html`` : html`<comment-icon-sk class=tiny></comment-icon-sk>`;
  }

  commitIcons(commit: Commit): Array<TemplateResult> {
    const res: Array<TemplateResult> = [];
    if (this.data().comments.get(commit.hash)?.get('')?.length || 0 > 0) {
      res.push(html`<comment-icon-sk class="tiny icon-right"></comment-icon-sk>`);
    }
    if (commit.ignoreFailure) {
      res.push(html`<block-icon-sk class="tiny icon-right"></block-icon-sk>`);
    }
    const reverted = this.data().reverted_map.get(commit.hash);
    if (reverted && (reverted.timestamp! > commit.timestamp!)) {
      res.push(html`<undo-icon-sk class="tiny icon-right" @mouseenter=${() => this.highlightAssociatedCommit(reverted.hash, true)} @mouseleave=${() => this.highlightAssociatedCommit(reverted.hash, true)}></undo-icon-sk>`);
    }
    const relanded = this.data().relanded_map.get(commit.hash);
    if (relanded && (relanded.timestamp! > commit.timestamp!)) {
      res.push(html`<redo-icon-sk class="tiny icon-right" @mouseenter=${() => this.highlightAssociatedCommit(relanded.hash, false)} @mouseleave=${() => this.highlightAssociatedCommit(relanded.hash, false)}></redo-icon-sk>`);
    }
    return res;
  }

  highlightAssociatedCommit(hash: string, revert: boolean) {
    $$(`#${hash}`, this)?.classList.toggle(revert ? 'highlight-revert' : 'highlight-reland')
  }

  addTaskHeaders(res: Array<TemplateResult>): Map<TaskSpec, number> {
    console.log('adding headers');
    console.log(this.data().tasksBySpec.size);
    const taskSpecStartCols: Map<TaskSpec, number> = new Map();
    let categoryStartCol = 2; // first column is commits.
    // We walk category/subcategory/taskspec info 'depth-first' so filtered out taskspecs can correctly filter out unnecessary subcategories, etc.
    this.data().categories.forEach((categoryDetails: CategorySpec, categoryName: string) => {
      let subcategoryStartCol = categoryStartCol;
      categoryDetails.taskSpecsBySubCategory.forEach((taskSpecs: Array<string>, subcategoryName: string) => {
        let taskSpecStartCol = subcategoryStartCol;
        taskSpecs.filter((ts) => this.includeTaskSpec(ts)).forEach((taskSpec: string) => {
          taskSpecStartCols.set(taskSpec, taskSpecStartCol);
          res.push(
            // TODO: flaky, comment, ignore failure boxes, not taskspec name.
            html`<div class="category task-spec" style=${this.gridLocation(TASKSPEC_START_ROW, taskSpecStartCol++)} title=${taskSpec}>${this.taskSpecIcons(taskSpec)}</div>`
          );
        });
        if (taskSpecStartCol != subcategoryStartCol) {
          // Added at least one TaskSpec in this subcategory, so add a Subcategory header.
          const subcategoryEndCol = taskSpecStartCol;
          res.push(
            html`<div class=category style=${this.gridLocation(SUBCATEGORY_START_ROW, subcategoryStartCol, SUBCATEGORY_START_ROW + 1, subcategoryEndCol)}>${subcategoryName}</div>`
          );
          subcategoryStartCol = subcategoryEndCol;
        }
      });
      if (subcategoryStartCol != categoryStartCol) {
        // Added at least one Subcategory in this category, so add a Category header.
        const categoryEndCol = subcategoryStartCol;
        res.push(
          html`<div class=category style=${this.gridLocation(CATEGORY_START_ROW, categoryStartCol, CATEGORY_START_ROW + 1, categoryEndCol)}>${categoryName}</div>`
        )
        categoryStartCol = categoryEndCol;
      }
    });
    return taskSpecStartCols;
  }

  addTasks(): Array<TemplateResult> {
    const res: Array<TemplateResult> = [];
    // Grid column number of each TaskSpec.
    const taskSpecStartCols: Map<TaskSpec, number> = this.addTaskHeaders(res);
    const taskStartRow = TASKSPEC_START_ROW + 1;
    const tasksAddedToTemplate: Set<TaskId> = new Set();
    for (const [i, commit] of this.data().commits.entries()) {
      console.log('THIS IS ADDIG A COMMIT');
      const rowStart = taskStartRow + i;
      const title = this.displayCommitSubject ? commit.shortAuthor : commit.shortSubject
      const text = !this.displayCommitSubject ? commit.shortAuthor : commit.shortSubject
      res.push(
        html`<div class=commit style=${this.gridLocation(rowStart, 1)} id=${commit.hash} title=${title}>${text}${this.commitIcons(commit)}</div>`
      );
      const tasksBySpec = this.data().tasksByCommit.get(commit.hash);
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
          // tasks we complete the addition, and don't want to duplicate the
          // addition when processing later commits.
          tasksAddedToTemplate.add(task.id);
          const displayTaskRows = this.displayTaskRows(task, i);
          if (displayTaskRows.every(Boolean)) {
            // The task bubble is contiguous, just draw a single div over that span.
            res.push(
              html`<div class=${taskClasses(task, "grow")} style=${this.gridLocation(rowStart, colStart, rowStart + displayTaskRows.length)} title=${taskTitle(task)}>${this.taskIcon(task)}</div>`
            );
          } else {
            // A commit on another branch interrupted the task, draw mutiple divs to represent the break.
            let currRow = rowStart;
            // TODO(westont): consider further optimizing for minimal divs for broken tasks (combine contiguous rows).
            res.push(
              html`<div class="multicommit-task grow" style=${this.gridLocation(rowStart, colStart, rowStart + displayTaskRows.length)}>
                ${displayTaskRows.map((display, index) => {
                // On holes we just drop a hidden div.
                // TODO(westont): What if the other branch has tryjobs? Perhaps we should sort out styling for an empty template, or reduce z index.
                let ret: TemplateResult = html`<div class="hidden ${taskClasses(task)}" style=${this.gridLocation(currRow - rowStart + 1, 1)}></div>`;
                if (display) {
                  let brokenBorderClasses = this.getDashedBorderClasses(displayTaskRows, index);
                  ret = html`<div class=${taskClasses(task, ...brokenBorderClasses)} style=${this.gridLocation(currRow - rowStart + 1, 1)}>${index === 0 ? this.taskIcon(task) : ''}</div>`;
                }
                currRow++;
                return ret;
              })}
                    </div>`
            );
          }
        });
      }
    }
    // Add a single div covering the grid, behind everything, that highlights alternate rows.
    let row = taskStartRow;
    const nextRowDiv = () => html`<div style=${this.gridLocation(row, 1, ++row, taskSpecStartCols.size + 2)}></div>`;
    res.push(html`<div id=rowOverlay>${Array(this.data().commits.length).fill(1).map(nextRowDiv)}</div>`);
    return res;
  }

  getDashedBorderClasses(displayTaskRows: Array<boolean>, index: number) {
    const ret: Array<string> = [];
    if (index > 0 && !displayTaskRows[index - 1]) {
      ret.push('dashed-top');
    }
    if (index < displayTaskRows.length - 1 && !displayTaskRows[index + 1]) {
      ret.push('dashed-bottom');
    }
    return ret;
  }

  openTaskSpecDialog(name: string) {


  }

  displayTaskRows(task: Task, latestCommitIndex: number) {
    // Only a single commit, or the last shown commit, obviously contiguous.
    if (task.commits!.length < 2 || latestCommitIndex >= this.data().commits.length - 1) return [true];
    const thisTaskOverCommits: Array<boolean> = [true,];
    // Check for parental gaps. Commits may be sorted, but we don't assume that.
    let displayCommitsCount = 1;
    // We update this as we 'walk backward' through the commits this task covers.
    let currentCommitInTask = this.data().commits[latestCommitIndex];
    // Follow the ancestory up to the penultimate commit, since we look ahead by 1.
    // Earlier here means visually below.
    for (let earlierCommitIndex = latestCommitIndex + 1; earlierCommitIndex < this.data().commits.length; earlierCommitIndex++) {
      // Exit if we know we've account for all commits in the task, to avoid an extra 'false' at the end of the returned array.
      if (displayCommitsCount === task.commits!.length) break;

      let earlierCommit = this.data().commits[earlierCommitIndex];
      if (currentCommitInTask.parents!.indexOf(earlierCommit.hash) === -1) {
        // Branch leaves a gap.
        thisTaskOverCommits.push(false);
      } else {
        // This is expected to be true, since this task covers at least one more commit, and the next oldest commit is our current commits parent.
        if (task.commits!.indexOf(earlierCommit.hash) !== -1) {
          thisTaskOverCommits.push(true);
          displayCommitsCount++;
          currentCommitInTask = earlierCommit;
        }
      }
    }
    return thisTaskOverCommits;
  }

  data(): CommitsDataSk {
    return $$('commits-data-sk') as CommitsDataSk;
  }

  draw() {
    this._render();
  }
};

define('commits-table-sk', CommitsTableSk);

// shortCommit returns the first 7 characters of a commit hash.
function shortCommit(commit: string): string {
  return commit.substring(0, 7);
}

function taskClasses(task: Task, ...classes: Array<string>) {
  const map: Record<string, any> = { 'task': true };
  map[`task-${(task.status || "PENDING").toLowerCase()}`] = true;
  classes.forEach(c => map[c] = true);
  return classMap(map);
}

function taskTitle(task: Task) {
  return `${task.name} @${task.commits!.length > 1 ? '\n' : ' '}${task.commits!.join(",\n")}`;
}
