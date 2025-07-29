import { LitElement, html, css } from 'lit';
import { customElement, query, state } from 'lit/decorators.js';
import '@material/web/button/filled-button.js';
import '@material/web/button/outlined-button.js';
import '@material/web/textfield/outlined-text-field.js';
import '@material/web/dialog/dialog.js';
import '@material/web/icon/icon.js';
import '@material/web/tabs/tabs.js';
import '@material/web/tabs/primary-tab.js';
import '@material/web/select/outlined-select.js';
import '@material/web/select/select-option.js';

import type { MdDialog } from '@material/web/dialog/dialog';
import type { Tabs } from '@material/web/tabs/internal/tabs.js';
import type { MdOutlinedSelect } from '@material/web/select/outlined-select.js';
import { listBenchmarks, listBots, listStories } from '../../services/api';

/**
 * @element pinpoint-new-job-sk
 *
 * @description A modal for creating a new Pinpoint job.
 *
 */
@customElement('pinpoint-new-job-sk')
export class PinpointNewJobSk extends LitElement {
  static styles = css`
    md-dialog {
      max-width: 70vw;
      max-height: 90vh;
    }

    .modal-header {
      display: flex;
      justify-content: space-between;
      align-items: center;
    }

    .modal-content-container {
      padding-top: 16px;
    }

    .detailed-grid {
      display: grid;
      grid-template-columns: 2fr 1fr;
      gap: 24px;
      align-items: start;
    }

    .help-section {
      background-color: var(--md-sys-color-surface-container-lowest);
      border-radius: 12px;
      padding: 16px;
      font-size: 0.875rem;
      line-height: 1.4;
    }

    .help-section h3 {
      margin-top: 0;
      margin-bottom: 8px;
      font-size: 1rem;
      color: var(--md-sys-color-on-surface);
    }

    .help-section p {
      margin: 0 0 1.5em 0;
      color: var(--md-sys-color-on-surface-variant);
    }

    .help-section ul {
      padding-left: 20px;
      margin: 1em 0;
    }
    .help-section li {
      margin-bottom: 0.5em;
    }

    .form-section {
      display: flex;
      flex-direction: column;
      gap: 16px;
    }

    .form-section h2 {
      font-size: 1.2em;
      font-weight: 500;
      margin: 0;
      padding-bottom: 8px;
      border-bottom: 1px solid var(--md-sys-color-outline-variant);
    }

    .form-section h3 {
      font-size: 1em;
      font-weight: 500;
      margin: 8px 0 -8px 0;
      color: var(--md-sys-color-on-surface);
    }

    .form-section p {
      margin: 0 0 1em 0;
      color: var(--md-sys-color-on-surface-variant);
      font-size: 0.9em;
      line-height: 1.4;
    }

    .form-section ul {
      list-style: none;
      padding-left: 0;
      margin: 8px 0;
      color: var(--md-sys-color-on-surface-variant);
    }

    .form-section li {
      padding: 4px 0;
    }

    .form-section li b {
      color: var(--md-sys-color-on-surface);
      min-width: 120px;
      display: inline-block;
    }

    md-outlined-text-field,
    md-outlined-select {
      width: 100%;
    }

    .about-section {
      padding: 0 12px 24px 12px;
      color: var(--md-sys-color-on-surface-variant);
    }
    .about-section h3 {
      margin-top: 0;
      margin-bottom: 8px;
      font-size: 1.2em;
      font-weight: 500;
      color: var(--md-sys-color-on-surface);
    }

    .simplified-view {
      padding: 24px;
      text-align: center;
      color: var(--md-sys-color-on-surface-variant);
    }
  `;

  @query('md-dialog') private _dialog!: MdDialog;

  @state() private _activeTab: 'simplified' | 'detailed' = 'detailed';

  @state() private _benchmarks: string[] = [];

  @state() private _bots: string[] = [];

  @state() private _stories: string[] = [];

  @state() private _selectedBenchmark = '';

  @state() private _selectedBot = '';

  @state() private _selectedStory = '';

  @state() private _iterationCount = '10';

  @state() private _bugId = '';

  async connectedCallback() {
    super.connectedCallback();
    try {
      this._benchmarks = await listBenchmarks();
      this._bots = await listBots('');
    } catch (e) {
      console.error('Failed to load initial data for new job modal', e);
    }
  }

  public show() {
    this._dialog.show();
  }

  private close() {
    this._dialog.close();
  }

  private onTabChanged(e: CustomEvent) {
    const tabs = e.target as Tabs;
    this._activeTab = tabs.activeTabIndex === 0 ? 'detailed' : 'simplified';
  }

  private async onBenchmarkChanged(e: Event) {
    const select = e.target as MdOutlinedSelect;
    this._selectedBenchmark = select.value;
    this._selectedBot = '';
    this._selectedStory = '';
    this._stories = [];

    if (this._selectedBenchmark) {
      try {
        [this._bots, this._stories] = await Promise.all([
          listBots(this._selectedBenchmark),
          listStories(this._selectedBenchmark),
        ]);
      } catch (err) {
        console.error(`Failed to get bots or stories for ${this._selectedBenchmark}`, err);
        // Fallback to all bots, stories will be empty.
        this._bots = await listBots('');
      }
    } else {
      // No benchmark selected, get all bots and clear stories
      this._bots = await listBots('');
    }
  }

  private renderDetailedView() {
    return html`
      <div class="about-section">
        <h3>About your job</h3>
        <p>
          A Pinpoint job can either be a <b>bisection</b> to find a commit that caused a performance
          regression, or a <b>try job</b> to compare performance between two commits.
        </p>
      </div>
      <div class="detailed-grid">
        <div class="form-section">
          <h2>Select and customize your Chrome Build</h2>
          <h3>Base Commit</h3>
          <md-outlined-text-field
            label="Commit Hash"
            placeholder="Commit Hash"></md-outlined-text-field>
          <h3>Experimental Commit</h3>
          <md-outlined-text-field
            label="Commit Hash"
            placeholder="Commit Hash"></md-outlined-text-field>
        </div>
        <div class="help-section">
          <h3>Chrome Build</h3>
          <p>
            Provide two commit points (as git hashes) to define the range for the job. For a try
            job, this is the A/B comparison.
          </p>
        </div>

        <div class="form-section">
          <h2>Select and configure device and benchmark to test</h2>
          <md-outlined-select label="Benchmark" @change=${this.onBenchmarkChanged}>
            <md-select-option></md-select-option>
            ${this._benchmarks.map(
              (b) => html`<md-select-option .value=${b}>${b}</md-select-option>`
            )}
          </md-outlined-select>
          <md-outlined-select label="Device to test on" .value=${this._selectedBot}>
            ${this._bots.map((b) => html`<md-select-option .value=${b}>${b}</md-select-option>`)}
          </md-outlined-select>
          <md-outlined-select
            label="Story"
            .value=${this._selectedStory}
            ?disabled=${!this._selectedBenchmark}>
            ${this._stories.map((s) => html`<md-select-option .value=${s}>${s}</md-select-option>`)}
          </md-outlined-select>
          <md-outlined-text-field label="Story tags (optional)"></md-outlined-text-field>
        </div>
        <div class="help-section">
          <h3>Device and Benchmark</h3>
          <p>
            Specify the hardware and the performance test to run. Each benchmark can have multiple
            stories, which are specific scenarios to test.
          </p>
        </div>

        <div class="form-section">
          <h2>(Optional) Name your run and set iterations</h2>
          <md-outlined-text-field label="Job Name"></md-outlined-text-field>
          <md-outlined-text-field
            label="Iteration Count"
            type="number"
            .value=${this._iterationCount}
            @input=${(e: InputEvent) =>
              (this._iterationCount = (
                e.target as HTMLInputElement
              ).value)}></md-outlined-text-field
          ><md-outlined-text-field
            label="Bug ID (optional)"
            type="number"
            .value=${this._bugId}
            @input=${(e: InputEvent) =>
              (this._bugId = (e.target as HTMLInputElement).value)}></md-outlined-text-field>
        </div>
        <div class="help-section">
          <h3>Job Name, Iterations & Bug ID</h3>
          <p>
            Give your job a memorable name for easier identification later. If left blank, a name
            will be generated.
          </p>
          <p>
            The number of iterations to run the benchmark. Higher iterations usually yield more
            granular benchmark results. This value defaults to 10.
          </p>
          <p>
            If this job is related to a bug, you can provide the bug ID here for tracking purposes.
          </p>
        </div>
      </div>
    `;
  }

  private renderSimplifiedView() {
    return html`
      <div class="detailed-grid">
        <div class="form-section">
          <h2>1. Define Commit Range</h2>
          <p>
            A Pinpoint job can either be a <b>bisection</b> to find a commit that caused a
            performance regression, or a <b>try job</b> to compare performance between two commits.
            Provide two commit points to define the range for the job.
          </p>
          <h3>Base Commit</h3>
          <md-outlined-text-field
            label="Commit Hash"
            placeholder="Commit Hash"></md-outlined-text-field>
          <h3>Experimental Commit</h3>
          <md-outlined-text-field
            label="Commit Hash"
            placeholder="Commit Hash"></md-outlined-text-field>

          <h2>2. Review Test Configuration</h2>
          <p>
            This simplified flow uses a standard test configuration for general performance
            analysis. For custom settings, use the "Detailed" tab.
          </p>
          <ul>
            <li><b>Benchmark:</b> <span>speedometer3.crossbench</span></li>
            <li><b>Device:</b> <span>mac-m1-pro-perf</span></li>
            <li><b>Story:</b> <span>default</span></li>
            <li><b>Iteration Count:</b> <span>20</span></li>
          </ul>
        </div>
        <div class="help-section">
          <h3>What is Pinpoint?</h3>
          <p>
            Pinpoint is a performance testing tool for Chrome that helps diagnose regressions and
            evaluate performance changes. It automates the process of building Chrome at different
            revisions, running benchmarks, and comparing the results.
          </p>
          <p>
            You can run two main types of jobs:
            <ul>
              <li><b>Try Job:</b> An A/B test that compares performance between two specific commits.</li>
              <li><b>Bisection:</b> A binary search across a range of commits to automatically find the one that introduced a performance regression.</li>
            </ul>
          </p>
          <h3>Commit Range</h3>
          <p>
            Provide two commit points (as git hashes) to define the job's scope. For a try job, these are your A and B points. For a bisection, this is the range to search.
          </p>
          <h3>Test Configuration</h3>
          <p>
            This simplified view uses a common, pre-selected configuration for quick testing. The
            benchmark, device, and other parameters are fixed. If you need to test on different
            devices or run other benchmarks, please use the "Detailed" tab.
          </p>
        </div>
      </div>
    `;
  }

  render() {
    return html`
      <md-dialog>
        <div slot="headline" class="modal-header">
          <span>Start a Pinpoint Job</span>
        </div>
        <div slot="content">
          <md-tabs
            .activeIndex=${this._activeTab === 'detailed' ? 0 : 1}
            @change=${this.onTabChanged}>
            <md-primary-tab>Detailed</md-primary-tab>
            <md-primary-tab>Simplified</md-primary-tab>
          </md-tabs>
          <div class="modal-content-container">
            ${this._activeTab === 'detailed'
              ? this.renderDetailedView()
              : this.renderSimplifiedView()}
          </div>
        </div>
        <div slot="actions">
          <md-outlined-button @click=${this.close}>Cancel</md-outlined-button>
          <md-filled-button>Start</md-filled-button>
        </div>
      </md-dialog>
    `;
  }
}
