/**
 * @module modules/commit-detail-picker-sk
 * @description <h2><code>commit-detail-picker-sk</code></h2>
 *
 * @evt commit-selected - Emitted when a commit is selected. The details are of
 * the form CommitDetailPanelSkCommitSelectedDetails
 */

import '../commit-detail-panel-sk';
import '../../../elements-sk/modules/spinner-sk';
import '../day-range-sk';

import { html, LitElement } from 'lit';
import { customElement, property, state, query } from 'lit/decorators.js';
import { Task, TaskStatus } from '@lit/task';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { errorMessage } from '../errorMessage';
import { Commit, CommitNumber, RangeRequest } from '../json';
import { CommitDetailPanelSkCommitSelectedDetails } from '../commit-detail-panel-sk/commit-detail-panel-sk';
import { DayRangeSkChangeDetail } from '../day-range-sk/day-range-sk';

const NO_COMMIT_SELECTED_MSG = 'Choose a commit.';

const initialRange = {
  begin: Math.floor(Date.now() / 1000 - 24 * 60 * 60),
  end: Math.floor(Date.now() / 1000),
};

@customElement('commit-detail-picker-sk')
export class CommitDetailPickerSk extends LitElement {
  @state()
  private range: DayRangeSkChangeDetail = initialRange;

  @property({ type: Number })
  selection: CommitNumber = CommitNumber(-1);

  @state()
  private selected: number = -1;

  @state()
  private _fetchTrigger = 0;

  @query('dialog')
  private dialog!: HTMLDialogElement;

  private _commitTask = new Task<[number, CommitNumber], Commit[]>(this, {
    task: async ([_trigger, _selection_arg]: [number, CommitNumber]) => {
      try {
        const body: RangeRequest = {
          begin: this.range.begin,
          end: this.range.end,
          offset: this.selection,
        };

        const resp = await fetch('/_/cidRange/', {
          method: 'POST',
          body: JSON.stringify(body),
          headers: {
            'Content-Type': 'application/json',
          },
        });
        const cids = (await jsonOrThrow(resp)) as Commit[];
        cids.reverse();

        for (let i = 0; i < cids.length; i++) {
          if ((cids[i] as unknown as Commit).offset === this.selection) {
            this.selected = i;
            break;
          }
        }

        if (cids.length > 0) {
          this.range = {
            ...this.range,
            begin: cids[cids.length - 1].ts,
            end: cids[0].ts,
          };
        }

        return cids;
      } catch (error: any) {
        errorMessage(error);
        throw error;
      }
    },
    args: () => [this._fetchTrigger, this.selection],
  });

  private get _title() {
    const index = this.selected;
    if (index === -1) {
      return NO_COMMIT_SELECTED_MSG;
    }
    const d = (this._commitTask.value as Commit[])?.[index];
    if (!d) {
      return NO_COMMIT_SELECTED_MSG;
    }
    return `${d.author} -  ${d.message}`;
  }

  createRenderRoot() {
    return this;
  }

  render() {
    return html`
      <button @click=${this.open}>${this._title}</button>
      <dialog>
        <h2>Choose a commit</h2>
        <commit-detail-panel-sk
          @commit-selected="${this.panelSelect}"
          .details="${this._commitTask.value ?? []}"
          selectable
          .selected=${this.selected}></commit-detail-panel-sk>
        <button class="close-dialog" @click=${this.close}>Close</button>

        <hr />
        <p class="tiny">Change the range of commits displayed:</p>
        <details>
          <summary>Date Range</summary>
          <day-range-sk
            id="range"
            @day-range-change=${this.rangeChange}
            .begin=${this.range.begin}
            .end=${this.range.end}></day-range-sk>
          <spinner-sk ?active=${this._commitTask.status === TaskStatus.PENDING}></spinner-sk>
        </details>
      </dialog>
    `;
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._fetchTrigger++;
  }

  private rangeChange(e: CustomEvent<DayRangeSkChangeDetail>) {
    this.range = e.detail;
    this._fetchTrigger++;
  }

  private panelSelect(e: Event) {
    this.selected = (e as CustomEvent<CommitDetailPanelSkCommitSelectedDetails>).detail.selected;
    this.close();
  }

  private close() {
    this.dialog.close();
  }

  private open() {
    this.dialog.showModal();
  }
}
