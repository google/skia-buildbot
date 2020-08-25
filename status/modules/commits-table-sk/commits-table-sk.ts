/**
 * @module modules/commits-table-sk
 * @description An element that manages fetching and processing commits data for Status.
 */

import { $$ } from 'common-sk/modules/dom';
import { define } from 'elements-sk/define';
import { html, TemplateResult, Part } from 'lit-html';
import { styleMap, StyleInfo } from 'lit-html/directives/style-map';
import { classMap, ClassInfo } from 'lit-html/directives/class-map';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { errorMessage } from 'elements-sk/errorMessage';
import { truncateWithEllipses } from '../../../golden/modules/common';


import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import {
    IncrementalCommitsResponse, Task, Comment, IncrementalUpdate,
    Branch, LongCommit, IncrementalCommitsRequest, ResponseMetadata
} from '../rpc/statusFe'
import 'elements-sk/select-sk';

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
export interface DisplayTask extends Task {
  addedToDom: boolean;
}

export class CommitsTableSk extends ElementSk {
  commits: Array<Commit> = [];
  commitsByHash: Map<CommitHash, Commit> = new Map();

  private static template = (el: CommitsTableSk) => html`
<commits-data-sk @commits-data-update=${el.draw}></commits-data-sk>
${el.data() ? CommitsTableSk.tableTemplate(el) : ''}
`;
// Break the template in two so we can conditionally use the commits-data-sk, we have to render once to make it exist, then we re render to fetch its data and fill in.
private static tableTemplate = (el: CommitsTableSk) => html`
<div id=commitsTableContainer>
  <div id=commits>
    <div id=legend>Legend placeholder</div>
    ${''/*el.data().commits.map((commit: Commit, index: number) => html`
    <div class=commit>
      <div class=time-spacer></div>
      <span class=commit-details>${commit.author}</span>
    </div>
`)*/}
  </div>
  <div id=tasksTable>
    ${el.computeTaskHeaders()}
  </div>
</div>`;

  constructor() {
      super(CommitsTableSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
    ($$('commits-data-sk', this) as CommitsDataSk)
  }

  gridLocation(rowStart: number, colStart: number, rowEnd: number = rowStart+1, colEnd: number = colStart+1) {
    // rowStart / ColStart / RowEnd / ColEnd
    return styleMap({'gridArea': `${rowStart} / ${colStart} / ${rowEnd} / ${colEnd}`})
  }
  computeTaskHeaders(): Array<TemplateResult> {
    const res: Array<TemplateResult> = [];
    // Work on rendering tasks. do this by creating a taskSpecStartCols Map<TaskSpecName, number> below, and using it.
    const taskSpecStartCols: Map<TaskSpec, number> = new Map();
    const categoryStartRow = 1;
    const subcategoryStartRow = categoryStartRow + 1;
    const taskSpecStartRow = subcategoryStartRow + 1;
    const taskStartRow = taskSpecStartRow + 1;
    let categoryStartCol = 2; // first column is commits.
    /*const tasksSeen: Set<string> = new Set();
    const taskSeen = (row: number, col: number) => {
      const ret = tasksSeen.has(`${row}/${col}`);
    };*/
    this.data().categories.forEach((categoryDetails: CategorySpec, categoryName: string) => {
      let subcategoryStartCol = categoryStartCol;
      // Draw Category header.
      const categoryEndCol = categoryStartCol + categoryDetails.colspan;
      res.push(
        html`<div class=category style=${this.gridLocation(categoryStartRow, categoryStartCol, categoryStartRow+1, categoryEndCol)}>${categoryName}</div>`
      )
      categoryStartCol = categoryEndCol;
      // Draw each Subcategory.
      categoryDetails.taskSpecsBySubCategory.forEach((taskSpecs: Array<string>, subcategoryName: string) => {
        let taskSpecStartCol = subcategoryStartCol;
        const subcategoryEndCol = subcategoryStartCol + taskSpecs.length;
        res.push(
          html`<div class=category style=${this.gridLocation(subcategoryStartRow, subcategoryStartCol, subcategoryStartRow+1, subcategoryEndCol)}>${subcategoryName}</div>`
        );
        subcategoryStartCol = subcategoryEndCol;
        // Draw each TaskSpec.
        taskSpecs.forEach((taskSpec: string) => {
          taskSpecStartCols.set(taskSpec, taskSpecStartCol);
          res.push(
            // TODO: flaky, comment, ignore failure boxes, not taskspec name.
            html`<div class=category style=${this.gridLocation(taskSpecStartRow, taskSpecStartCol++)}>${taskSpec}</div>`
          );
        });
      });
    });
    for (const [i, commit] of this.data().commits.entries()) {
      const rowStart = taskStartRow + i;
          res.push(
            html`<div class=commit style=${this.gridLocation(rowStart, 1)}>${commit.shortAuthor}</div>`
          );
      // TODO this is where we need to style the div color, and outline based on branch holes, etc
      // TODO deal with multiple commits by getting commit index of last commit from task, and expanding row end.
      const tasksBySpec = this.data().tasksByCommit.get(commit.hash);
      if (tasksBySpec) {
        tasksBySpec.forEach((task: Task, name: TaskSpec) => {
          // We mark tasks as added, since the first time we see multi-commit
          // tasks we complete the addition, and don't want to duplicate the
          // addition when processing later commits.
          const displayTask = task as DisplayTask;
          if (!displayTask.addedToDom) {
            displayTask.addedToDom = true;
            const colStart = taskSpecStartCols.get(name) as number;
            if (this.contiguousTask(task, i)) {
              res.push(
                html`<div class=${taskClasses(task)} style=${this.gridLocation(rowStart, colStart, rowStart + task.commits.length)}>${task.id}</div>`
              );
            } else {
              // branch F@#$^s up the div. :/

            }
          }
        });
      } else {
        //TODO wat.
      }
    }
    // Add a single div covering the grid, behind everything, that highlights alternate rows.
    let row = taskStartRow;
    const nextRowDiv = () => html`<div style=${this.gridLocation(row, 1, ++row, taskSpecStartCols.size + categoryStartCol)}></div>`;
    res.push(html`<div id=rowOverlay>${ Array(this.data().commits.length).fill(1).map(nextRowDiv)}</div>`);
    return res;
  }
  contiguousTask(task: Task, latestCommitIndex: number) {
    // Only a single commit, or the last shown commit, obviously contiguous.
    if (task.commits.length < 2 || latestCommitIndex >= this.data().commits.length-1) return true;
    const thisTaskOverCommits: Array<boolean> = [true,];
    let currCommit = this.data().commits[latestCommitIndex];
    let currCommitIndex = latestCommitIndex;
    // Next here means below, which is 'before us in time'.
    let earlierCommit = this.data().commits[++currCommitIndex];
    // Check for parental gaps. Commits may be sorted, but we don't assume that.
    if (currCommit.parents.indexOf(earlierCommit.hash) === -1) {
      // TODO DO NOT SUBMIT something isn't right here.
      // Branch leaves a gap.
      return false;
      thisTaskOverCommits.push(false);

    } else {
      // It would make sense that the task has this commit as well, since its commit list is >=2, and this is the parent of the latest commit the task covers.
      if (task.commits.indexOf(earlierCommit.hash) !== -1) {

      } else {
        console.error(`task has multiple commits, and covers ${currCommit.hash} but not ${earlierCommit.hash}`);
      }
    }

    return true;
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

function taskClasses(task : Task) {
  const statusClass = {};
  const classes : Record<string, any> = { 'task': 1 }; // `task-${(task.status || "PENDING").toLowerCase()}`
  classes[`task-${(task.status || "PENDING").toLowerCase()}`] = true;
  return classMap(classes);
}