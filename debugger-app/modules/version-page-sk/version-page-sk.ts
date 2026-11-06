/**
 * @module modules/version-page-sk
 * @description <h2><code>version-page-sk</code></h2>
 *
 * Shows links to all older versions of the debugger (which do not automatically update).
 *
 */
import { html } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import '../../../infra-sk/modules/theme-chooser-sk';
import '../../../infra-sk/modules/app-sk';

const urls: string[] = [];

const debuggerVersion = (url: string) =>
  html`<li>
    <a href="${url}">${url}</a>
  </li>`;

export class VersionPageSk extends ElementSk {
  constructor() {
    super(VersionPageSk.template);
  }

  private static template = () =>
    html` <app-sk>
      <header>
        <h2>Skia WASM Debugger Versions</h2>
        <span>
          <theme-chooser-sk></theme-chooser-sk>
        </span>
      </header>
      <main id="content">
        <ul>
          ${urls.map((u: string) => debuggerVersion(u))}
        </ul>
      </main>
    </app-sk>`;

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
  }
}

define('version-page-sk', VersionPageSk);
