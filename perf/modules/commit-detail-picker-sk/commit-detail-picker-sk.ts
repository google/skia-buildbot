/**
 * @module modules/commit-detail-picker-sk
 * @description <h2><code>commit-detail-picker-sk</code></h2>
 *
 * @evt commit-selected - Event produced when a commit is selected. The
 *     the event detail contains the serialized cid.CommitDetail.
 *
 *      {
 *        author: "foo (foo@example.org)",
 *        url: "skia.googlesource.com/bar",
 *        message: "Commit from foo.",
 *        ts: 1439649751,
 *      },
 *
 * @attr {Number} selected - The index of the selected commit.
 *
 */

import '../commit-detail-panel-sk';
import 'elements-sk/styles/buttons';

import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import dialogPolyfill from 'dialog-polyfill';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { Commit } from '../json';
import { CommitDetailPanelSkCommitSelectedDetails } from '../commit-detail-panel-sk/commit-detail-panel-sk';

const NO_COMMIT_SELECTED_MSG = 'Choose a commit.';

export class CommitDetailPickerSk extends ElementSk {
  private _details: Commit[];

  private _dialog: HTMLDialogElement | null = null;

  constructor() {
    super(CommitDetailPickerSk.template);
    this._details = [];
  }

  private static _titleFrom = (ele: CommitDetailPickerSk) => {
    const index = ele.selected;
    if (index === -1) {
      return NO_COMMIT_SELECTED_MSG;
    }
    const d = ele._details[index];
    if (!d) {
      return NO_COMMIT_SELECTED_MSG;
    }
    return `${d.author} -  ${d.message}`;
  };

  private static template = (ele: CommitDetailPickerSk) => html`
    <button @click=${ele._open}>${CommitDetailPickerSk._titleFrom(ele)}</button>
    <dialog>
      <commit-detail-panel-sk
        @commit-selected="${ele._panelSelect}"
        .details="${ele._details}"
        selectable
        selected=${ele.selected}
      ></commit-detail-panel-sk>
      <button @click=${ele._close}>Close</button>
    </dialog>
  `;


  connectedCallback(): void {
    super.connectedCallback();
    this._upgradeProperty('details');
    this._render();
    this._dialog = this.querySelector('dialog')!;
    dialogPolyfill.registerDialog(this._dialog);
  }

  attributeChangedCallback(): void {
    this._render();
  }

  private _panelSelect(e: Event) {
    this.selected = (e as CustomEvent<
      CommitDetailPanelSkCommitSelectedDetails
    >).detail.selected;
    this._render();
  }

  private _close() {
    this._dialog!.close();
    this._render();
  }

  private _open() {
    this._dialog!.showModal();
    this._render();
  }

  static get observedAttributes(): string[] {
    return ['selected'];
  }

  /** Mirrors the selected attribute. */
  get selected(): number {
    return +(this.getAttribute('selected') || '-1');
  }

  set selected(val: number) {
    this.setAttribute('selected', `${val}`);
  }


  /** An array of serialized cid.CommitDetail. */
  get details(): Commit[] {
    return this._details;
  }

  set details(val: Commit[]) {
    this._details = val;
    this._render();
  }
}

define('commit-detail-picker-sk', CommitDetailPickerSk);
