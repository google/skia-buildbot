/**
 * @module modules/note-editor-sk
 * @description <h2><code>note-editor-sk</code></h2>
 *
 * Displays a dialog to edit an Annotation Message.
 */
import { html } from 'lit/html.js';
import { $$ } from '../../../infra-sk/modules/dom';
import { define } from '../../../elements-sk/modules/define';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { Annotation } from '../json';

const defaultAnnotation: Annotation = {
  Message: '',
  User: '',
  Timestamp: '',
};

export class NoteEditorSk extends ElementSk {
  private dialog: HTMLDialogElement | null = null;

  private annotation: Annotation = defaultAnnotation;

  private resolve: ((value: Annotation | undefined) => void) | null = null;

  constructor() {
    super(NoteEditorSk.template);
  }

  private static template = (ele: NoteEditorSk) => html`
    <dialog>
      <label>
        Note:
        <input id="note" type="text" .value=${ele.annotation.Message} />
      </label>

      <div class="controls">
        <button @click=${ele.clearClick} id="clear">Clear</button>
        <button @click=${ele.cancelClick} id="cancel">Cancel</button>
        <button @click=${ele.okClick} id="ok">OK</button>
      </div>
    </dialog>
  `;

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
    this.dialog = this.querySelector<HTMLDialogElement>('dialog');
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
      this.dialog!.showModal();
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
