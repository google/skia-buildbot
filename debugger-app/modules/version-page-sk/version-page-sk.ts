/**
 * @module modules/version-page-sk
 * @description <h2><code>version-page-sk</code></h2>
 *
 * Shows links to all older versions of the debugger (which do not automatically update).
 *
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import '../../../infra-sk/modules/theme-chooser-sk';
import '../../../infra-sk/modules/app-sk';

const urls = [
    "https://android-12-debugger.skia.org",
    "https://chrome-m100-debugger.skia.org",
    "https://chrome-m99-debugger.skia.org",
    "https://chrome-m98-debugger.skia.org",
    "https://chrome-m97-debugger.skia.org",
    "https://chrome-m96-debugger.skia.org",
    "https://chrome-m95-debugger.skia.org",
    "https://chrome-m94-debugger.skia.org",
    "https://chrome-m93-debugger.skia.org",
    "https://chrome-m92-debugger.skia.org",
    "https://chrome-m91-debugger.skia.org",
    "https://chrome-m90-debugger.skia.org",
];

const debuggerVersion = (url: string) => {
  return html`<li>
  <a href="${url}">${url}</a>
</li>`;
}

export class VersionPageSk extends ElementSk {
  constructor() {
    super(VersionPageSk.template);
  }

  private static template = (ele: VersionPageSk) => html`
    <app-sk>
      <header>
        <h2>Skia WASM Debugger Versions</h2>
        <span>
        <theme-chooser-sk></theme-chooser-sk>
      </span>
      </header>
      <main id=content>
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
