/**
 * @module infra-sk/modules/systemd-unit-status-sk
 * @description <h2><code>systemd-unit-status-sk</code></h2>
 *
 * @attr machine - The name of the machine the service is running on.
 *
 * @evt unit-action - An event triggered when the user wants to perform an action
 *        on the service. The detail of the event has the form:
 *
 * @prop {Object} value - Expected to be a systemd.UnitStatus.
 *
 * <pre>
 *  {
 *    machine: "skia-monitoring",
 *    name: "logserver.service",
 *    action: "start"
 *  }
 * </pre>
 */
import { html, render } from 'lit-html'

import 'elements-sk/styles/buttons'
import { upgradeProperty } from 'elements-sk/upgradeProperty'

import { diffDate } from 'common-sk/modules/human'

const template = (ele) => html`
  <button raised data-action="start"   data-name="${ele.value.status.Name}" @click=${ele._click}>Start  </button>
  <button raised data-action="stop"    data-name="${ele.value.status.Name}" @click=${ele._click}>Stop   </button>
  <button raised data-action="restart" data-name="${ele.value.status.Name}" @click=${ele._click}>Restart</button>
  <div class=uptime>${diffDate(ele.value.props ? +ele.value.props.ExecMainStartTimestamp/1000 : 'n/a')}</div>
  <div class="${ele.value.status.SubState} state">${ele.value.status.SubState}</div>
  <div class=service>${ele.value.status.Name}</div>
`;

window.customElements.define('systemd-unit-status-sk', class extends HTMLElement {
  constructor() {
    super();
    this._value = null;
  }

  connectedCallback() {
    upgradeProperty(this, 'value');
    this._render(this);
  }

  get value() { return this._value; }
  set value(val) {
    this._value = val;
    this._render(this);
  }

  _render() {
    if (this.value) {
      render(template(this), this, {eventContext: this});
    }
  }

  _click(e) {
    this.dispatchEvent(new CustomEvent('unit-action', {
      detail: {
        machine: this.getAttribute('machine'),
        name: e.target.dataset.name,
        action: e.target.dataset.action,
      },
      bubbles: true}
    ));
  }
});
