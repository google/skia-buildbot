/**
 * @module modules/note-editor-sk
 * @description <h2><code>note-editor-sk</code></h2>
 *
 * @evt
 *
 * @attr
 *
 * @example
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

export class NoteEditorSk extends ElementSk {
  private static template = (ele: NoteEditorSk) =>
    html`<h3>Hello world</h3>`;

  constructor() {
    super(NoteEditorSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }
};

define('note-editor-sk', NoteEditorSk);
