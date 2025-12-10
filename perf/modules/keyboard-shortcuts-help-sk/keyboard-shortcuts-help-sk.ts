/**
 * @module modules/keyboard-shortcuts-help-sk
 * @description A dialog that displays registered keyboard shortcuts.
 */
import { html, TemplateResult } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { KeyboardShortcutHandler, ShortcutRegistry } from '../common/keyboard-shortcuts';
import '@material/web/dialog/dialog.js';
import { MdDialog } from '@material/web/dialog/dialog.js';

export class KeyboardShortcutsHelpSk extends ElementSk {
  private dialog: MdDialog | null = null;

  public handler: KeyboardShortcutHandler | null = null;

  constructor() {
    super(KeyboardShortcutsHelpSk.template);
  }

  private static template = (ele: KeyboardShortcutsHelpSk) => html`
    <md-dialog id="help-dialog">
      <div slot="headline">Keyboard Shortcuts</div>
      <div slot="content">
        <table class="shortcuts-table">
          ${ele.renderShortcuts()}
        </table>
      </div>
      <div slot="actions">
        <button @click=${() => ele.close()}>Close</button>
      </div>
    </md-dialog>
  `;

  private renderShortcuts() {
    const registry = ShortcutRegistry.getInstance();
    const shortcuts = registry.getShortcuts();
    const rows: TemplateResult[] = [];

    shortcuts.forEach((list, category) => {
      const relevantShortcuts = list.filter(
        (s) =>
          !this.handler ||
          !s.method ||
          typeof this.handler[s.method as keyof KeyboardShortcutHandler] === 'function'
      );

      if (relevantShortcuts.length > 0) {
        rows.push(html`
          <tr>
            <td colspan="2" class="category-header">${category}</td>
          </tr>
        `);
        relevantShortcuts.forEach((shortcut) => {
          rows.push(html`
            <tr>
              <td class="key-cell">${shortcut.key}</td>
              <td>${shortcut.description}</td>
            </tr>
          `);
        });
      }
    });

    return rows;
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
    this.dialog = this.querySelector('#help-dialog');
  }

  open() {
    this._render(); // Re-render to pick up any new shortcuts
    this.dialog?.show();
  }

  close() {
    this.dialog?.close();
  }
}

define('keyboard-shortcuts-help-sk', KeyboardShortcutsHelpSk);
