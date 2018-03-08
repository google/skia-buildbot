/** @module common/systemd-unit-status-sk */
import { html, render } from 'lit-html/lib/lit-extended'

import 'skia-elements/buttons'
import { upgradeProperty } from 'skia-elements/upgradeProperty'

import { diffDate } from 'common/human'

const template = (ele) => html`
  <button raised data-action="start"   data-name$="${ele.value.status.Name}" on-click=${e => ele._click(e)}>Start  </button>
  <button raised data-action="stop"    data-name$="${ele.value.status.Name}" on-click=${e => ele._click(e)}>Stop   </button>
  <button raised data-action="restart" data-name$="${ele.value.status.Name}" on-click=${e => ele._click(e)}>Restart</button>
  <div class=uptime>${diffDate(ele.value.props ? +ele.value.props.ExecMainStartTimestamp/1000 : 'n/a')}</div>
  <div class$="${ele.value.status.SubState + ' state'}">${ele.value.status.SubState}</div>
  <div class=service>${ele.value.status.Name}</div>
`;

/** <code>systemd-unit-status-sk</code>
 *
 * @attr machine - The name of the machine the service is running on.
 *
 * @evt unit-action - An event triggered when the user wants to perform an action
 *        on the service. The detail of the event has the form:
 *
 * <pre>
 *  {
 *    machine: "skia-monitoring",
 *    name: "logserver.service",
 *    action: "start"
 *  }
 * </pre>
 */
class SystemUnitStatusSk extends HTMLElement {
  constructor() {
    super();
    this._value = null;
  }

  connectedCallback() {
    upgradeProperty(this, 'value');
    this._render(this);
  }

  /** @prop {Object} value - Expected to be a systemd.UnitStatus. */
  get value() { return this._value; }
  set value(val) {
    this._value = val;
    this._render(this);
  }

  _render() {
    if (this.value) {
      render(template(this), this);
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
}

window.customElements.define('systemd-unit-status-sk', SystemUnitStatusSk);
