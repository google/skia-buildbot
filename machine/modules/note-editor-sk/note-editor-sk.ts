/**
 * @module modules/note-editor-sk
 * @description <h2><code>note-editor-sk</code></h2>
 *
 * Displays a dialog to edit an Annotation Message.
 */
import { $$ } from 'common-sk/modules/dom';
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import dialogPolyfill from 'dialog-polyfill';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { Annotation } from '../json';
import 'elements-sk/styles/buttons';
import '../theme/index';

const defaultAnnotation: Annotation = {
  Message: '',
  User: '',
  Timestamp: '',
};

export class NoteEditorSk extends ElementSk {
  private dialog: HTMLDialogElement | null = null;

  private annotation: Annotation = defaultAnnotation;

  private resolve: ((value: Annotation | undefined)=> void) | null = null;

  constructor() {
    super(NoteEditorSk.template);
  }

  private static template = (ele: NoteEditorSk) => html`
  <dialog>
    <label>
      Note:
      <input id=note type=text .value=${ele.annotation.Message}>
    </label>

    <div class=controls>
      <button @click=${ele.clearClick} id=clear>Clear</button>
      <button @click=${ele.cancelClick} id=cancel>Cancel</button>
      <button @click=${ele.okClick} id=ok>OK</button>
    </div>
  </dialog>
  `;

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
    this.dialog = this.querySelector<HTMLDialogElement>('dialog');
    dialogPolyfill.registerDialog(this.dialog!);
  }

  /**
   * Edits a copy of the given Annotation and on OK resolves to the edited
   * Annotation, and resolves to 'undefined' on Cancel.
   */
  edit(annotation: Annotation): Promise<Annotation | undefined> {
    return new Promise((resolve) => {
      this.resolve = resolve;
      this.annotation = Object.assign({}, annotation);
      this._render();
      // eslint-disable-next-line no-unused-expressions
      this.dialog?.showModal();
    });
  }

  private cancelClick() {
    if (!this.resolve) {
      return;
    }
    this.resolve(undefined);
    this.dialog!.close();
    this.resolve = null;
  }

  private okClick() {
    if (!this.resolve) {
      return;
    }
    this.annotation.Message = $$<HTMLInputElement>('#note', this)!.value;
    this.resolve(this.annotation);
    this.dialog!.close();
    this.resolve = null;
  }

  private clearClick() {
    if (!this.resolve) {
      return;
    }
    this.annotation.Message = '';
    this.resolve(this.annotation);
    this.dialog!.close();
    this.resolve = null;
  }
}

define('note-editor-sk', NoteEditorSk);
