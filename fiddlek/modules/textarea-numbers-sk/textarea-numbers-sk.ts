/**
 * @module modules/textarea-numbers-sk
 * @description <h2><code>textarea-numbers-sk</code></h2>
 *
 * A code editor element with numbers and the ability to mark lines that contain
 * errors.
 */
import 'codemirror/mode/clike/clike'; // Syntax highlighting for c-like languages.
import { define } from 'elements-sk/define';
import CodeMirror from 'codemirror';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { isDarkMode } from '../../../infra-sk/modules/theme-chooser-sk/theme-chooser-sk';

export class TextareaNumbersSk extends ElementSk {
  /** The CodeMirror control. */
  private codeMirror: CodeMirror.Editor | null = null;

  /** All the lines we've marked as having errors. */
  private errorLines: CodeMirror.TextMarker[] = [];

  /** Returns the CodeMirror theme based on the state of the page's darkmode.
   *
   * For this to work the associated CSS themes must be loaded. See
   * textarea-numbers-sk.scss.
   */
  private static themeFromCurrentMode = () => (isDarkMode() ? 'base16-dark' : 'base16-light');

  connectedCallback(): void {
    super.connectedCallback();

    // Creates and attaches the CodeMirror control as this elements only child.
    // Note we don't call _render().
    this.codeMirror = CodeMirror(this, {
      lineNumbers: true,
      mode: 'text/x-c++src',
      theme: TextareaNumbersSk.themeFromCurrentMode(),
      viewportMargin: Infinity,
    });

    this._upgradeProperty('value');

    // Listen for theme changes.
    document.addEventListener('theme-chooser-toggle', () => {
      this.codeMirror!.setOption(
        'theme',
        TextareaNumbersSk.themeFromCurrentMode(),
      );
    });
  }

  /** Removes all error line annotations. */
  clearErrors(): void {
    this.errorLines.forEach((textMarker) => {
      textMarker.clear();
    });
  }

  /** Indicates there is an error on line n. */
  setErrorLine(n: number): void {
    // Set the class of that line to 'cm-error'.
    this.errorLines.push(
      this.codeMirror!.markText(
        { line: n - 1, ch: 0 },
        { line: n - 1, ch: 200 }, // Some large number for the character offset.
        {
          className: 'cm-error', // See the base16-dark.css file in CodeMirror for the class name.
        },
      )!,
    );
  }

  /** Move the cursor to the given row and column. */
  setCursor(row: number, col: number): void {
    this.codeMirror!.focus();
    this.codeMirror!.setCursor({ line: row - 1, ch: col - 1 });
  }

  /** @prop The text content of the edit box. */
  get value(): string {
    if (!this.codeMirror) {
      return '';
    }
    return this.codeMirror!.getValue();
  }

  set value(val: string) {
    if (this.codeMirror) {
      this.codeMirror!.setValue(val);
    }
    this.clearErrors();
  }
}

define('textarea-numbers-sk', TextareaNumbersSk);
