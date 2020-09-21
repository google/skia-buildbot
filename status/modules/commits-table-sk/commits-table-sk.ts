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
 */

import { $, $$ } from 'common-sk/modules/dom';
import { define } from 'elements-sk/define';
import { html, TemplateResult } from 'lit-html';
import { styleMap } from 'lit-html/directives/style-map';
import { classMap } from 'lit-html/directives/class-map';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { Task } from '../rpc/status';

import 'elements-sk/select-sk';
import 'elements-sk/icon/comment-icon-sk';
import 'elements-sk/icon/texture-icon-sk';
import 'elements-sk/icon/block-icon-sk';
import 'elements-sk/icon/undo-icon-sk';
import 'elements-sk/icon/redo-icon-sk';
import '../commits-data-sk';
import {
  CommitsDataSk,
  CategorySpec,
  TaskSpec,
  TaskId,
  Commit,
} from '../commits-data-sk/commits-data-sk';

const CATEGORY_START_ROW = 1;
const SUBCATEGORY_START_ROW = 2;
const TASKSPEC_START_ROW = 3;

const REVERT_HIGHLIGHT_CLASS = 'highlight-revert';
const RELAND_HIGHLIGHT_CLASS = 'highlight-reland';

enum Filter {
  'interesting',
  'failures',
  'all',
  'nocomment',
  'comments',
  'search',
}

export class CommitsTableSk extends ElementSk {
  private _displayCommitSubject: boolean = false;
  private _filter: Filter = Filter.interesting;
  private _search: RegExp = new RegExp('');

  private static template = (el: CommitsTableSk) => html`<div class="commitsTableContainer">
    <div class="legend" style=${el.gridLocation(1, 1, 4)}>
      <comment-icon-sk class="tiny"></comment-icon-sk>Comments<br />
      <texture-icon-sk class="tiny"></texture-icon-sk>Flaky<br />
      <block-icon-sk class="tiny"></block-icon-sk>Ignore Failure<br />
      <undo-icon-sk class="tiny fill-red"></undo-icon-sk>Revert<br />
      <redo-icon-sk class="tiny fill-green"></redo-icon-sk>Reland<br />
    </div>
    <div class="tasksTable">${el.fillTableTemplate()}</div>
  </div>`;

  constructor() {
    super(CommitsTableSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
    this.data().addEventListener('end-task', () => this._render());
    this.addEventListener('click', this.onClick);
  }

  disconnectedCallback() {
    this.removeEventListener('click', this.onClick);
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

  // TODO(westont): Make this handler handle taskspec, task, and commit dialogs, once dialog
  // elements are written.
  // Arrow notation to allow for reference of same function in removeEventListener.
  private onClick = (event: Event) => {
    const target = event.target as HTMLElement;
    if (target.classList.contains('task-spec')) {
      console.log(`will show dialog for ${target.getAttribute('title')}`);
    } else {
      console.log('will close dialog');
    }
  };

  /**
   * gridLocation returns a lit StyleMap Part to inline on an element to place it between the
   * provided css grid row and column tracks.
   */
  gridLocation(
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
   */
  includeTaskSpec(taskSpec: string): boolean {
    const specDetails = this.data().taskSpecs.get(taskSpec);
    if (!specDetails) {
      return true;
    }
    switch (this._filter) {
      case Filter.all:
        return true;
      case Filter.comments:
        return specDetails.hasComment();
      case Filter.nocomment:
        return specDetails.hasFailingNoComment();
      case Filter.failures:
        return specDetails.hasFailing();
      case Filter.interesting:
        return specDetails.interesting();
      case Filter.search:
        return this._search.test(taskSpec);
    }
  }

  /**
   * taskSpecIcons returns any needed comment related icons for a task spec.
   * @param taskSpec The taskSpec to assess.
   */
  taskSpecIcons(taskSpec: string): Array<TemplateResult> {
    const res: Array<TemplateResult> = [];
    const task = this.data().taskSpecs.get(taskSpec)!;
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
  taskIcon(task: Task): TemplateResult {
    return task.commits?.every((c) => {
      return !this.data().comments.get(c)?.get(task.name);
    })
      ? html``
      : html`<comment-icon-sk class="tiny"></comment-icon-sk>`;
  }

  /**
   * commitIcons returns any needed comment, revert, and reland related icons for a commit.
   * @param commit The commit to assess.
   */
  commitIcons(commit: Commit): Array<TemplateResult> {
    const res: Array<TemplateResult> = [];
    if (this.data().comments.get(commit.hash)?.get('')?.length || 0 > 0) {
      res.push(html`<comment-icon-sk class="tiny icon-right"></comment-icon-sk>`);
    }
    if (commit.ignoreFailure) {
      res.push(html`<block-icon-sk class="tiny icon-right"></block-icon-sk>`);
    }
    const reverted = this.data().revertedMap.get(commit.hash);
    if (reverted && reverted.timestamp! > commit.timestamp!) {
      res.push(html`<undo-icon-sk
        class="tiny icon-right fill-red"
        @mouseenter=${() => this.highlightAssociatedCommit(reverted.hash, true)}
        @mouseleave=${() => this.highlightAssociatedCommit(reverted.hash, true)}
      >
      </undo-icon-sk>`);
    }
    const relanded = this.data().relandedMap.get(commit.hash);
    if (relanded && relanded.timestamp! > commit.timestamp!) {
      res.push(html`<redo-icon-sk
        class="tiny icon-right fill-green"
        @mouseenter=${() => this.highlightAssociatedCommit(relanded.hash, false)}
        @mouseleave=${() => this.highlightAssociatedCommit(relanded.hash, false)}
      >
      </redo-icon-sk>`);
    }
    return res;
  }

  /**
   * highlightAssociatedCommit toggles a class on the relevant commit's div when the mouse enters
   * or leaves a revert or reland icon.
   * @param hash Hash of the commit reverting/relanding this CL, which is also the commit div's id.
   * @param revert Use the revert highlight class instead of reland highlight class.
   */
  highlightAssociatedCommit(hash: string, revert: boolean) {
    $$(`#${hash}`, this)?.classList.toggle(
      revert ? REVERT_HIGHLIGHT_CLASS : RELAND_HIGHLIGHT_CLASS
    );
  }

  addTaskHeaders(res: Array<TemplateResult>): Map<TaskSpec, number> {
    const taskSpecStartCols: Map<TaskSpec, number> = new Map();
    let categoryStartCol = 2; // first column is commits.
    // We walk category/subcategory/taskspec info 'depth-first' so filtered out taskspecs can
    // correctly filter out unnecessary subcategories, etc.
    this.data().categories.forEach((categoryDetails: CategorySpec, categoryName: string) => {
      let subcategoryStartCol = categoryStartCol;
      categoryDetails.taskSpecsBySubCategory.forEach(
        (taskSpecs: Array<string>, subcategoryName: string) => {
          let taskSpecStartCol = subcategoryStartCol;
          taskSpecs
            .filter((ts) => this.includeTaskSpec(ts))
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

  multiCommitTaskSlots(
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

  addTasks(
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
        const displayTaskRows = this.displayTaskRows(task, commitIndex);
        if (displayTaskRows.every(Boolean)) {
          // The task bubble is contiguous, just draw a single div over that span.
          res.push(
            html`<div
              class=${taskClasses(task, 'grow')}
              style=${this.gridLocation(rowStart, colStart, rowStart + displayTaskRows.length)}
              title=${taskTitle(task)}
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
              style=${this.gridLocation(rowStart, colStart, rowStart + displayTaskRows.length)}
            >
              ${this.multiCommitTaskSlots(displayTaskRows, rowStart, task)}
            </div>`
          );
        }
      });
    }
  }

  /**
   * fillTableTemplate returns an array of templates (containing headers, commits, tasks, etc),
   * each styled with 'grid-area' to place them inside a css-grid element, covering one or more
   * cells.
   */
  fillTableTemplate(): Array<TemplateResult> {
    // Elements, each styled to cover one or more cells of a css grid element.
    // E.g.includes divs for commits and taskspecs that are single - row / column headings, but
    // also divs for tasks that may cover multiple commits(rows) and divs for category headings
    // that may span multiple columns.
    const res: Array<TemplateResult> = [];
    // Add headers and get grid column number of each TaskSpec.
    const taskSpecStartCols: Map<TaskSpec, number> = this.addTaskHeaders(res);
    const taskStartRow = TASKSPEC_START_ROW + 1;
    const tasksAddedToTemplate: Set<TaskId> = new Set();
    // Commits are ordered newest to oldest, so the first commit is visually near the top.
    for (const [i, commit] of this.data().commits.entries()) {
      const rowStart = taskStartRow + i;
      const title = this.displayCommitSubject ? commit.shortAuthor : commit.shortSubject;
      const text = !this.displayCommitSubject ? commit.shortAuthor : commit.shortSubject;
      res.push(
        html`<div
          class="commit"
          style=${this.gridLocation(rowStart, 1)}
          id=${commit.hash}
          title=${title}
        >
          ${text}${this.commitIcons(commit)}
        </div>`
      );
      const tasksBySpec = this.data().tasksByCommit.get(commit.hash);
      if (tasksBySpec) {
        this.addTasks(tasksBySpec, taskSpecStartCols, rowStart, i, tasksAddedToTemplate, res);
      }
    }
    // Add a single div covering the grid, behind everything, that highlights alternate rows.
    let row = taskStartRow;
    const nextRowDiv = () => html` <div
      style=${this.gridLocation(row, 1, ++row, taskSpecStartCols.size + 2)}
    ></div>`;
    res.push(html` <div class="rowUnderlay">
      ${Array(this.data().commits.length).fill(1).map(nextRowDiv)}
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

  /**
   * displayTaskRows returns an array describing which of the next N commits the task covers.
   * e.g. for Tasks covering contiguous commits (no commits on other branches), return will
   * be [true, ...[true]].  For tasks covering commits with interstitial other-branch commits,
   * return will be e.g. [true, false, true, true].
   * @param task The task being assessed.
   * @param latestCommitIndex: The index of the top/most recent commit covered by the task.
   */
  displayTaskRows(task: Task, latestCommitIndex: number) {
    // Only a single commit, or the last shown commit, obviously contiguous.
    if (task.commits!.length < 2 || latestCommitIndex >= this.data().commits.length - 1) {
      return [true];
    }
    const thisTaskOverCommits: Array<boolean> = [true];
    // Check for parental gaps. Commits may be sorted, but we don't assume that.
    let displayCommitsCount = 1;
    // We update this as we 'walk backward' through the commits this task covers.
    let currentCommitInTask = this.data().commits[latestCommitIndex];
    // Follow the ancestory up to the penultimate commit, since we look ahead by 1.
    // Earlier here means visually below.
    for (
      let earlierCommitIndex = latestCommitIndex + 1;
      earlierCommitIndex < this.data().commits.length;
      earlierCommitIndex++
    ) {
      // Exit if we know we've account for all commits in the task, to avoid an extra 'false' at
      // the end of the returned array.
      if (displayCommitsCount === task.commits!.length) break;

      let earlierCommit = this.data().commits[earlierCommitIndex];
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

  // TODO(westont): Combine this class with commits-data-sk.
  data(): CommitsDataSk {
    return $$('commits-data-sk') as CommitsDataSk;
  }

  draw() {
    this._render();
  }
}

define('commits-table-sk', CommitsTableSk);

function taskClasses(task: Task, ...classes: Array<string>) {
  const map: Record<string, any> = { task: true };
  map[`task-${(task.status || 'PENDING').toLowerCase()}`] = true;
  classes.forEach((c) => (map[c] = true));
  return classMap(map);
}

function taskTitle(task: Task) {
  return `${task.name} @${task.commits!.length > 1 ? '\n' : ' '}${task.commits!.join(',\n')}`;
}
