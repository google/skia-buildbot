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

export class CommitsDataSk extends ElementSk {
    commits: Array<LongCommit> = [];
    branch_heads: Array<Branch> = [];
    swarming_url: string | null = null;
    task_scheduler_url: string | null = null;
    // taskId -> Task
  tasks: Map<string, Task> = new Map();
  // commitHash -> (taskName -> Task)
  tasksByCommit: Map<string, Map<string, Task>> = new Map();

    task_details: object | null = null;
    task_specs: object | null = null;
    categories: object | null = null;
    category_list: Array<string> = [];
    commits_map: object | null = null;
    logged_in: boolean | null = null;
    relanded_map: object | null = null;
    repo: string | null = null;
    repo_base: string | null = null;
    reverted_map: object | null = null;

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
              this.commits = update.commits;
              this.branch_heads = update.branchheads || this.branch_heads;
              this.swarming_url = update.swarmingurl || this.swarming_url;
              this.task_scheduler_url = update.taskschedulerurl || this.task_scheduler_url;

              // Map task Id to Task
              if (update.tasks) {
                for (let i = 0; i < update.tasks.length; i++) {
                  const task = update.tasks[i];
                  this.tasks.set(task.id, task);
                }
              }

              // TODO: remove old commits?

              // Map commits to tasks
              for (let entry of this.tasks) {
                const id: string = entry[0];
                const task: Task = entry[1];
                if (task.commits) {
                  for (let commit of task.commits) {
                    let tasksForCommit = this.tasksByCommit.get(commit);
                if (!tasksForCommit) {
                  tasksForCommit = new Map();
                  this.tasksByCommit = tasksForCommit;
                }
                tasksForCommit[task.name] = task;
                  }
                }
              }





                /*task_details: object | null = null;
                task_specs: object | null = null;
                tasks: object | null = null;
                categories: object | null = null;
                category_list: Array<string> | null = null;
                -----commits: Array<object> | null = null;
                commits_map: object | null = null;
                logged_in: boolean | null = null;
                relanded_map: object | null = null;
                repo: string | null = null;
                repo_base: string | null = null;
                reverted_map: object | null = null;
                swarming_url: string | null = null;
                task_scheduler_url: string | null = null;*/
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