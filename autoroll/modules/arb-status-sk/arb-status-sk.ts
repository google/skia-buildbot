/**
 * @module autoroll/modules/arb-status-sk
 * @description <h2><code>arb-status-sk</code></h2>
 *
 * <p>
 * This element displays the status of a single Autoroller.
 * </p>
 */

import { html, render } from 'lit-html'

import { $$ } from 'common-sk/modules/dom';
import { diffDate, localeTime } from 'common-sk/modules/human';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';

import { define } from 'elements-sk/define'
import 'elements-sk/styles/buttons';
import 'elements-sk/styles/select';
import 'elements-sk/styles/table';
import 'elements-sk/tabs-panel-sk';
import 'elements-sk/tabs-sk';

import { LoginTo } from '../../../infra-sk/modules/login';

// Truncate the given string to the given length. If the string was
// shortened, change the last three characters to ellipsis.
// TODO(borenet): Move this somewhere common so that it can be shared.
export function truncate(str: string, len: number) {
  if (str.length > len) {
    var ellipsis = "..."
    return str.substring(0, len - ellipsis.length) + ellipsis;
  }
  return str
}


const template = (ele: ARBStatusSk, status: Status) => html`
  <tabs-sk>
    <button value="status">Roller Status</button>
    <button value="manual">Trigger Manual Rolls</button>
  </tabs-sk>
  ${!ele.editRights ? html`<div>${ele.pleaseLoginMsg}</div>` : html``}
  <tabs-panel-sk selected="0">
    <div class="status">
      <div id="loadstatus">
        Reload (s)
        <input
            id="refreshInterval"
            type="number"
            value="${ele.refreshInterval}"
            label="Reload (s)"
            @input=${ele._reloadChanged}
            ></input>
        Last loaded at <span>${localeTime(ele.lastLoaded)}</span>
      </div>
      <table>
        ${status.config.parentWaterfall ? html`
          <tr>
            <td class="nowrap">Parent Repo Build Status</td>
            <td class="nowrap unknown">
              <span class="big"><a href="${status.config.parentWaterfall}" target="_blank">${status.config.parentWaterfall}</a></span>
            </td>
          </tr>
        ` : html``}
        <tr>
          <td class="nowrap">Current Mode:</td>
          <td class="nowrap unknown">
            <span class="big">${status.mode.mode}</span>
          </td>
        </tr>
        <tr>
          <td class="nowrap">Set By:</td>
          <td class="nowrap unknown">
            ${status.mode.user}${status.mode.message ? html`: ${status.mode.message}`: html``}
          </td>
        </tr>
        <tr>
          <td class="nowrap">Change Mode:</td>
          <td class="nowrap">
            ${status.validModes.map((mode: string) => mode == status.mode.mode ? html`` : html`
              <button
                  @click="${() => {ele._modeButtonPressed(mode)}}"
                  ?disabled="${!ele.editRights || ele.modeChangePending}"
                  title="${ele.editRights ? "Change the mode." : ele.pleaseLoginMsg}"
                  value="${mode}">
                ${ele._getModeButtonLabel(status.mode.mode, mode)}
              </button>
            `)}
          </td>
        </tr>
        <tr>
          <td class="nowrap">Status:</td>
          <td class="nowrap">
            <span class="${ele._statusClass(status.status)}"><span class="big">${status.status}</span></span>
            ${status.status.indexOf("throttle") >= 0 ? html`
              <span>until ${localeTime(new Date(status.throttledUntil * 1000))}</span>
              <button
                  @click="${ele._unthrottle}"
                  ?disabled="${!ele.editRights}"
                  title="${ele.editRights ? "Unthrottle the roller." : ele.pleaseLoginMsg}">
                Force Unthrottle
              </button>
            ` : html``}
            ${status.status.indexOf("waiting for roll window") >= 0 ? html`
              <span>until ${localeTime(ele.rollWindowStart)}</span>
            ` : html``}
          </td>
        </tr>
        ${ele.editRights && status.error ? html`
          <tr>
            <td class="nowrap">Error:</td>
            <td><pre>${status.error}</pre></td>
          </tr>
        ` : html``}
        <tr>
          <td class="nowrap">Current Roll:</td>
          <td>
            <div>
              ${status.currentRoll ? html`
                <a href="${ele._issueURL(status.currentRoll)}" class="big" target="_blank">${status.currentRoll.subject}</a>
              ` : html`<span>(none)</span>`}
            </div>
            <div>
              ${status.currentRoll ? status.currentRoll.tryResults.map((tryResult) => html`
                <div class="trybot">
                  ${tryResult.url ? html`
                    <a href="${tryResult.url}"
                        class="${ele._trybotClass(tryResult)}"
                        target="_blank">
                      ${tryResult.builder}
                    </a>
                  ` : html`
                    <span class="nowrap"
                        class="${ele._trybotClass(tryResult)}">
                      ${tryResult.builder}
                    </span>
                  `}
                  ${tryResult.category === "cq" ? html`` : html`
                    <span class="nowrap small">(${tryResult.category})</span>
                  `}
                </div>
              `) : html``}
            </div>
          </td>
        </tr>
        ${status.lastRoll ? html`
          <tr>
            <td class="nowrap">Previous roll result:</td>
            <td>
              <span class="${ele._rollClass(status.lastRoll)}">${ele._rollResult(status.lastRoll)}</span>
              <a href="${ele._issueURL(status.lastRoll)}" target="_blank" class="small">(detail)</a>
            </td>
          </tr>
        ` : html``}
        <tr>
          <td class="nowrap">History:</td>
          <td>
            <table>
              <tr>
                <th>Roll</th>
                <th>Last Modified</th>
                <th>Result</th>
              </tr>
              ${status.recent.map((roll: Roll) => html`
                <tr>
                  <td><a href="${ele._issueURL(roll)}" target="_blank">${roll.subject}</a></td>
                  <td>${diffDate(roll.modified)} ago</td>
                  <td><span class="${ele._rollClass(roll)}">${ele._rollResult(roll)}</span></td>
                </tr>
              `)}
            </table>
          </td>
        </tr>
        <tr>
          <td class="nowrap">Full History:</td>
          <td>
            <a href="${status.fullHistoryUrl}" target="_blank">
              ${status.fullHistoryUrl}
            </a>
          </td>
        </tr>
        <tr>
          <td class="nowrap">Strategy for choosing next roll revision:</td>
          <td class="nowrap">
            <select
                id="strategySelect"
                ?disabled="${!ele.editRights}"
                title="${ele.editRights ? "Change the strategy for choosing the next revision to roll." : ele.pleaseLoginMsg}"
                @change="${ele._selectedStrategyChanged}">
              ${status.validStrategies.map((strategy: string) => html`
                <option value="${strategy}" ?selected="${strategy == status.strategy.strategy}">${strategy}</option>
              `)}
            </select>
          </td>
        </tr>
        <tr>
          <td class="nowrap">Set By:</td>
          <td class="nowrap unknown">
            ${status.strategy.user}${status.strategy.message ? html`: ${status.strategy.message}` : html``}
          </td>
        </tr>
      </table>
    </div>
    <div class="manual">
      <table>
        ${status.config.supportsManualRolls ? html`
          ${!ele.rollCandidates ? html`
            The roller is up to date; there are no revisions which could be manually rolled.
          ` : html``}
          <tr>
            <th>Revision</th>
            <th>Description</th>
            <th>Timestamp</th>
            <th>Requester</th>
            <th>Requested at</th>
            <th>Roll</th>
          </tr>
          ${ele.rollCandidates.map((rollCandidate) => html`
            <tr class="rollCandidate">
              <td>
                ${rollCandidate.revision.url ? html`
                  <a href="${rollCandidate.revision.url}" target="_blank">${rollCandidate.revision.display}</a>
                ` : html`
                  ${rollCandidate.revision.display}
                `}
              </td>
              <td>${!!rollCandidate.revision.description ? truncate(rollCandidate.revision.description, 100) : html``}</td>
              <td>${localeTime(new Date(rollCandidate.revision.timestamp))}</td>
              <td>${rollCandidate.roll ? rollCandidate.roll.requester : html``}</td>
              <td>${rollCandidate.roll ? localeTime(new Date(rollCandidate.roll.timestamp)): html``}</td>
              <td>
                ${rollCandidate.roll && rollCandidate.roll.url ? html`
                  <a href="${rollCandidate.roll.url}", target="_blank">${rollCandidate.roll.url}</a>
                ` : html``}
                ${!!rollCandidate.roll && !rollCandidate.roll.url && rollCandidate.roll.status ? rollCandidate.roll.status : html``}
                ${!rollCandidate.roll ? html`
                  <button
                      @click="${() => {ele._requestManualRoll(rollCandidate.revision.id)}}"
                      class="requestRoll"
                      ?disabled=${!ele.editRights}
                      title="${ele.editRights ? "Request a roll to this revision." : ele.pleaseLoginMsg}">
                    Request Roll
                  </button>
                ` : html``}
                ${!!rollCandidate.roll && !!rollCandidate.roll.result ? html`
                  <span class="${ele._reqResultClass(rollCandidate.roll)}">${rollCandidate.roll.result}</span>
                ` : html``}
              </td>
            </tr>
          `)}
          <tr class="rollCandidate">
            <td>
              <input id="manualRollRevInput" label="type revision/ref"></input>
            </td>
            <td><!-- no description        --></td>
            <td><!-- no revision timestamp --></td>
            <td><!-- no requester          --></td>
            <td><!-- no request timestamp  --></td>
            <td>
              <button
                  @click="${() => {
                    ele._requestManualRoll((<HTMLInputElement>$$("#manualRollRevInput")).value);
                  }}"
                  class="requestRoll"
                  ?disabled=${!ele.editRights}
                  title="${ele.editRights ? "Request a roll to this revision." : ele.pleaseLoginMsg}">
                Request Roll
              </button>
            </td>
          </tr>
        ` : html`
          This roller does not support manual rolls. If you want this feature,
          update the config file for the roller to enable it. Note that some
          rollers cannot support manual rolls for technical reasons.
        `}
      </table>
    </div>
  </tabs-panel-sk>
  <dialog id="modeChangeDialog">
    <h2>Enter a message:</h2>
    <input type="text" id="modeChangeMsgInput"></input>
    <button @click="${() => {ele._changeMode(false)}}">Cancel</button>
    <button @click="${() => {ele._changeMode(true)}}">Submit</button>
  </dialog>
  <dialog id="strategyChangeDialog">
    <h2>Enter a message:</h2>
    <input type="text" id="strategyChangeMsgInput"></input>
    <button @click="${() => {ele._changeStrategy(false)}}">Cancel</button>
    <button @click="${() => {ele._changeStrategy(true)}}">Submit</button>
  </dialog>
`;

export class Config {
  parentWaterfall: string
  supportsManualRolls: boolean
  timeWindow: string

  constructor() {
    this.parentWaterfall = "";
    this.supportsManualRolls = false;
    this.timeWindow = "";
  }
}

export class ManualRollRequest {
  requester: string
  result: string
  revision: string
  rollerName: string
  status: string
  timestamp: string
  url: string

  constructor() {
    this.requester = "";
    this.result = "";
    this.revision = "";
    this.rollerName = "";
    this.status = "";
    this.timestamp = "";
    this.url = "";
  }
}

export class Mode {
  message: string
  mode: string
  time: string
  user: string

  constructor() {
    this.message = "";
    this.mode = "";
    this.time = "";
    this.user = "";
  }
}

export class TryResult {
  builder: string
  category: string
  created_ts: string
  result: string
  status: string
  url: string

  constructor() {
    this.builder = "";
    this.category = "";
    this.created_ts = "";
    this.result = "";
    this.status = "";
    this.url = "";
  }
}

export class Roll {
  closed: boolean
  commitQueue: boolean
  committed: boolean
  cqDryRun: boolean
  created: string
  issue: number
  modified: string
  subject: string
  result: string
  rollingFrom: string
  rollingTo: string
  tryResults: TryResult[]

  constructor() {
    this.closed = false;
    this.commitQueue = false;
    this.committed = false;
    this.cqDryRun = false;
    this.created = "";
    this.issue = 0;
    this.modified = "";
    this.result = "";
    this.rollingFrom = "";
    this.rollingTo = "";
    this.subject = "";
    this.tryResults = [];
  }
}

export class Revision {
  id: string
  description: string
  display: string
  timestamp: string
  url: string

  constructor() {
    this.id = "";
    this.description = "";
    this.display = "";
    this.timestamp = "";
    this.url = "";
  }
}

export class RollCandidate{
  revision: Revision
  roll: ManualRollRequest

  constructor() {
    this.revision = new Revision();
    this.roll = new ManualRollRequest();
  }
}

export class Strategy {
  message: string
  strategy: string
  time: string
  user: string

  constructor() {
    this.message = "";
    this.strategy = "";
    this.time = "";
    this.user = "";
  }
}

export class Status {
  config: Config
  currentRoll: Roll
  currentRollRev: string
  error: string
  fullHistoryUrl: string
  issueUrlBase: string
  lastRoll: Roll
  lastRollRev: string
  manualRequests: ManualRollRequest[]
  mode: Mode
  notRolledRevs: Revision[]
  numBehind: number
  numFailed: number
  recent: Roll[]
  status: string
  strategy: Strategy
  throttledUntil: number
  validModes: string[]
  validStrategies: string[]

  constructor() {
    this.config = new Config();
    this.currentRoll = new Roll();
    this.currentRollRev = "";
    this.error = "";
    this.fullHistoryUrl = "";
    this.issueUrlBase = "";
    this.lastRoll = new Roll();
    this.lastRollRev = "";
    this.manualRequests = [];
    this.mode = new Mode();
    this.notRolledRevs = [];
    this.numBehind = 0;
    this.numFailed = 0;
    this.recent = [];
    this.status = "";
    this.strategy = new Strategy();
    this.throttledUntil = 0;
    this.validModes = [];
    this.validStrategies = [];
  }
}

class ARBStatusSk extends HTMLElement {
  private _editRights: boolean;
  private _lastLoaded: Date;
  private _modeChangePending: boolean;
  private readonly _pleaseLoginMsg = "Please login to make changes.";
  private _refreshInterval = 60;
  private _rollCandidates: RollCandidate[];
  private _rollWindowStart: Date;
  private _selectedMode: string;
  private _status: Status;
  private _strategyChangePending: boolean;
  private _timeout: number;

  constructor() {
    super();
    this._editRights = false;
    this._lastLoaded = new Date(0);
    this._modeChangePending = false;
    this._rollCandidates = [];
    this._rollWindowStart = new Date(0);
    this._selectedMode = "";
    this._status = new Status();
    this._strategyChangePending = false;
    this._timeout = 0;
  }

  get editRights() {
    return this._editRights;
  }
  get lastLoaded() {
    return this._lastLoaded;
  }
  get modeChangePending() {
    return this._modeChangePending;
  }
  get pleaseLoginMsg() {
    return this._pleaseLoginMsg;
  }
  get refreshInterval() {
    return this._refreshInterval;
  }
  get rollCandidates() {
    return this._rollCandidates;
  }
  get rollWindowStart() {
    return this._rollWindowStart;
  }
  get strategyChangePending() {
    return this._strategyChangePending;
  }

  connectedCallback() {
    this._render();
    LoginTo("/loginstatus/").then((loginstatus: any) => {
      this._editRights = loginstatus.IsAGoogler;
      this._render();
    });
    this._reload();
  }

  _modeButtonPressed(mode: string) {
    if (mode == this._status.mode.mode) {
      return;
    }
    this._selectedMode = mode;
    (<HTMLDialogElement>$$("#modeChangeDialog", this))?.showModal();
  }

  _fetch(input: RequestInfo, init?: RequestInit | undefined): Promise<any> {
    this.dispatchEvent(new CustomEvent('begin-task', { bubbles: true }));
    return fetch(input, init).then(jsonOrThrow)
        .then((v: any) => {
          this.dispatchEvent(new CustomEvent('end-task', { bubbles: true }));
          return v;
        }, (err: any) => {
          this.dispatchEvent(new CustomEvent('fetch-error', {
            detail: {
              error: err,
              loading: input,
            },
            bubbles: true,
          }));
          return err;
        })
        .catch((err: any) => {
          this.dispatchEvent(new CustomEvent('fetch-error', {
            detail: {
              error: err,
              loading: input,
            },
            bubbles: true,
          }));
          throw err;
        })
  }

  _changeMode(submit: boolean) {
    (<HTMLDialogElement>$$("#modeChangeDialog", this))?.close();
    if (!submit) {
      this._selectedMode = "";
      return;
    }
    let modeChangeMsgInput = <HTMLInputElement>$$("#modeChangeMsgInput", this);
    if (!modeChangeMsgInput) {
      return;
    }
    this._modeChangePending = true;
    let mode = new Mode();
    mode.message = modeChangeMsgInput.value;
    mode.mode = this._selectedMode;
    let url = window.location.pathname + "/json/mode";
    this._fetch(url, {
      method: "POST",
      body: JSON.stringify(mode),
      headers: {
        'Content-Type': 'application/json',
      },
    }).then((json) => {
      this._modeChangePending = false;
      modeChangeMsgInput.value = "";
      this._update(json);
    }, (err) => {
      this._modeChangePending = false;
      this._render();
    });
  }

  _changeStrategy(submit: boolean) {
    (<HTMLDialogElement>$$("#strategyChangeDialog", this))?.close();
    let strategySelect = <HTMLSelectElement>$$("#strategySelect");
    let strategyChangeMsgInput = <HTMLInputElement>$$("#strategyChangeMsgInput");
    if (!submit) {
      if (!!strategySelect) {
        strategySelect.value = this._status.strategy.strategy;
      }
      return;
    }
    if (!strategyChangeMsgInput || !strategySelect) {
      return;
    }
    let strategy = new Strategy();
    strategy.message = strategyChangeMsgInput.value;
    strategy.strategy = strategySelect.value;
    let url = window.location.pathname + "/json/strategy";
    this._fetch(url, {
      method: "POST",
      body: JSON.stringify(strategy),
      headers: {
        'Content-Type': 'application/json',
      },
    }).then((json) => {
      this._strategyChangePending = false;
      strategyChangeMsgInput.value = "";
      this._update(json);
    }, (err) => {
      this._strategyChangePending = false;
      strategySelect.value = this._status.strategy.strategy;
      this._render();
    });
  }

  // _computeRollWindowStart returns a string indicating when the configured
  // roll window will start. If errors are encountered, in particular those
  // relating to parsing the roll window, the returned string will contain
  // the error.
  _computeRollWindowStart(config: Config) {
    if (!config || !config.timeWindow) {
      return "";
    }
    // TODO(borenet): This duplicates code in the go/time_window package.

    // parseDayTime returns a 2-element array containing the hour and
    // minutes as ints. Throws an error (string) if the given string cannot
    // be parsed as hours and minutes.
    const parseDayTime = function(s: string) {
      const timeSplit = s.split(":");
      if (timeSplit.length !== 2) {
        throw "Expected time format \"hh:mm\", not " + s;
      }
      const hours = parseInt(timeSplit[0]);
      if (hours < 0 || hours >= 24) {
        throw "Hours must be between 0-23, not " + timeSplit[0];
      }
      const minutes = parseInt(timeSplit[1]);
      if (minutes < 0 || minutes >= 60) {
        throw "Minutes must be between 0-59, not " + timeSplit[1];
      }
      return [hours, minutes];
    };

    // Parse multiple day/time windows, eg. M-W 00:00-04:00; Th-F 00:00-02:00
    const windows = [];
    const split = config.timeWindow.split(";");
    for (let i = 0; i < split.length; i++) {
      const dayTimeWindow = split[i].trim();
      // Parse individual day/time window, eg. M-W 00:00-04:00
      const windowSplit = dayTimeWindow.split(" ");
      if (windowSplit.length !== 2) {
        return "unknown; expected format \"D hh:mm\", not " + dayTimeWindow;
      }
      const dayExpr = windowSplit[0].trim();
      const timeExpr = windowSplit[1].trim();

      // Parse the starting and ending times.
      const timeExprSplit = timeExpr.split("-");
      if (timeExprSplit.length !== 2) {
        return "unknown; expected format \"hh:mm-hh:mm\", not " + timeExpr;
      }
      let startTime;
      try {
        startTime = parseDayTime(timeExprSplit[0]);
      } catch(e) {
        return e;
      }
      let endTime;
      try {
        endTime = parseDayTime(timeExprSplit[1]);
      } catch(e) {
        return e;
      }

      // Parse the day(s).
      const allDays = ["Su", "M", "Tu", "W", "Th", "F", "Sa"];
      const days = [];

      // "*" means every day.
      if (dayExpr === "*") {
        days.push(...allDays.map((_, i) => i));
      } else {
        const rangesSplit = dayExpr.split(",");
        for (let i = 0; i < rangesSplit.length; i++) {
          const rangeSplit = rangesSplit[i].split("-");
          if (rangeSplit.length === 1) {
            const day = allDays.indexOf(rangeSplit[0]);
            if (day === -1) {
              return "Unknown day " + rangeSplit[0];
            }
            days.push(day);
          } else if (rangeSplit.length === 2) {
            const startDay = allDays.indexOf(rangeSplit[0]);
            if (startDay === -1) {
              return "Unknown day " + rangeSplit[0];
            }
            let endDay = allDays.indexOf(rangeSplit[1]);
            if (endDay === -1) {
              return "Unknown day " + rangeSplit[1];
            }
            if (endDay < startDay) {
              endDay += 7;
            }
            for (let day = startDay; day <= endDay; day++) {
              days.push(day % 7);
            }
          } else {
            return "Invalid day expression " + rangesSplit[i];
          }
        }
      }

      // Add the windows to the list.
      for (let i = 0; i < days.length; i++) {
        windows.push({
          day: days[i],
          start: startTime,
          end: endTime,
        });
      }
    }

    // For each window, find the timestamp at which it opens next.
    const now = new Date().getTime();
    const openTimes = windows.map((w) => {
      let next = new Date(now);
      next.setUTCHours(w.start[0], w.start[1], 0, 0);
      let dayOffsetMs = (w.day - next.getUTCDay()) * 24 * 60 * 60 * 1000;
      next = new Date(next.getTime() + dayOffsetMs);
      if (next.getTime() < now) {
        // If we've missed this week's window, bump forward a week.
        next = new Date(next.getTime() + 7*24*60*60*1000);
      }
      return next;
    });

    // Pick the next window.
    openTimes.sort((a, b) => a.getTime() - b.getTime());
    return openTimes[0].toLocaleString();
  }

  _issueURL(roll: Roll): string {
    if (roll) {
      return this._status.issueUrlBase + roll.issue;
    }
    return "";
  }

  _getModeButtonLabel(currentMode: string, mode: string) {
    // TODO(borenet): This is a hack; it doesn't respect this.validModes.
    const modeButtonLabels: {[key:string]: {[key:string]: string}} = {
      "running": {
        "stopped": "stop",
        "dry run": "switch to dry run",
      },
      "stopped": {
        "running": "resume",
        "dry run": "switch to dry run",
      },
      "dry run": {
        "running": "switch to normal mode",
        "stopped": "stop",
      },
    };
    return modeButtonLabels[currentMode][mode];
  }

  _reloadChanged() {
    let refreshIntervalInput = <HTMLInputElement>$$("refreshIntervalInput", this);
    if (refreshIntervalInput) {
      this._refreshInterval = refreshIntervalInput.valueAsNumber;
      this._resetTimeout();
    }
  }

  _resetTimeout() {
    if (this._timeout) {
      window.clearTimeout(this._timeout);
    }
    if (this._refreshInterval > 0) {
      this._timeout = window.setTimeout(() => {
        this._reload();
      }, this._refreshInterval * 1000);
    }
  }

  _reload() {
    var url = window.location.pathname + "/json/status";
    console.log("Loading status from " + url);
    this._fetch(url).then((json) => {
      this._update(json);
      this._resetTimeout();
    }).catch((err) => {
      this._resetTimeout();
    });
  }

  _reqResultClass(req: ManualRollRequest) {
    if (!req) {
      return "";
    }
    const manualRequestResultClass: {[key:string]:string} = {
      "SUCCESS": "fg-success",
      "FAILURE": "fg-failure",
    }
    return manualRequestResultClass[req.result];
  }

  _requestManualRoll(rev: string) {
    let url = window.location.pathname + "/json/manual";
    let req: ManualRollRequest = {
      result: "",
      revision: rev,
      requester: "",
      rollerName: "",
      status: "",
      timestamp: "",
      url: "",
    };
    this._fetch(url, {
      method: "POST",
      body: JSON.stringify(req),
      headers: {
        'Content-Type': 'application/json',
      },
    }).then((json) => {
      let req = <ManualRollRequest>json;
      let exist = this._rollCandidates.find((r) => r.revision.id == req.revision);
      if (!!exist) {
        exist.roll = req;
      } else {
        this._rollCandidates.push({
          revision: {
            id: req.revision,
            description: "",
            display: req.revision,
            timestamp: "",
            url: "",
          },
          roll: req,
        });
      }
      let manualRollRevInput = <HTMLInputElement>$$("#manualRollRevInput");
      if (!!manualRollRevInput) {
        manualRollRevInput.value = "";
      }
      this._render();
    });
  }

  _rollClass(roll: Roll) {
    if (!roll) {
      return "unknown";
    }
    const rollClassMap: {[key:string]: string} = {
      "succeeded": "fg-success",
      "failed": "fg-failure",
      "in progress": "fg-unknown",
      "dry run succeeded": "fg-success",
      "dry run failed": "fg-failure",
      "dry run in progress": "fg-unknown",
    };
    return rollClassMap[roll.result] || "fg-unknown";
  }

  _rollResult(roll: Roll) {
    if (!roll) {
      return "unknown";
    }
    return roll.result;
  }

  _statusClass(status: string) {
    const statusClassMap: {[key:string]: string} = {
      "idle":                          "fg-unknown",
      "active":                        "fg-unknown",
      "success":                       "fg-success",
      "failure":                       "fg-failure",
      "throttled":                     "fg-failure",
      "dry run idle":                  "fg-unknown",
      "dry run active":                "fg-unknown",
      "dry run success":               "fg-success",
      "dry run success; leaving open": "fg-success",
      "dry run failure":               "fg-failure",
      "dry run throttled":             "fg-failure",
      "stopped":                       "fg-failure",
    }
    return statusClassMap[status] || "";
  }

  _selectedStrategyChanged() {
    if ((<HTMLSelectElement>$$("strategySelect", this))?.value == this._status.strategy.strategy) {
      return;
    }
    (<HTMLDialogElement>$$("#strategyChangeDialog", this))?.showModal();
  }

  _trybotClass(trybot: TryResult) {
    if (trybot.status == "STARTED") {
      return "fg-unknown";
    } else if (trybot.status == "COMPLETED") {
      const trybotClass: {[key:string]:string} = {
        "CANCELED": "fg-failure",
        "FAILURE": "fg-failure",
        "SUCCESS": "fg-success",
      }
      return trybotClass[trybot.result] || "";
    } else {
      return "fg-unknown";
    }
  }

  _unthrottle() {
    var url = window.location.pathname + "/json/unthrottle";
    this._fetch(url, {method: "POST"});
  }

  _update(status: Status) {
    var rollCandidates: RollCandidate[] = [];
    let manualByRev: {[key:string]:ManualRollRequest} = {};
    if (status.notRolledRevs) {
      if (status.manualRequests) {
        for (let i = 0; i < status.manualRequests.length; i++) {
          const req = status.manualRequests[i];
          manualByRev[req.revision] = req;
        }
      }
      for (let i = 0; i < status.notRolledRevs.length; i++) {
        var rev = status.notRolledRevs[i];
        let candidate = new RollCandidate();
        candidate.revision = rev;
        var req = manualByRev[rev.id];
        delete manualByRev[rev.id];
        if (!req && status.currentRoll && status.currentRoll.rollingTo == rev.id) {
          req = new ManualRollRequest();
          req.requester = "autoroller";
          req.result = "";
          req.revision = "";
          req.status = "STARTED",
          req.timestamp = status.currentRoll.created;
          req.url = this._issueURL(status.currentRoll);
        }
        candidate.roll = req;
        rollCandidates.push(candidate);
      }
    }
    for (var key in manualByRev) {
      const req = manualByRev[key];
      const rev = new Revision();
      rev.id = req.revision;
      rev.display = req.revision;
      rollCandidates.push({
        revision: rev,
        roll: req,
      });
    };
    this._lastLoaded = new Date();
    this._rollCandidates = rollCandidates;
    this._rollWindowStart = this._computeRollWindowStart(status.config);
    this._status = status;
    console.log("Reloaded status.");
    this._render();
  }

  _render() {
    render(template(this, this._status), this, {eventContext: this});
  }
}

define('arb-status-sk', ARBStatusSk);