/**
 * @module modules/textarea-numbers-sk
 * @description <h2><code>textarea-numbers-sk</code></h2>
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
import CodeMirror from 'codemirror';
import 'codemirror/mode/clike/clike';

export class TextareaNumbersSk extends ElementSk {
  private static defaultContent = `  void draw(SkCanvas* canvas) {
    SkPaint p;
    p.setColor(SK_ColorRED);
    p.setAntiAlias(true);
    p.setStyle(SkPaint::kStroke_Style);
    p.setStrokeWidth(10);

    canvas->drawLine(20, 20, 100, 100, p);
}`;

  private static template = (ele: TextareaNumbersSk) => html``;

  private codeMirror: CodeMirror.Editor | null = null;

  constructor() {
    super(TextareaNumbersSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
    this.codeMirror = CodeMirror(this, {
      value: TextareaNumbersSk.defaultContent,
      lineNumbers: true,
      mode: 'text/x-c++src',
      theme: 'base16-dark',
    });
  }
}

define('textarea-numbers-sk', TextareaNumbersSk);
