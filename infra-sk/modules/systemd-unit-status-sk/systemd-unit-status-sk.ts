/**
 * @module infra-sk/modules/systemd-unit-status-sk
 * @description <h2><code>systemd-unit-status-sk</code></h2>
 *
 * @attr machine - The name of the machine the service is running on.
 *
 * @evt unit-action - An event triggered when the user wants to perform an action
 *        on the service. The detail of the event is of type SystemdUnitStatusSkEventDetail, e.g.:
 *
 * <pre>
 *  {
 *    machine: "skia-monitoring",
 *    name: "logserver.service",
 *    action: "start"
 *  }
 * </pre>
 */
import { define } from 'elements-sk/define'
import { html, render } from 'lit-html'

import { SystemdUnitStatus } from './json';

import 'elements-sk/styles/buttons'
import { upgradeProperty } from 'elements-sk/upgradeProperty'

import { diffDate } from 'common-sk/modules/human'

export interface SystemdUnitStatusSkEventDetail {
  machine: string,
  name: string,
  action: string,
}

export class SystemdUnitStatusSk extends HTMLElement {
  private static template = (ele: SystemdUnitStatusSk) => html`
    <button raised data-action="start"   data-name="${ele.value!.status!.Name}" @click=${ele.onClick}>Start  </button>
    <button raised data-action="stop"    data-name="${ele.value!.status!.Name}" @click=${ele.onClick}>Stop   </button>
    <button raised data-action="restart" data-name="${ele.value!.status!.Name}" @click=${ele.onClick}>Restart</button>
    <div class=uptime>${diffDate(ele.value!.props ? +ele.value!.props.ExecMainStartTimestamp/1000 : 'n/a')}</div>
    <div class="${ele.value!.status!.SubState} state">${ele.value!.status!.SubState}</div>
    <div class=service>${ele.value!.status!.Name}</div>
  `;

  private _value: SystemdUnitStatus | null = null;

  connectedCallback() {
    upgradeProperty(this, 'value');
    this.render();
  }

  get value(): SystemdUnitStatus | null { return this._value; }
  set value(val: SystemdUnitStatus | null) {
    this._value = val;
    this.render();
  }

  private render() {
    if (this.value) {
      render(SystemdUnitStatusSk.template(this), this, {eventContext: this});
    }
  }

  private onClick(e: MouseEvent) {
    const target = e.target as HTMLButtonElement;
    this.dispatchEvent(new CustomEvent<SystemdUnitStatusSkEventDetail>('unit-action', {
      detail: {
        machine: this.getAttribute('machine'),
        name: target.dataset.name,
        action: target.dataset.action,
      } as SystemdUnitStatusSkEventDetail,
      bubbles: true}
    ));
  }
}

define('systemd-unit-status-sk', SystemdUnitStatusSk);
