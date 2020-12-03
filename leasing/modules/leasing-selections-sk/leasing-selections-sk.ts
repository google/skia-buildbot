/**
 * @module module/leasing-selections-sk
 * @description <h2><code>leasing-selections-sk</code></h2>
 *
 * <p>
 *   Contains the selections the user needs to make to schedule a leasing
 *   swarming task.
 * </p>
 *
 */

import { define } from 'elements-sk/define';
import { html, TemplateResult } from 'lit-html';
import { $$ } from 'common-sk/modules/dom';

import 'elements-sk/error-toast-sk';
import 'elements-sk/icon/folder-icon-sk';
import 'elements-sk/icon/gesture-icon-sk';
import 'elements-sk/icon/help-icon-sk';
import 'elements-sk/icon/home-icon-sk';
import 'elements-sk/icon/star-icon-sk';
import 'elements-sk/nav-button-sk';
import 'elements-sk/nav-links-sk';
import 'elements-sk/select-sk';
import 'elements-sk/styles/select';
import { errorMessage } from 'elements-sk/errorMessage';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { device, getAKAStr, doImpl } from '../leasing';

import '../../../infra-sk/modules/login-sk';

import { Task, PoolDetails } from '../json';

export class LeasingSelectionsSk extends ElementSk {
  private _all_pools: string[] = ['Skia'];

  private _pool: string = 'Skia';

  private _osTypes: { [key: string]: number; } = {};

  private _deviceTypes: Record<string, string> = {};

  private _selectedDeviceType: string = '';

  private _selectedOsType: string = '';

  private _osToDeviceTypes: { [key: string]: { [key: string]: number; }; } = {};

  private _loadingDetails: boolean = false;


  constructor() {
    super(LeasingSelectionsSk.template);

    this._fetchSupportedPools();
    this._fetchOsDetails();
  }

  private static template = (ele: LeasingSelectionsSk) => html`
    <table class="options panel">

      <tr>
        <td class="step-title">Select Pool</td>
        <td>
          <select id="pool" ?disabled=${ele._loadingDetails} .selection=${ele._pool} @input=${ele._poolChanged}>
            ${LeasingSelectionsSk._displayPools(ele)}
          </select>
        </td>
      </tr>

      <tr>
        <td class="step-title">Select OS Type</td>
        <td>
          <select id="os_type" ?disabled=${ele._loadingDetails} @input=${ele._osTypeChanged}>
            ${LeasingSelectionsSk._displayOsTypes(ele)}
          </select>
        </td>
      </tr>

      <tr>
        <td class="step-title">Select Device Type</td>
        <td>
          <select id="device_type" ?disabled=${ele._loadingDetails || (ele._selectedOsType !== 'Android' && !ele._selectedOsType.startsWith('iOS'))}>
            ${LeasingSelectionsSk._displayDeviceTypes(ele)}
          </select>
        </td>
      </tr>

      <tr>
        <td class="step-title">Specify BotId (optional)</td>
        <td>
          <input id="bot_id" ?disabled=${ele._loadingDetails}></input>
          <br/>
          <span class="smaller-font">Note: OS Type and Device Type are ignored if this is populated</span>
        </td>
      </tr>

      <tr>
        <td class="step-title">Specify Swarming Task Id<br/>to keep artifacts ready on bot<br/>(optional)</td>
        <td>
          <input id="task_id" ?disabled=${ele._loadingDetails}></input>
        </td>
      </tr>

      <tr>
        <td class="step-title">Lease Duration</td>
        <td>
          <select id="duration" ?disabled=${ele._loadingDetails}>
            <option value="1" title="1 Hour">1hr</option>
            <option value="2" title="2 Hours">2hr</option>
            <option value="6" title="6 Hours">6hr</option>
            <option value="23" title="23 Hours">23hr</option>
          </select>
        </td>

      <tr>
        <td class="step-title">Description</td>
        <td>
          <input id="desc" ?disabled=${ele._loadingDetails} placeholder="Description is required"></input>
        </td>
      </tr>

      <tr>
        <td colspan="2" class="center">
          <button raised @click=${ele._addTask} ?disabled=${ele._loadingDetails}>Lease Bot</button>
        </td>
      </tr>

    </table>
`;

  private static _displayPools(ele: LeasingSelectionsSk): TemplateResult[] {
    return ele._all_pools.map((p) => html`
    <option
      ?selected=${ele._pool === p}
      value=${p}
      title=${p}
      >${p}
    </option>`);
  }

  private static _displayOsTypes(ele: LeasingSelectionsSk): TemplateResult[] {
    if (ele._osTypes === {}) {
      return [html``];
    }
    return Object.keys(ele._osTypes).map((o) => html`
    <option
      value=${o}
      title=${o}
      >${o} - ${ele._osTypes[o]} bots online
    </option>`);
  }

  private static _displayDeviceTypes(ele: LeasingSelectionsSk): TemplateResult[] {
    if (ele._osToDeviceTypes === {}) {
      return [html``];
    }
    if (!(ele._selectedOsType in ele._osToDeviceTypes)) {
      return [html``];
    }
    const deviceTypes = ele._osToDeviceTypes[ele._selectedOsType];
    return Object.keys(deviceTypes).map((d) => html`
    <option
      value=${d}
      title=${d}
      >${device(d)} ${getAKAStr(d)} - ${deviceTypes[d]} bots online
    </option>`);
  }

  _poolChanged(e: InputEvent): void {
    this._pool = (e.target! as HTMLInputElement).value;
    this._fetchOsDetails();
  }

  _osTypeChanged(e: InputEvent): void {
    this._selectedOsType = (e.target! as HTMLInputElement).value;
    this._render();
  }

  _fetchSupportedPools(): void {
    doImpl('/_/get_supported_pools', {}, (json) => {
      this._all_pools = json;
      this._render();
    });
  }

  _fetchOsDetails(): void {
    doImpl(`/_/pooldetails?pool=${this._pool}`, {}, (json: PoolDetails) => {
      this._osTypes = json.os_types;
      this._osToDeviceTypes = json.os_to_device_types;
      // Select first OsType.
      this._selectedOsType = Object.keys(this._osTypes)[0];
      this._render();
    });
  }

  _addTask(): void {
    const pool = ($$('#pool', this) as HTMLInputElement).value;
    const osType = ($$('#os_type', this) as HTMLInputElement).value;
    const deviceType = ($$('#device_type', this) as HTMLInputElement).value;
    const botId = ($$('#bot_id', this) as HTMLInputElement).value;
    const taskId = ($$('#task_id', this) as HTMLInputElement).value;
    const duration = ($$('#duration', this) as HTMLInputElement).value;
    const desc = ($$('#desc', this) as HTMLInputElement).value;

    // Validate inputs.
    if (!desc) {
      errorMessage('Please specify a description');
      return;
    }

    // Confirm.
    const confirmed = window.confirm('Proceed with adding leasing task?');
    if (!confirmed) {
      return;
    }

    // Call backend to add task.
    const detail = {} as Task;
    detail.pool = pool;
    detail.botId = botId;
    if (!botId) {
      detail.osType = osType;
      if (deviceType) {
        detail.deviceType = deviceType;
      }
    }
    detail.taskIdForIsolates = taskId;
    detail.duration = duration;
    detail.description = desc;

    this._loadingDetails = true;
    this._render();
    doImpl('/_/add_leasing_task', detail, () => {
      window.location.href = '/my_leases';
    });
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
  }

  /** @prop appTitle {string} Reflects the app_title attribute for ease of use. */
  get appTitle(): string { return this.getAttribute('app_title')!; }

  set appTitle(val: string) { this.setAttribute('app_title', val); }

  disconnectedCallback(): void {
    super.disconnectedCallback();
  }
}

define('leasing-selections-sk', LeasingSelectionsSk);
