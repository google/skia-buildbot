/**
 * @module modules/capacity-sk
 * @description <h2><code>capacity-sk</code></h2>
 *
 * The bulk of the device capacity page.
 * @evt
 *
 * @attr
 *
 * @example
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import '../../../infra-sk/modules/theme-chooser-sk';
import '../../../infra-sk/modules/app-sk';
import '../../../infra-sk/modules/login-sk';
import '../../../ct/modules/input-sk';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import 'elements-sk/error-toast-sk';
import 'elements-sk/icon/battery-charging-80-icon-sk';
import 'elements-sk/icon/dashboard-icon-sk';
import 'elements-sk/icon/devices-icon-sk';
import 'elements-sk/icon/arrow-drop-down-icon-sk';
import 'elements-sk/icon/arrow-drop-up-icon-sk';
import { StatusService, GetStatusService, BotSet, BotSet_DimensionsEntry } from '../rpc';
import { errorMessage } from 'elements-sk/errorMessage';
import { $$, DomReady } from 'common-sk/modules/dom';
import { stateReflector } from 'common-sk/modules/stateReflector';
import { HintableObject } from 'common-sk/modules/hintable';

// Row represents a given bot's data after applying user inputs,
// split into 2 classes so we can define the ColumnName type for sorting.
// Times are in minutes.
class ColumnSet {
  config: string = '';
  commitTime: number = 0;
  commitTasks: number = 0;
  cqTime: number = 0;
  cqTasks: number = 0;
  botDays: number = 0;
  optimisticBots: number = 0;
  pessimisticBots: number = 0;
  botCount: number = 0;
  optimisticPercent: number = 0;
}

class Row extends ColumnSet {
  swarmingUrl: string = '';
  displayClass: string = '';
}

type ColumnName = keyof ColumnSet;
type SortDirection = 1 | -1;

// Used by stateReflector.
class State {
  commits: number = 30;
  cq: number = 1.5;
  optimistic = 90;
  pessimistic = 60;
  backfill = 100;
  sortColumn: ColumnName = 'optimisticPercent';
  sortDirection: SortDirection = 1;
}

export class CapacitySk extends ElementSk {
  private client: StatusService = GetStatusService();
  private stateHasChanged = () => {};

  private static template = (el: CapacitySk) =>
    html`<app-sk>
      <header>
        <h1>Capacity Statistics for Skia Bots</h1>
        <div class="spacer"></div>
        <login-sk></login-sk>
        <theme-chooser-sk></theme-chooser-sk>
      </header>
      <aside>
        <div>
          <div class="table">
            <a class="tr" href="/">
              <span class="td">
                <dashboard-icon-sk class="icon"></dashboard-icon-sk> Status Tree
              </span>
            </a>
            <a class="tr" href="https://goto.google.com/skbl">
              <span class="td">
                <devices-icon-sk class="icon"></devices-icon-sk> Swarming Bots
              </span>
            </a>
            <a class="tr" href="/capacity">
              <span class="td">
                <battery-charging-80-icon-sk class="icon"></battery-charging-80-icon-sk>
                Capacity Stats
              </span>
            </a>
          </div>
        </div>
      </aside>

      <main>
        <div class="inputs horizontal">
          <input-sk
            @change=${() => el.refresh()}
            id="commits"
            label="Commits Per Day (typically 15-35)"
          ></input-sk>
          <input-sk @change=${() => el.refresh()} id="cq" label="CQ attempts per commit"></input-sk>
          <!-- TODO(kjlubick) actually compute utilization (metrics) and display the range here for
          reference.-->
          <input-sk
            @change=${() => el.refresh()}
            id="optimistic"
            label="Optimistic Utilization % Estimate"
          ></input-sk>
          <input-sk
            @change=${() => el.refresh()}
            id="pessimistic"
            label="Pessimistic Utilization % Estimate"
          ></input-sk>
          <input-sk
            @change=${() => el.refresh()}
            id="backfill"
            label="Target Backfill %"
          ></input-sk>
        </div>

        <table>
          <thead>
            <tr>
              <th @click=${() => el.updateSort('config')}>Bot Config${el.sortIcon('config')}</th>
              <th @click=${() => el.updateSort('commitTime')}>
                Minutes per Commit${el.sortIcon('commitTime')}
              </th>
              <th @click=${() => el.updateSort('commitTasks')}>
                Tasks per Commit${el.sortIcon('commitTasks')}
              </th>
              <th @click=${() => el.updateSort('cqTime')}>
                Minutes per CQ run${el.sortIcon('cqTime')}
              </th>
              <th @click=${() => el.updateSort('cqTasks')}>Tasks on CQ${el.sortIcon('cqTasks')}</th>
              <th @click=${() => el.updateSort('botDays')}>
                Bot days of work / actual day${el.sortIcon('botDays')}
              </th>
              <th @click=${() => el.updateSort('optimisticBots')}>
                Required Bots (optimistic)${el.sortIcon('optimisticBots')}
              </th>
              <th @click=${() => el.updateSort('pessimisticBots')}>
                Required Bots (pessimistic)${el.sortIcon('pessimisticBots')}
              </th>
              <th @click=${() => el.updateSort('botCount')}>
                Actual Bot Count${el.sortIcon('botCount')}
              </th>
              <th @click=${() => el.updateSort('optimisticPercent')}>
                Percent of Optimistic Estimate${el.sortIcon('optimisticPercent')}
              </th>
            </tr>
          </thead>
          <tbody>
            ${el.botsets
              .map((botset) => el.rowFromBotset(botset))
              .sort((a, b) => el.compareRow(a, b))
              .map(
                (row) => html` <tr class=${row.displayClass}>
                  <td><a href=${row.swarmingUrl}>${row.config}</a></td>
                  <td>${row.commitTime.toFixed(1)}</td>
                  <td>${row.commitTasks}</td>
                  <td>${row.cqTime.toFixed(1)}</td>
                  <td>${row.cqTasks}</td>
                  <td>${row.botDays.toFixed(1)}</td>
                  <td>${row.optimisticBots.toFixed(1)}</td>
                  <td>${row.pessimisticBots.toFixed(1)}</td>
                  <td>${row.botCount}</td>
                  <td>${row.optimisticPercent.toFixed(1)} %</td>
                </tr>`
              )}
          </tbody>
        </table>
      </main>

      <footer><error-toast-sk></error-toast-sk></footer>
    </app-sk> `;

  constructor() {
    super(CapacitySk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
    // Load default values into inputs.
    this.setState((new State() as unknown) as HintableObject);
    // This will cause setState to be called again once the page loads, loading
    // any parameters from the url.
    this.stateHasChanged = stateReflector(
      () => this.getState(),
      (fromUrl) => this.setState(fromUrl)
    );
    this.client
      .getBotUsage({})
      .then((resp) => {
        this.botsets = resp.botSets || [];
        this._render();
      })
      .catch(errorMessage);
  }
  private sortColumn: ColumnName = 'optimisticPercent';
  private sortDirection: SortDirection = 1;
  private botsets: BotSet[] = [];

  private updateSort(column: ColumnName) {
    if (column === this.sortColumn) {
      // If same column is clicked, we flip the sort direction.
      this.sortDirection = (this.sortDirection * -1) as SortDirection;
    } else {
      this.sortColumn = column;
      this.sortDirection = 1;
    }
    this.refresh();
  }
  private sortIcon(column: ColumnName) {
    return column !== this.sortColumn
      ? html``
      : this.sortDirection === 1
      ? html`<arrow-drop-down-icon-sk></arrow-drop-down-icon-sk>`
      : html`<arrow-drop-up-icon-sk></arrow-drop-up-icon-sk>`;
  }

  private refresh() {
    this.stateHasChanged();
    this._render();
  }

  private compareRow(a: Row, b: Row) {
    const aVal = a[this.sortColumn];
    const bVal = b[this.sortColumn];
    if (aVal === bVal) return 0;
    return (aVal < bVal ? -1 : 1) * this.sortDirection;
  }

  private rowFromBotset(botset: BotSet) {
    const commitsPerDay = Number(($$('#commits', this) as HTMLInputElement).value);
    const cqMultiplier = Number(($$('#cq', this) as HTMLInputElement).value);
    const optimisticUtilization = Number(($$('#optimistic', this) as HTMLInputElement).value);
    const pessimisticUtilization = Number(($$('#pessimistic', this) as HTMLInputElement).value);
    const targetBackfillPercent = Number(($$('#backfill', this) as HTMLInputElement).value);
    const workMultiplier = botWorkMultiplier(
      botset,
      commitsPerDay,
      cqMultiplier,
      targetBackfillPercent
    );
    const optimisticBots = botEstimate(workMultiplier, optimisticUtilization);
    const pessimisticBots = botEstimate(workMultiplier, pessimisticUtilization);
    const swarmingUrl = Object.keys(botset.dimensions!).reduce(
      (url, dimKey) => `${url}&f=${dimKey}:${botset.dimensions![dimKey]}`,
      'https://chromium-swarm.appspot.com/tasklist?c=name&c=state&c=created_ts&c=user&c=gpu&c=device_type&c=os&l=50&s=created_ts%3Adesc'
    );
    return <Row>{
      config: botConfig(botset.dimensions!),
      commitTime: botset.msPerCommit / (60 * 1000),
      commitTasks: botset.totalTasks,
      cqTime: botset.msPerCq / (60 * 1000),
      cqTasks: botset.cqTasks,
      botDays: workMultiplier,
      optimisticBots: optimisticBots,
      pessimisticBots: pessimisticBots,
      botCount: botset.botCount,
      optimisticPercent: (100 * botset.botCount) / optimisticBots,
      swarmingUrl: swarmingUrl,
      displayClass:
        botset.botCount < optimisticBots
          ? 'lowBotCount'
          : botset.botCount < pessimisticBots
          ? 'mediumBotCount'
          : 'highBotCount',
    };
  }

  private getState(): HintableObject {
    const state: State = {
      commits: Number(($$('#commits', this) as HTMLInputElement).value),
      cq: Number(($$('#cq', this) as HTMLInputElement).value),
      optimistic: Number(($$('#optimistic', this) as HTMLInputElement).value),
      pessimistic: Number(($$('#pessimistic', this) as HTMLInputElement).value),
      backfill: Number(($$('#backfill', this) as HTMLInputElement).value),
      sortColumn: this.sortColumn,
      sortDirection: this.sortDirection,
    };
    return (state as unknown) as HintableObject;
  }

  private setState(fromUrl: HintableObject) {
    let state = (fromUrl as unknown) as State;
    ($$('#commits', this) as HTMLInputElement).value = state.commits.toString();
    ($$('#cq', this) as HTMLInputElement).value = state.cq.toString();
    ($$('#optimistic', this) as HTMLInputElement).value = state.optimistic.toString();
    ($$('#pessimistic', this) as HTMLInputElement).value = state.pessimistic.toString();
    ($$('#backfill', this) as HTMLInputElement).value = state.backfill.toString();
    this.sortColumn = state.sortColumn;
    this.sortDirection = state.sortDirection;
    this._render();
  }
}

function botEstimate(workMultiplier: number, util: number) {
  return workMultiplier / (util / 100);
}

// Represents the number of 100% utilized bots needed to keep up with work.
function botWorkMultiplier(
  item: BotSet,
  commits_per_day: number,
  cq_multiplier: number,
  target_backfill: number
) {
  let days = (item.msPerCommit * commits_per_day * target_backfill) / 100;
  days += item.msPerCq * cq_multiplier * commits_per_day;
  return days / (24 * 60 * 60 * 1000);
}

const ANDROID_ALIASES = {
  angler: 'Nexus 6p',
  athene: 'Moto G4',
  bullhead: 'Nexus 5X',
  dragon: 'Pixel C',
  flo: 'Nexus 7 [2013]',
  flounder: 'Nexus 9',
  foster: 'NVIDIA Shield',
  fugu: 'Nexus Player',
  gce_x86: 'Android on GCE',
  goyawifi: 'Galaxy Tab 3',
  grouper: 'Nexus 7 [2012]',
  hammerhead: 'Nexus 5',
  herolte: 'Galaxy S7 [Global]',
  heroqlteatt: 'Galaxy S7 [AT&T]',
  j5xnlte: 'Galaxy J5',
  m0: 'Galaxy S3',
  mako: 'Nexus 4',
  manta: 'Nexus 10',
  marlin: 'Pixel XL',
  sailfish: 'Pixel',
  shamu: 'Nexus 6',
  sprout: 'Android One',
  zerofltetmo: 'Galaxy S6',
} as const;

const GPU_ALIASES = {
  '1002': 'AMD',
  '1002:6613': 'AMD Radeon R7 240',
  '1002:6646': 'AMD Radeon R9 M280X',
  '1002:6779': 'AMD Radeon HD 6450/7450/8450',
  '1002:679e': 'AMD Radeon HD 7800',
  '1002:6821': 'AMD Radeon HD 8870M',
  '1002:683d': 'AMD Radeon HD 7770/8760',
  '1002:9830': 'AMD Radeon HD 8400',
  '1002:9874': 'AMD Carrizo',
  '102b': 'Matrox',
  '102b:0522': 'Matrox MGA G200e',
  '102b:0532': 'Matrox MGA G200eW',
  '102b:0534': 'Matrox G200eR2',
  '10de': 'NVIDIA',
  '10de:08a4': 'NVIDIA GeForce 320M',
  '10de:08aa': 'NVIDIA GeForce 320M',
  '10de:0a65': 'NVIDIA GeForce 210',
  '10de:0fe9': 'NVIDIA GeForce GT 750M Mac Edition',
  '10de:0ffa': 'NVIDIA Quadro K600',
  '10de:104a': 'NVIDIA GeForce GT 610',
  '10de:11c0': 'NVIDIA GeForce GTX 660',
  '10de:1244': 'NVIDIA GeForce GTX 550 Ti',
  '10de:1401': 'NVIDIA GeForce GTX 960',
  '10de:1ba1': 'NVIDIA GeForce GTX 1070',
  '10de:1cb3': 'NVIDIA Quadro P400',
  '8086': 'Intel',
  '8086:0046': 'Intel Ironlake HD Graphics',
  '8086:0102': 'Intel Sandy Bridge HD Graphics 2000',
  '8086:0116': 'Intel Sandy Bridge HD Graphics 3000',
  '8086:0166': 'Intel Ivy Bridge HD Graphics 4000',
  '8086:0412': 'Intel Haswell HD Graphics 4600',
  '8086:041a': 'Intel Haswell HD Graphics',
  '8086:0a16': 'Intel Haswell HD Graphics 4400',
  '8086:0a26': 'Intel Haswell HD Graphics 5000',
  '8086:0a2e': 'Intel Haswell Iris Graphics 5100',
  '8086:0d26': 'Intel Haswell Iris Pro Graphics 5200',
  '8086:0f31': 'Intel Bay Trail HD Graphics',
  '8086:1616': 'Intel Broadwell HD Graphics 5500',
  '8086:161e': 'Intel Broadwell HD Graphics 5300',
  '8086:1626': 'Intel Broadwell HD Graphics 6000',
  '8086:162b': 'Intel Broadwell Iris Graphics 6100',
  '8086:1912': 'Intel Skylake HD Graphics 530',
  '8086:1926': 'Intel Skylake Iris 540/550',
  '8086:193b': 'Intel Skylake Iris Pro 580',
  '8086:22b1': 'Intel Braswell HD Graphics',
  '8086:591e': 'Intel Kaby Lake HD Graphics 615',
  '8086:5926': 'Intel Kaby Lake Iris Plus Graphics 640',
} as const;

type AliasTable = typeof GPU_ALIASES | typeof ANDROID_ALIASES;

function applyAlias(str: string, lookup: AliasTable) {
  const nodash = str.split('-')[0] as keyof AliasTable;
  var alias = lookup[nodash];
  if (alias) {
    return `${alias} (${str})`;
  }
  return str;
}
function botConfig(dims: BotSet_DimensionsEntry) {
  let os = '(unspecified)';
  if (dims.os) {
    os = dims.os;
  }
  let pool = '(unspecified)';
  if (dims.pool) {
    pool = dims.pool;
  }

  let rest = '';
  if (dims.device) {
    rest = `Device: ${dims.device}`;
  }
  if (dims.device_type) {
    const alias = applyAlias(dims.device_type, ANDROID_ALIASES);
    rest = `Device Type: ${alias}`;
  }
  if (dims.gpu) {
    const alias = applyAlias(dims.gpu, GPU_ALIASES);
    rest = `GPU: ${alias}`;
  }
  if (dims.cpu) {
    rest += `, CPU: ${dims.cpu}`;
  }

  if (pool !== 'Skia') {
    return `OS: ${os}, Pool: ${pool}, ${rest}`;
  }
  return `OS: ${os}, ${rest}`;
}

define('capacity-sk', CapacitySk);
