/**
 * @module modules/commit-detail-picker-sk
 * @description <h2><code>commit-detail-picker-sk</code></h2>
 *
 * @evt commit-selected - Emitted when a commit is selected. The details are of
 * the form CommitDetailPanelSkCommitSelectedDetails
 */

import '../commit-detail-panel-sk';
import 'elements-sk/styles/buttons';
import 'elements-sk/spinner-sk';
import '../day-range-sk';

import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import dialogPolyfill from 'dialog-polyfill';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { errorMessage } from '../errorMessage';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { Commit, CommitNumber, RangeRequest } from '../json';
import { CommitDetailPanelSkCommitSelectedDetails } from '../commit-detail-panel-sk/commit-detail-panel-sk';
import { DayRangeSkChangeDetail } from '../day-range-sk/day-range-sk';

const NO_COMMIT_SELECTED_MSG = 'Choose a commit.';

const initialRange = {
  begin: Math.floor(Date.now() / 1000 - 24 * 60 * 60),
  end: Math.floor(Date.now() / 1000),
};

export class CommitDetailPickerSk extends ElementSk {
  private range: DayRangeSkChangeDetail = initialRange;

  private _selection: CommitNumber = -1;

  private selected: number = -1;

  private details: Commit[];

  private dialog: HTMLDialogElement | null = null;

  private updatingCommits: boolean = false;

  constructor() {
    super(CommitDetailPickerSk.template);
    this.details = [];
  }

  private static _titleFrom = (ele: CommitDetailPickerSk) => {
    const index = ele.selected;
    if (index === -1) {
      return NO_COMMIT_SELECTED_MSG;
    }
    const d = ele.details[index];
    if (!d) {
      return NO_COMMIT_SELECTED_MSG;
    }
    return `${d.author} -  ${d.message}`;
  };

  private static template = (ele: CommitDetailPickerSk) => html`
    <button @click=${ele.open}>${CommitDetailPickerSk._titleFrom(ele)}</button>
    <dialog>
    <h2>Choose a commit</h2>
      <commit-detail-panel-sk
        @commit-selected="${ele.panelSelect}"
        .details="${ele.details}"
        selectable
        selected=${ele.selected}
      ></commit-detail-panel-sk>
      <button @click=${ele.close}>Close</button>

      <hr>
      <p class=tiny>Change the range of commits displayed:</p>
      <details>
        <summary>
          Date Range
        </summary>
        <day-range-sk
          id="range"
          @day-range-change=${ele.rangeChange}
          begin=${ele.range.begin}
          end=${ele.range.end}
        ></day-range-sk>
        <spinner-sk ?active=${ele.updatingCommits}></spinner-sk>
      </details>
    </dialog>
  `;

  connectedCallback(): void {
    super.connectedCallback();
    this._upgradeProperty('details');
    this._upgradeProperty('selection');
    this._render();
    this.dialog = this.querySelector('dialog')!;
    dialogPolyfill.registerDialog(this.dialog);
    this.updateCommitSelections();
  }

  attributeChangedCallback(): void {
    this._render();
  }

  private rangeChange(e: CustomEvent<DayRangeSkChangeDetail>) {
    this.range = e.detail;
    this.updateCommitSelections();
  }

  private async updateCommitSelections() {
    this.updatingCommits = true;
    this._render();

    const body: RangeRequest = {
      begin: this.range.begin,
      end: this.range.end,
      offset: this.selection,
    };

    try {
      const resp = await fetch('/_/cidRange/', {
        method: 'POST',
        body: JSON.stringify(body),
        headers: {
          'Content-Type': 'application/json',
        },
      });
      const cids = await jsonOrThrow(resp) as Commit[];
      cids.reverse();

      this.details = cids;

      for (let i = 0; i < cids.length; i++) {
        if (((cids[i] as unknown) as Commit).offset === this.selection) {
          this.selected = i;
          break;
        }
      }
      this.range.begin = cids[cids.length - 1].ts;
      this.range.end = cids[0].ts;
      this._render();
    } catch (error) {
      errorMessage(error);
    } finally {
      this.updatingCommits = false;
      this._render();
    }
  }

  private panelSelect(e: Event) {
    this.selected = (e as CustomEvent<
      CommitDetailPanelSkCommitSelectedDetails
    >).detail.selected;
    this._render();
    this.close();
  }

  private close() {
    this.dialog!.close();
    this._render();
  }

  private open() {
    this.dialog!.showModal();
    this._render();
  }

  /** The CommitNumber that is selected. */
  get selection(): CommitNumber { return this._selection; }

  set selection(val: CommitNumber) {
    this._selection = val;
    this.updateCommitSelections();
  }
}

define('commit-detail-picker-sk', CommitDetailPickerSk);
