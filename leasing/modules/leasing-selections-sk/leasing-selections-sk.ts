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
  private all_pools: string[] = ['Skia'];

  private pool: string = 'Skia';

  private osTypes: { [key: string]: number; } = {};

  private deviceTypes: Record<string, string> = {};

  private selectedDeviceType: string = '';

  private selectedOsType: string = '';

  private osToDeviceTypes: { [key: string]: { [key: string]: number } | null } = {};

  private loadingDetails: boolean = false;

  constructor() {
    super(LeasingSelectionsSk.template);

    this.fetchSupportedPools();
    this.fetchOsDetails();
  }

  private static template = (ele: LeasingSelectionsSk) => html`
    <table class="options panel">

      <tr>
        <td class="step-title">Select Pool</td>
        <td>
          <select id="pool" ?disabled=${ele.loadingDetails} .selection=${ele.pool} @input=${ele.poolChanged}>
            ${LeasingSelectionsSk.displayPools(ele)}
          </select>
        </td>
      </tr>

      <tr>
        <td class="step-title">Select OS Type</td>
        <td>
          <select id="os_type" ?disabled=${ele.loadingDetails} @input=${ele.osTypeChanged}>
            ${LeasingSelectionsSk.displayOsTypes(ele)}
          </select>
        </td>
      </tr>

      <tr>
        <td class="step-title">Select Device Type</td>
        <td>
          <select id="device_type" ?disabled=${ele.loadingDetails || (ele.selectedOsType !== 'Android' && !ele.selectedOsType.startsWith('iOS'))}>
            ${LeasingSelectionsSk.displayDeviceTypes(ele)}
          </select>
        </td>
      </tr>

      <tr>
        <td class="step-title">Specify BotId (optional)</td>
        <td>
          <input id="bot_id" ?disabled=${ele.loadingDetails}></input>
          <br/>
          <span class="smaller-font">Note: OS Type and Device Type are ignored if this is populated</span>
        </td>
      </tr>

      <tr>
        <td class="step-title">Specify Swarming Task Id<br/>to keep artifacts ready on bot<br/>(optional)</td>
        <td>
          <input id="task_id" ?disabled=${ele.loadingDetails}></input>
        </td>
      </tr>

      <tr>
        <td class="step-title">Lease Duration</td>
        <td>
          <select id="duration" ?disabled=${ele.loadingDetails}>
            <option value="1" title="1 Hour">1hr</option>
            <option value="2" title="2 Hours">2hr</option>
            <option value="6" title="6 Hours">6hr</option>
            <option value="23" title="23 Hours">23hr</option>
          </select>
        </td>

      <tr>
        <td class="step-title">Description</td>
        <td>
          <input id="desc" ?disabled=${ele.loadingDetails} placeholder="Description is required"></input>
        </td>
      </tr>

      <tr>
        <td colspan="2" class="center">
          <button raised @click=${ele.addTask} ?disabled=${ele.loadingDetails}>Lease Bot</button>
        </td>
      </tr>

    </table>
`;

  private static displayPools(ele: LeasingSelectionsSk): TemplateResult[] {
    return ele.all_pools.map((p) => html`
    <option
      ?selected=${ele.pool === p}
      value=${p}
      title=${p}
      >${p}
    </option>`);
  }

  private static displayOsTypes(ele: LeasingSelectionsSk): TemplateResult[] {
    if (ele.osTypes === {}) {
      return [html``];
    }
    return Object.keys(ele.osTypes).map((o) => html`
    <option
      value=${o}
      title=${o}
      >${o} - ${ele.osTypes[o]} bots online
    </option>`);
  }

  private static displayDeviceTypes(ele: LeasingSelectionsSk): TemplateResult[] {
    if (ele.osToDeviceTypes === {}) {
      return [html``];
    }
    if (!(ele.selectedOsType in ele.osToDeviceTypes)) {
      return [html``];
    }
    const deviceTypes = ele.osToDeviceTypes[ele.selectedOsType] || {};
    return Object.keys(deviceTypes).map((d) => html`
    <option
      value=${d}
      title=${d}
      >${device(d)} ${getAKAStr(d)} - ${deviceTypes[d]} bots online
    </option>`);
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
  }

  private poolChanged(e: InputEvent): void {
    this.pool = (e.target! as HTMLInputElement).value;
    this.fetchOsDetails();
  }

  private osTypeChanged(e: InputEvent): void {
    this.selectedOsType = (e.target! as HTMLInputElement).value;
    this._render();
  }

  private fetchSupportedPools(): void {
    doImpl('/_/get_supported_pools', {}, (json) => {
      this.all_pools = json;
      this._render();
    });
  }

  private fetchOsDetails(): void {
    doImpl(`/_/pooldetails?pool=${this.pool}`, {}, (json: PoolDetails) => {
      this.osTypes = json.os_types || {};
      this.osToDeviceTypes = json.os_to_device_types || {};
      // Select first OsType.
      this.selectedOsType = Object.keys(this.osTypes)[0];
      this._render();
    });
  }

  private addTask(): void {
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

    this.loadingDetails = true;
    this._render();
    doImpl('/_/add_leasing_task', detail, () => {
      window.location.href = '/my_leases';
    });
  }

  /** @prop appTitle {string} Reflects the app_title attribute for ease of use. */
  get appTitle(): string { return this.getAttribute('app_title')!; }

  set appTitle(val: string) { this.setAttribute('app_title', val); }
}

define('leasing-selections-sk', LeasingSelectionsSk);
