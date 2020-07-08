/**
 * @module modules/arb-scaffold-sk
 * @description <h2><code>arb-scaffold-sk</code></h2>
 *
 * Defines the overall structure for all Autoroll pages.
 */

import { html, render } from 'lit-html'

import { define } from 'elements-sk/define';
import 'elements-sk/icon/home-icon-sk';
import { upgradeProperty } from 'elements-sk/upgradeProperty'

import '../../../infra-sk/modules/app-sk';
import '../../../infra-sk/modules/login-sk';

const template = (ele: ARBScaffoldSk) => html`
<app-sk>
  <header>
    <h1>${ele.title}</h1>
    <login-sk></login-sk>
  </header>

  <aside>
    <nav>
      <a href="/" tab-index=0>
        <home-icon-sk></home-icon-sk><span>Home</span>
      </a>
    </nav>
  </aside>

  <main></main>

  <footer><error-toast-sk></error-toast-sk></footer>
</app-sk>
`;

class ARBScaffoldSk extends HTMLElement {
  private _title: string;

  constructor() {
    super();
    this._title = "";
  }

  get title() { return this._title; }
  set title(title: string) {
    this._title = title;
    this._render();
  }

  connectedCallback() {
    upgradeProperty(this, "title");
    this._render();
  }

  _render() {
    render(template(this), this, {eventContext: this});
  }
}

define('arb-scaffold-sk', ARBScaffoldSk);
