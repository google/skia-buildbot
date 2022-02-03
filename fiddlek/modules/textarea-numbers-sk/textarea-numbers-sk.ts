/**
 * @module modules/textarea-numbers-sk
 * @description <h2><code>textarea-numbers-sk</code></h2>
 *
 * A code editor element with numbers and the ability to mark lines that contain
 * errors.
 */
import 'codemirror5/addon/fold/foldcode';
import 'codemirror5/addon/fold/foldgutter';
import 'codemirror5/lib/codemirror';
import 'codemirror5/mode/clike/clike'; // Syntax highlighting for c-like languages.
import { define } from 'elements-sk/define';
import CodeMirror from 'codemirror5';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { isDarkMode } from '../../../infra-sk/modules/theme-chooser-sk/theme-chooser-sk';

const FOLDABLE_BLOCK_START = '// SK_FOLD_START';
const FOLDABLE_BLOCK_END = '// SK_FOLD_END';

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
      extraKeys: {
        // Keyboard shortcuts for adding fold block tokens.
        'Ctrl-S': function(cm) { cm.replaceRange(FOLDABLE_BLOCK_START, cm.getCursor()); },
        'Ctrl-E': function(cm) { cm.replaceRange(FOLDABLE_BLOCK_END, cm.getCursor()); },
      },
      foldGutter: true,
      gutters: ['CodeMirror-linenumbers', 'CodeMirror-foldgutter'],
      foldOptions: {
        // Looks for FOLDABLE_BLOCK_START in the current line and then scans
        // the remaining code to find a corresponding FOLDABLE_BLOCK_END.
        rangeFinder: (cm: CodeMirror.Editor, pos: CodeMirror.Position): CodeMirror.FoldRange | undefined => {
          const startLineText = cm.getLine(pos.line);
          const blockStartIndex = startLineText.indexOf(FOLDABLE_BLOCK_START);
          if (blockStartIndex === -1) {
            // We did not find a start block. Return empty values.
            return undefined;
          }

          // We found the start block now let's look for the end block.
          let blockEndLine = -1;
          let countOfNestedBlocks = 0;
          for (let i = pos.line + 1, end = cm.lastLine(); i <= end; ++i) {
            const lineText = cm.getLine(i);
            if (lineText.includes(FOLDABLE_BLOCK_START)) {
              // We found another start block. There might be nested blocks here.
              countOfNestedBlocks++;
            }
            if (lineText.includes(FOLDABLE_BLOCK_END)) {
              if (countOfNestedBlocks > 0) {
                // We found an end block which is part of a nested block.
                countOfNestedBlocks--;
              } else if (countOfNestedBlocks === 0) {
                // We found the end block of the block that we started off with.
                blockEndLine = i;
                break;
              }
            }
          }

          if (blockEndLine === -1) {
            // We did not find a matching end block. Return empty values.
            return undefined;
          }
          return {
            from: CodeMirror.Pos(pos.line, blockStartIndex),
            to: CodeMirror.Pos(blockEndLine, cm.getLine(blockEndLine).length),
          };
        },
        // Creates a fold widget that contains the count of total folded lines.
        widget: (from: CodeMirror.Position, to: CodeMirror.Position): string => {
          const content = this.codeMirror!.getRange(from, to);
          const count = content.trim().split('\n').length;
          return `${count}...`;
        },
      },
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
      // When we assign a value, automatically fold all blocks.
      CodeMirror.commands.foldAll(this.codeMirror);
    }
    this.clearErrors();
  }
}

define('textarea-numbers-sk', TextareaNumbersSk);
