/**
 * @module modules/job-search-sk
 * @description <h2><code>job-search-sk</code></h2>
 *
 * Provides UI for searching the jobs in the DB.
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { ParamSet, toParamSet, fromParamSet } from 'common-sk/modules/query';
import {
  TaskSchedulerService,
  SearchJobsResponse,
  SearchJobsRequest,
  Job,
  JobStatus,
} from '../rpc';
import 'elements-sk/icon/delete-icon-sk';
import 'elements-sk/styles/buttons';
import 'elements-sk/styles/table';
import { $$ } from 'common-sk/modules/dom';

// Names and types of search terms.
interface DisplaySearchTerm {
  label: string;
  type: string;
}

// TODO(borenet): Find a way not to duplicate the contents of SearchJobRequest.
const searchTerms: { [key: string]: DisplaySearchTerm } = {
  name: { label: 'Name', type: 'text' },
  repo: { label: 'Repo', type: 'text' },
  revision: { label: 'Revision', type: 'text' },
  issue: { label: 'Issue', type: 'text' },
  patchset: { label: 'Patchset', type: 'text' },
  buildbucketBuildId: { label: 'Buildbucket Build ID', type: 'text' },
  isForce: { label: 'Manually Triggered', type: 'checkbox' },
  status: { label: 'Status', type: 'text' },
  timeStart: { label: 'Start Time', type: 'datetime-local' },
  timeEnd: { label: 'End Time', type: 'datetime-local' },
};

// Display parameters for job results.
interface DisplayJobResult {
  label: string;
  class: string;
}

const jobStatusToLabelAndClass: { [key: string]: DisplayJobResult } = {
  [JobStatus.JOB_STATUS_IN_PROGRESS]: {
    label: 'in progress',
    class: 'bg-in-progress',
  },
  [JobStatus.JOB_STATUS_SUCCESS]: {
    label: 'succeeded',
    class: 'bg-success',
  },
  [JobStatus.JOB_STATUS_FAILURE]: {
    label: 'failed',
    class: 'bg-failure',
  },
  [JobStatus.JOB_STATUS_MISHAP]: {
    label: 'mishap',
    class: 'bg-mishap',
  },
  [JobStatus.JOB_STATUS_CANCELED]: {
    label: 'canceled',
    class: 'bg-canceled',
  },
};

interface SearchTerm {
  key: string;
  value: string;
}

export class JobSearchSk extends ElementSk {
  private static template = (ele: JobSearchSk) => html`
    <div class="container">
      <table class="searchTerms">
        ${Array.from(ele.searchTerms.values()).map(
          (term: SearchTerm) => html`
            <tr class="searchTerms">
              <th>
                <label for="${term.key}">
                  ${searchTerms[term.key]!.label}
                </label>
              </th>
              <td>
                ${term.key == 'status'
                  ? html`
                      <select
                        id="${term.key}"
                        @change="${(ev: Event) => {
                          const input = (<HTMLSelectElement>ev.target)!;
                          term.value = input.value;
                          ele.updateQuery();
                        }}"
                        selected=
                      >
                        ${Object.entries(jobStatusToLabelAndClass).map(
                          ([status, labelAndClass]) => html`
                            <option
                              value="${status}"
                              ?selected="${term.value == status}"
                            >
                              ${labelAndClass.label}
                            </option>
                          `
                        )}
                      </select>
                    `
                  : html`
                    <input
                        .id="${term.key}"
                        .type="${searchTerms[term.key]!.type}"
                        .value="${term.value}"
                        ?checked="${
                          searchTerms[term.key]!.type == 'checkbox' &&
                          term.value == 'true'
                        }"
                        @change="${(ev: Event) => {
                          const input = (<HTMLInputElement>ev.target)!;
                          if (searchTerms[term.key]!.type == 'checkbox') {
                            term.value = input.checked ? 'true' : 'false';
                          } else {
                            term.value = input.value;
                          }
                          ele.updateQuery();
                        }}"
                        >
                    </input>
                `}
              </td>
              <td>
                <button
                  class="delete"
                  @click="${() => {
                    ele.searchTerms.delete(term.key);
                    ele._render();
                    ele.updateQuery();
                  }}"
                >
                  <delete-icon-sk></delete-icon-sk>
                </button>
              </td>
            </tr>
          `
        )}
        <tr class="searchTerms">
          <td>
            <select
              @change="${(ev: Event) => {
                const select = <HTMLSelectElement>ev.target!;
                const selected = select.value;
                ele.searchTerms.set(selected, {
                  key: select.value,
                  value: '',
                });
                select.selectedIndex = 0;
                ele._render();
                ele.updateQuery();
                // Auto-focus the new input field.
                const inp = $$<HTMLInputElement>('#' + selected, ele)!;
                inp?.focus();
              }}"
            >
              <option disabled selected>Add Search Term</option>
              ${Object.entries(searchTerms)
                .filter(([key, _]) => !ele.searchTerms.get(key))
                .map(
                  ([key, term]) => html`
                    <option .value="${key}">${term.label}</option>
                  `
                )}
            </select>
          </td>
          <td></td>
          <td>
            <button class="search" @click="${ele.search}">Search</button>
          </td>
        </tr>
      </table>
    </div>
    ${ele.results && ele.results.length > 0
      ? html`
          <div class="container">
            <table>
              <tr>
                <th>ID</th>
                <th>Name</th>
                <th>Repo</th>
                <th>Revision</th>
                <th>Codereview Link</th>
                <th>Status</th>
                <th>Manually Triggered</th>
                <th>Created At</th>
                <th>
                  <button class="cancel" @click="${ele.cancelAll}">
                    <delete-icon-sk></delete-icon-sk>
                    Cancel All
                  </button>
                </th>
              </tr>

              ${ele.results.map(
                (job: Job) => html`
                  <tr>
                    <td>
                      <a href="/job/${job.id}" target="_blank">${job.id}</a>
                    </td>
                    <td>${job.name}</td>
                    <td>
                      <a href="${job.repoState?.repo}" target="_blank">
                        ${job.repoState?.repo}
                      </a>
                    </td>
                    <td>
                      <a
                        href="${job.repoState!.repo}/+show/${job.repoState!
                          .revision}"
                        target="_blank"
                      >
                        ${job.repoState!.revision.substring(0, 12)}
                      </a>
                    </td>
                    <td>
                      ${job.repoState?.patch?.issue &&
                      job.repoState?.patch?.patchset &&
                      job.repoState?.patch?.server
                        ? html`
                            <a
                              href="${job.repoState?.patch?.server}/c/${job
                                .repoState?.patch?.issue}/${job.repoState?.patch
                                ?.patchset}"
                              target="_blank"
                              >${job.repoState?.patch?.server}/c/${job.repoState
                                ?.patch?.issue}/${job.repoState?.patch
                                ?.patchset}
                            </a>
                          `
                        : html``}
                    </td>
                    <td class="${jobStatusToLabelAndClass[job.status]!.class}">
                      ${jobStatusToLabelAndClass[job.status]!.label}
                    </td>
                    <td>${job.isForce ? 'true' : 'false'}</td>
                    <td>${job.createdAt}</td>
                    <td>
                      ${job.status === JobStatus.JOB_STATUS_IN_PROGRESS
                        ? html`
                            <button
                              class="cancel"
                              @click="${() => ele.cancel(job)}"
                            >
                              <delete-icon-sk></delete-icon-sk>
                              Cancel
                            </button>
                          `
                        : html``}
                    </td>
                  </tr>
                `
              )}
            </table>
          </div>
        `
      : html``}
  `;

  private results: Job[] = [];
  private _rpc: TaskSchedulerService | null = null;
  private searchTerms: Map<string, SearchTerm> = new Map();

  constructor() {
    super(JobSearchSk.template);
  }

  get rpc(): TaskSchedulerService | null {
    return this._rpc;
  }

  set rpc(rpc: TaskSchedulerService | null) {
    this._rpc = rpc;
    if (this.searchTerms.size > 0) {
      this.search();
    }
  }

  connectedCallback() {
    super.connectedCallback();
    if (window.location.search) {
      const params = toParamSet(window.location.search.substring(1));
      Object.entries(params).forEach((entry: [string, string[]]) => {
        const key = entry[0];
        const value = entry[1][0]; // Just take the first one.
        this.searchTerms.set(key, {
          key: key,
          value: value,
        });
      });
      if (this.rpc) {
        this.search();
      }
    }
    this._render();
  }

  private updateQuery() {
    const params: ParamSet = {};
    this.searchTerms.forEach((term: SearchTerm) => {
      params[term.key] = [term.value];
    });
    const newUrl =
      window.location.href.split('?')[0] + '?' + fromParamSet(params);
    window.history.replaceState('', '', newUrl);
  }

  private search() {
    const req = {
      buildbucketBuildId: this.searchTerms.get('buildbucketBuildId')?.value || '',
      hasBuildbucketBuildId: !!this.searchTerms.get('buildbucketBuildId'),
      isForce: this.searchTerms.get('isForce')?.value === 'true',
      hasIsForce: !!this.searchTerms.get('isForce'),
      issue: this.searchTerms.get('issue')?.value || '',
      hasIssue: !!this.searchTerms.get('issue'),
      name: this.searchTerms.get('name')?.value || '',
      hasName: !!this.searchTerms.get('name'),
      patchset: this.searchTerms.get('patchset')?.value || '',
      hasPatchset: !!this.searchTerms.get('patchset'),
      repo: this.searchTerms.get('repo')?.value || '',
      hasRepo: !!this.searchTerms.get('repo'),
      revision: this.searchTerms.get('revision')?.value || '',
      hasRevision: !!this.searchTerms.get('revision'),
      status: (this.searchTerms.get('status')?.value || null) as JobStatus,
      hasStatus: !!this.searchTerms.get('status'),
      timeEnd: new Date(
        this.searchTerms.get('timeEnd')?.value || 0
      ).toISOString(),
      hasTimeEnd: !!this.searchTerms.get('timeEnd'),
      timeStart: new Date(
        this.searchTerms.get('timeStart')?.value || 0
      ).toISOString(),
      hasTimeStart: !!this.searchTerms.get('timeStart'),
    };
    this.rpc!.searchJobs(req as SearchJobsRequest).then(
      (resp: SearchJobsResponse) => {
        this.results = resp.jobs!;
        this._render();
      }
    );
  }

  private cancel(job: Job) {
    this.rpc!.cancelJob({ id: job.id }).then(() => {
      const result = this.results.find((result: Job) => result.id == job.id);
      if (result) {
        result.status = JobStatus.JOB_STATUS_CANCELED;
        this._render();
      }
    });
  }

  private cancelAll() {
    this.results.forEach((job: Job) => {
      if (job.status == JobStatus.JOB_STATUS_IN_PROGRESS) {
        this.cancel(job);
      }
    });
  }
}

define('job-search-sk', JobSearchSk);
