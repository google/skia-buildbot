/**
 * @module modules/navigation-sk
 * @description <h2><code>navigation-sk</code></h2>
 *
 * Element that offers navigation links for available pages.
 *
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';

import { ElementSk } from '../../../infra-sk/modules/ElementSk';

import 'elements-sk/icon/battery-charging-80-icon-sk';
import 'elements-sk/icon/dashboard-icon-sk';
import 'elements-sk/icon/devices-icon-sk';

export class NavigationSk extends ElementSk {
  private static template = (el: NavigationSk) =>
    html` <div class="table">
      <a class="tr" href="https://goto.google.com/skbl">
        <span class="td">
          <devices-icon-sk class="icon"></devices-icon-sk> Swarming Bots
        </span>
      </a>
      <a class="tr" href="/capacity">
        <span class="td">
          <battery-charging-80-icon-sk
            class="icon"
          ></battery-charging-80-icon-sk>
          Capacity Stats
        </span>
      </a>
    </div>`;

  constructor() {
    super(NavigationSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }
}

define('navigation-sk', NavigationSk);
