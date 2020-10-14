/**
 * @module modules/job-search-sk
 * @description <h2><code>job-search-sk</code></h2>
 *
 * @evt
 *
 * @attr
 *
 * @example
 */
import { define } from 'elements-sk/define';
import { $$ } from 'common-sk/modules/dom';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { ParamSet, toParamSet, fromParamSet } from 'common-sk/modules/query';
import {
  TaskSchedulerService,
  SearchJobsResponse,
  SearchJobsRequest,
  Job,
  JobStatus,
  CancelJobResponse,
} from '../rpc';
import 'elements-sk/collapse-sk';
import 'elements-sk/icon/delete-icon-sk';
import { job2ID } from '../rpc-mock';

// Names and types of search terms.
// TODO(borenet): Find a way not to duplicate the contents of SearchJobRequest.
const searchTerms = new Map<string, [string, string]>();
searchTerms.set('name', ['Name', 'text']);
searchTerms.set('repo', ['Repo', 'text']);
searchTerms.set('revision', ['Revision', 'text']);
searchTerms.set('issue', ['Issue', 'text']);
searchTerms.set('patchset', ['Patchset', 'text']);
searchTerms.set('buildbucketBuildId', ['Buildbucket Build ID', 'text']);
searchTerms.set('isForce', ['Manually Triggered', 'checkbox']);
searchTerms.set('status', ['Status', 'text']);
searchTerms.set('timeStart', ['Start Time', 'datetime-local']);
searchTerms.set('timeEnd', ['End Time', 'datetime-local']);

interface searchTerm {
  key: string;
  value: string;
}

export class JobSearchSk extends ElementSk {
  private static template = (ele: JobSearchSk) => html`
    <collapse-sk>
      <table>
        ${Array.from(ele.searchTerms.entries())
          .map((e) => e[1])
          .map(
            (term: searchTerm) => html`
        <tr>
          <td>
            <label for="${searchTerms.get(term.key)![1]}">
              ${searchTerms.get(term.key)![0]}
            </label>
          </td>
          <td>
            <input
                .id="${term.key}"
                .type="${searchTerms.get(term.key)![1]}"
                .value="${term.value}"
                ?checked="${
                  searchTerms.get(term.key)![1] == 'checkbox' &&
                  term.value == 'true'
                }"
                @change="${(ev: Event) => {
                  const input = (<HTMLInputElement>ev.target)!;
                  if (searchTerms.get(term.key)![1] == 'checkbox') {
                    term.value = input.checked ? 'true' : 'false';
                  } else {
                    term.value = input.value;
                  }
                  ele.updateQuery();
                }}"
                >
            </input>
          </td>
          <td>
            <button @click="${() => {
              ele.searchTerms.delete(term.key);
              ele._render();
              ele.updateQuery();
            }}">
              <delete-icon-sk></delete-icon-sk>
            </button>
        </tr>
      `
          )}
      </table>
      <select
        @change="${(ev: Event) => {
          const select = <HTMLSelectElement>ev.target!;
          ele.searchTerms.set(select.value, {
            key: select.value,
            value: '',
          });
          select.selectedIndex = 0;
          ele._render();
        }}"
      >
        <option disabled selected>Add Search Term</option>
        ${Array.from(searchTerms.entries())
          .filter(
            (entry: [string, [string, string]]) =>
              !ele.searchTerms.get(entry[0])
          )
          .map(
            (entry: [string, [string, string]]) => html`
              <option .value="${entry[0]}">${entry[1][0]}</option>
            `
          )}
      </select>
      <button @click="${ele.search}">Search</button>
    </collapse-sk>
    ${ele.results
      ? html`
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
                <button @click="${ele.cancelAll}">Cancel All</button>
              </th>
            </tr>

            ${ele.results.map(
              (job: Job) => html`
                <tr>
                  <td>
                    <a href="/job/${job.id}" target="_blank">${job.id}</a>
                  </td>
                  <td>${job.name}</td>
                  <td>${job.repoState?.repo}</td>
                  <td>${job.repoState?.revision}</td>
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
                              ?.patch?.issue}/${job.repoState?.patch?.patchset}
                          </a>
                        `
                      : html``}
                  </td>
                  <td class="${ele.statusClass(job)}">${job.status}</td>
                  <td>${job.isForce ? 'true' : 'false'}</td>
                  <td>
                    ${ele.isFinished(job)
                      ? html``
                      : html`
                          <button @click="${() => ele.cancel(job)}">
                            Cancel
                          </button>
                        `}
                  </td>
                </tr>
              `
            )}
          </table>
        `
      : html``}
  `;

  private results: Job[] = [];
  private _rpc: TaskSchedulerService | null = null;
  private searchTerms: Map<string, searchTerm> = new Map();

  constructor() {
    super(JobSearchSk.template);
  }

  get rpc(): TaskSchedulerService | null {
    return this._rpc;
  }

  set rpc(rpc: TaskSchedulerService | null) {
    this._rpc = rpc;
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
      this.search();
    }
    this._render();
  }

  private updateQuery() {
    const params: ParamSet = {};
    this.searchTerms.forEach((term: searchTerm) => {
      params[term.key] = [term.value];
    });
    const newUrl =
      window.location.href.split('?')[0] + '?' + fromParamSet(params);
    window.history.replaceState('', '', newUrl);
  }

  private search() {
    const req: SearchJobsRequest = {
      repoState: {
        repo: this.searchTerms.get('repo')?.value || '',
        revision: this.searchTerms.get('revision')?.value || '',
        patch: {
          issue: this.searchTerms.get('issue')?.value || '',
          patchset: this.searchTerms.get('patchset')?.value || '',
          server: '',
          patchRepo: '',
        },
      },
      buildbucketBuildId: parseInt(
        this.searchTerms.get('buildbucketBuildId')?.value || '0'
      ),
      name: this.searchTerms.get('name')?.value || '',
      status: (this.searchTerms.get('status')?.value || '') as JobStatus,
      // TODO(borenet): As written, the server can't distinguish between a
      // search for isForce == false and a search in which isForce is not set
      // because we don't care.
      isForce: this.searchTerms.get('isForce')?.value === 'true',
      timeEnd: this.searchTerms.get('timeEnd')?.value || '',
      timeStart: this.searchTerms.get('timeStart')?.value || '',
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
      this.cancel(job);
    });
  }
}

define('job-search-sk', JobSearchSk);
