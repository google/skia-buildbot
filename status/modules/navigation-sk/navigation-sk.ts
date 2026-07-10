/**
 * @module modules/navigation-sk
 * @description <h2><code>navigation-sk</code></h2>
 *
 * Element that offers navigation links for available pages.
 *
 */
import { html } from 'lit/html.js';
import { unsafeHTML } from 'lit/directives/unsafe-html.js';
import { define } from '../../../elements-sk/modules/define';

import { ElementSk } from '../../../infra-sk/modules/ElementSk';

import '../../../elements-sk/modules/icons/battery-charging-80-icon-sk';
import '../../../elements-sk/modules/icons/dashboard-icon-sk';
import '../../../elements-sk/modules/icons/devices-icon-sk';

export class NavigationSk extends ElementSk {
  private static template = () => html`
    <div class="table">
      ${NavigationSk.link('/', 'dashboard', 'Status Tree')}
      ${NavigationSk.link('https://goto.google.com/skbl', 'devices', 'Swarming Bots')}
      ${NavigationSk.link('/capacity', 'battery-charging-80', 'Capacity Stats')}
      ${NavigationSk.link('/orphaned-tasks-machines', 'dashboard', 'Orphaned Tasks/Machines')}
    </div>
  `;

  private static link = (url: string, icon: string, text: string) =>
    window.location.pathname === url
      ? html``
      : html`
          <a class="tr" href="${url}">
            <span class="td">
              ${unsafeHTML(`<${icon}-icon-sk class="icon"></${icon}-icon-sk>`)} ${text}
            </span>
          </a>
        `;

  constructor() {
    super(NavigationSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }
}

define('navigation-sk', NavigationSk);
