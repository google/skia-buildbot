/**
 * @module modules/textarea-numbers-sk
 * @description <h2><code>textarea-numbers-sk</code></h2>
 *
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import CodeMirror from 'codemirror';
import 'codemirror/mode/clike/clike';
import { isDarkMode } from '../../../infra-sk/modules/theme-chooser-sk/theme-chooser-sk';

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

  private errorLines: CodeMirror.TextMarker[] = [];

  private static themeFromCurrentMode = () =>
    isDarkMode() ? 'base16-dark' : 'base16-light';

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
      theme: TextareaNumbersSk.themeFromCurrentMode(),
    });

    this._upgradeProperty('value');

    document.addEventListener('theme-chooser-toggle', (e) => {
      this.codeMirror?.setOption(
        'theme',
        TextareaNumbersSk.themeFromCurrentMode()
      );
    });
  }

  /** Removes all error line annotations. */
  clearErrors() {
    this.errorLines.forEach((textMarker) => {
      textMarker.clear();
    });
  }

  /** Indicates there is an error on line n. */
  setErrorLine(n: number) {
    // Set the class of that line to 'cm-error'.
    this.errorLines.push(
      this.codeMirror?.markText(
        { line: n, ch: 0 },
        { line: n, ch: 200 }, // Some large character number.
        {
          className: 'cm-error',
        }
      )!
    );
  }

  /** Indicates there is an error at    row and column. */
  setCursor(row: number, col: number) {
    this.codeMirror?.setCursor(row, col);
  }

  /** @prop value {string} The text content of the edit box. */
  get value() {
    return this.codeMirror!.getValue();
  }

  set value(val: string) {
    this.codeMirror!.setValue(val);
    this.clearErrors();
  }
}

define('textarea-numbers-sk', TextareaNumbersSk);
