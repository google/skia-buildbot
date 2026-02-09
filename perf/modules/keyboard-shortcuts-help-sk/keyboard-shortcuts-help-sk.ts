/**
 * @module modules/keyboard-shortcuts-help-sk
 * @description A dialog that displays registered keyboard shortcuts.
 */
import { html, TemplateResult, LitElement } from 'lit';
import { customElement, property, query } from 'lit/decorators.js';
import { KeyboardShortcutHandler, ShortcutRegistry } from '../common/keyboard-shortcuts';
import '@material/web/dialog/dialog.js';
import { MdDialog } from '@material/web/dialog/dialog.js';

@customElement('keyboard-shortcuts-help-sk')
export class KeyboardShortcutsHelpSk extends LitElement {
  @query('#help-dialog') dialog!: MdDialog;

  @property({ attribute: false })
  handler: KeyboardShortcutHandler | null = null;

  createRenderRoot() {
    return this;
  }

  render() {
    return html`
      <md-dialog id="help-dialog">
        <div slot="headline">Keyboard Shortcuts</div>
        <div slot="content">
          <table class="shortcuts-table">
            ${this.renderShortcuts()}
          </table>
        </div>
        <div slot="actions">
          <button @click=${() => this.close()}>Close</button>
        </div>
      </md-dialog>
    `;
  }

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

  open() {
    this.requestUpdate();
    this.dialog?.show();
  }

  close() {
    this.dialog?.close();
  }
}
