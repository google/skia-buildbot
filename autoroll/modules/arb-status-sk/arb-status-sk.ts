/**
 * @module autoroll/modules/arb-status-sk
 * @description <h2><code>arb-status-sk</code></h2>
 *
 * <p>
 * This element displays the status of a single Autoroller.
 * </p>
 */

//import { dialogPolyfill } from 'dialog-polyfill';
import { html, render } from 'lit-html'

import { $$ } from 'common-sk/modules/dom';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';

import { define } from 'elements-sk/define'
import { errorMessage } from 'elements-sk/errorMessage';
import 'elements-sk/tabs-panel-sk';
import 'elements-sk/tabs-sk';

import { Login } from '../../../infra-sk/modules/login';
import '../../../res/js/common';


const template = (ele: ARBStatusSk, status: Status) => html`
  <tabs-sk>
    <button value="status">Roller Status</button>
    <button value="manual">Trigger Manual Rolls</button>
  </tabs-sk>
  ${!ele.editRights ? html`<div>${ele.pleaseLoginMsg}</div>` : html``}
  <tabs-panel-sk>
  <section class="status">
    <div class="horizontal layout center" id="loadstatus">
      <input
          id="refreshInterval"
          type="number"
          value="${ele.refreshInterval}"
          label="Reload (s)"
          @input=${ele._reloadChanged}
          ></input>
      <div class="flex"></div>
      <div>Last loaded at <span>${ele.lastLoaded}</span></div>
    </div>
    <div class="table">
      ${status.config.parentWaterfall ? html`
        <div class="tr">
          <div class="td nowrap">Parent Repo Build Status</div>
          <div class="td nowrap unknown">
            <span class="big"><a href="${status.config.parentWaterfall}" target="_blank">${status.config.parentWaterfall}</a></span>
          </div>
        </div>
      ` : html``}
      <div class="tr">
        <div class="td nowrap">Current Mode:</div>
        <div class="td nowrap unknown">
          <span class="big">${status.mode.mode}</span>
        </div>
      </div>
      <div class="tr">
        <div class="td nowrap">Set By:</div>
        <div class="td nowrap unknown">
          ${status.mode.user}${status.mode.message ? html`: ${status.mode.message}`: html``}
        </div>
      </div>
      <div class="tr">
        <div class="td nowrap">Change Mode:</div>
        <div class="td nowrap">
          ${ele.modeButtons.map((button) => html`
            <button
                class="${button.class}"
                on-tap="_modeButtonPressed"
                disabled=${!ele.editRights || ele.modeChangePending}
                title="${ele.editRights ? "Change the mode." : ele.pleaseLoginMsg}"
                value="${button.value}">
              ${button.label}
            </button>
          `)}
          <paper-spinner active="${ele.modeChangePending}"></paper-spinner>
        </div>
      </div>
      <div class="tr">
        <div class="td nowrap">Status:</div>
        <div class="td nowrap">
          <span class="${ele._statusClass(status.status)}"><span class="big">${status.status}</span></span>
          ${status.status.indexOf("throttle") >= 0 ? html`
            <span>until <human-date-sk date="${ele.throttledUntil}" seconds></human-date-sk></span>
            <button
                on-tap="_unthrottle"
                disabled="${!ele.editRights}"
                title="${ele.editRights ? "Unthrottle the roller." : ele.pleaseLoginMsg}">
              Force Unthrottle
            </button>
          ` : html``}
          ${status.status.indexOf("waiting for roll window") >= 0 ? html`
            <span>until <human-date-sk date="${ele.rollWindowStart}"></human-date-sk></span>
          ` : html``}
        </div>
      </div>
      ${ele.editRights && status.error ? html`
        <div class="tr">
          <div class="td nowrap">Error:</div>
          <div class="td"><pre>${status.error}</pre></div>
        </div>
      ` : html``}
      <div class="tr">
        <div class="td nowrap">Current Roll:</div>
        <div class="td">
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
        </div>
      </div>
      ${status.lastRoll ? html`
        <div class="tr">
          <div class="td nowrap">Previous roll result:</div>
          <div class="td">
            <span class="${ele._rollClass(status.lastRoll)}">${ele._rollResult(status.lastRoll)}</span>
            <a href="${ele._issueURL(status.lastRoll)}" target="_blank" class="small">(detail)</a>
          </div>
        </div>
      ` : html``}
      <div class="tr">
        <div class="td nowrap">History:</div>
        <div class="td">
          <div class="table">
            <div class="tr">
              <div class="th">Roll</div>
              <div class="th">Last Modified</div>
              <div class="th">Result</div>
            </div>
            ${status.recent.map((roll: Roll) => html`
              <div class="tr">
                <div class="td"><a href="${ele._issueURL(roll)}" target="_blank">${roll.subject}</a></div>
                <div class="td"><human-date-sk date="${roll.modified}" diff></human-date-sk> ago</div>
                <div class="td"><span class="${ele._rollClass(roll)}">${ele._rollResult(roll)}</span></div>
              </div>
            `)}
          </div>
        </div>
      </div>
      <div class="tr">
        <div class="td nowrap">Full History:</div>
        <div class="td">
          <a href="${status.fullHistoryUrl}" target="_blank">
            ${status.fullHistoryUrl}
          </a>
        </div>
      </div>
      <div class="tr">
        <div class="td nowrap">Strategy for choosing next roll revision:</div>
        <div class="td nowrap">
          <select
              id="strategyDropDown"
              disabled="${!ele.editRights}"
              title="${ele.editRights ? "Change the strategy for choosing the next revision to roll." : ele.pleaseLoginMsg}"
              @change="${ele._selectedStrategyChanged}">
            ${status.validStrategies.map((strategy: string) => html`
              <option value="${strategy}" ?selected="${strategy == status.strategy.strategy}">${strategy}</option>
            `)}
          </select>
          <paper-spinner active="${ele.strategyChangePending}"></paper-spinner>
        </div>
      </div>
      <div class="tr">
        <div class="td nowrap">Set By:</div>
        <div class="td nowrap unknown">
          ${status.strategy.user}${status.strategy.message ? html`: ${status.strategy.message}` : html``}
        </div>
      </div>
    </div>
  </section>
  <section class="manual">
    <div class="table">
      ${status.config.supportsManualRolls ? html`
        ${!ele.rollCandidates ? html`
          The roller is up to date; there are no revisions which could be manually rolled.
        ` : html``}
        <div class="tr">
          <div class="th">Revision</div>
          <div class="th">Description</div>
          <div class="th">Timestamp</div>
          <div class="th">Requester</div>
          <div class="th">Requested at</div>
          <div class="th">Roll</div>
        </div>
        ${ele.rollCandidates.map((rollCandidate) => html`
          <div class="tr rollCandidate">
            <div class="td">
              ${rollCandidate.revision.url ? html`
                <a href="${rollCandidate.revision.url}" target="_blank">${rollCandidate.revision.display}</a>
              ` : html`
                ${rollCandidate.revision.display}
              `}
            </div>
            <div class="td">${!!rollCandidate.revision.description ? sk.truncate(rollCandidate.revision.description, 100) : html``}</div>
            <div class="td"><human-date-sk date="${rollCandidate.revision.timestamp}"></human-date-sk></div>
            <div class="td">${rollCandidate.roll ? rollCandidate.roll.requester : html``}</div>
            <div class="td">${rollCandidate.roll ? html`<human-date-sk date="${rollCandidate.roll.timestamp}"></human-date-sk>` : html``}</div>
            <div class="td">
              ${rollCandidate.roll && rollCandidate.roll.url ? html`
                <a href="${rollCandidate.roll.url}", target="_blank">${rollCandidate.roll.url}</a>
              ` : html``}
              ${!!rollCandidate.roll && !rollCandidate.roll.url && rollCandidate.roll.status ? rollCandidate.roll.status : html``}
              ${!rollCandidate.roll ? html`
                <button
                    @click="${() => {ele._requestManualRoll(rollCandidate.revision.id)}}"
                    data-rev="${rollCandidate.revision.id}"
                    class="requestRoll"
                    disabled=${!ele.editRights}
                    title="${ele.editRights ? "Request a roll to this revision." : ele.pleaseLoginMsg}">
                  Request Roll
                </button>
              ` : html``}
              ${!!rollCandidate.roll && !!rollCandidate.roll.result ? html`
                <span class="${ele._reqResultClass(rollCandidate.roll)}">${rollCandidate.roll.result}</span>
              ` : html``}
            </div>
          </div>
        `)}
        <div class="tr rollCandidate">
          <div class="td">
            <input id="manualRollRevInput" label="type revision/ref"></input>
          </div>
          <div class="td"><!-- no description        --></div>
          <div class="td"><!-- no revision timestamp --></div>
          <div class="td"><!-- no requester          --></div>
          <div class="td"><!-- no request timestamp  --></div>
          <div class="td">
            <button
                @click="${() => {
                  ele._requestManualRoll((<HTMLInputElement>$$("#manualRollRevInput")).value);
                }}"
                class="requestRoll"
                disabled=${!ele.editRights}
                title="${ele.editRights ? "Request a roll to this revision." : ele.pleaseLoginMsg}">
              Request Roll
            </button>
          </div>
        </div>
      ` : html`
        This roller does not support manual rolls. If you want this feature,
        update the config file for the roller to enable it. Note that some
        rollers cannot support manual rolls for technical reasons.
      `}
    </div>
  </section>
  <dialog id="modeChangeDialog">
    <h2>Enter a message:</h2>
    <input type="text" id="mode_change_msg"></input>
    <button @click="${() => {ele._changeMode(false)}}">Cancel</button>
    <button @click="${() => {ele._changeMode(true)}}">Submit</button>
  </dialog>
  <dialog id="strategyChangeDialog">
    <h2>Enter a message:</h2>
    <input type="text" id="strategy_change_msg"></input>
    <button @click="${() => {ele._changeStrategy(false)}}">Cancel</button>
    <button @click="${() => {ele._changeStrategy(true)}}">Submit</button>
  </dialog>
  <url-param-sk name="tab" value="{{selectedTab}}" default="status"></url-param-sk>
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
  private _manualRollRevInput: HTMLInputElement | null;
  private _modeChangeDialog: HTMLDialogElement | null;
  private _modeChangePending: boolean;
  private _modeButtons: any[];
  private _modeChangeMsgInput: HTMLInputElement | null;
  private readonly _pleaseLoginMsg = "Please login to make changes.";
  private _refreshInterval = 60;
  private _refreshIntervalInput: HTMLInputElement | null;
  private _rollCandidates: RollCandidate[];
  private _rollWindowStart: Date;
  private _selectedMode: string;
  private _status: Status;
  private _strategyChangeDialog: HTMLDialogElement | null;
  private _strategyChangeMsgInput: HTMLInputElement | null;
  private _strategyChangePending: boolean;
  private _strategySelect: HTMLSelectElement | null;
  private _throttledUntil: Date;
  private _timeout: number;

  constructor() {
    super();
    this._editRights = false;
    this._lastLoaded = new Date(0);
    this._manualRollRevInput = null;
    this._modeChangeDialog = null;
    this._modeChangePending = false;
    this._modeChangeMsgInput = null;
    this._modeButtons = [];
    this._refreshIntervalInput = null;
    this._rollCandidates = [];
    this._rollWindowStart = new Date(0);
    this._selectedMode = "";
    this._status = new Status();
    this._strategyChangeDialog = null;
    this._strategyChangeMsgInput = null;
    this._strategyChangePending = false;
    this._strategySelect = null;
    this._throttledUntil = new Date(0);
    this._timeout = 0;
  }

  get editRights() {
    return this._editRights;
  }
  get lastLoaded() {
    return this._lastLoaded;
  }
  get modeButtons() {
    return this._modeButtons;
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
  get throttledUntil() {
    return this._throttledUntil;
  }

  ready() {
    this._manualRollRevInput = $$("#manualRollRevInput", this);
    this._modeChangeDialog = $$("#modeChangeDialog", this);
    //dialogPolyfill.registerDialog(this._modeChangeDialog);
    this._refreshIntervalInput = $$("#refreshInterval", this);
    this._strategyChangeDialog = $$("#strategyChangeDialog", this);
    this._strategySelect = $$("#strategySelect", this);
    Login.then((loginstatus: any) => {
      this._editRights = loginstatus.IsAGoogler;
    });
    this._reload();
  }

  _modeButtonPressed(mode: string) {
    if (!this._editRights) {
      errorMessage("You must be logged in with an @google.com account to set the ARB mode.");
      return
    }
    if (mode == this._status.mode.mode) {
      return;
    }
    this._selectedMode = mode;
    this._modeChangeDialog?.showModal();
  }

  _changeMode(submit: boolean) {
    this._modeChangeDialog?.close();
    if (!submit) {
      this._selectedMode = "";
      return;
    }
    if (!this._modeChangeMsgInput) {
      return;
    }
    errorMessage("Mode change in progress. This may take some time.");
    this._modeChangePending = true;
    let mode = new Mode();
    mode.message = this._modeChangeMsgInput.value;
    mode.mode = this._selectedMode;
    fetch(window.location.pathname + "/json/mode", {
      method: "POST",
      body: JSON.stringify(mode),
      headers: {
        'Content-Type': 'application/json',
      },
    }).then(jsonOrThrow).then((json) => {
      this._update(json);
      this._modeChangePending = false;
      if (!!this._modeChangeMsgInput) {
        this._modeChangeMsgInput.value = "";
      }
      errorMessage("Success!");
    }, (err) => {
      this._modeChangePending = false;
      errorMessage("Failed to change the mode: " + err.response);
    });
  }

  _changeStrategy(submit: boolean) {
    this._strategyChangeDialog?.close();
    if (!submit) {
      if (!!this._strategySelect) {
        this._strategySelect.value = this._status.strategy.strategy;
      }
      return;
    }
    if (!this._strategyChangeMsgInput || !this._strategySelect) {
      return;
    }
    let strategy = new Strategy();
    strategy.message = this._strategyChangeMsgInput.value;
    strategy.strategy = this._strategySelect.value;
    errorMessage("Strategy change in progress. This may take some time.");
    fetch(window.location.pathname + "/json/strategy", {
      method: "POST",
      body: JSON.stringify(strategy),
      headers: {
        'Content-Type': 'application/json',
      },
    }).then(jsonOrThrow).then((json) => {
      this._update(json);
      this._strategyChangePending = false;
      if (!!this._strategyChangeMsgInput) {
        this._strategyChangeMsgInput.value = "";
      }
      errorMessage("Success!");
    }, (err) => {
      this._strategyChangePending = false;
      errorMessage("Failed to change the strategy: " + err.response);
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
    if (this._refreshIntervalInput) {
      this._refreshInterval = this._refreshIntervalInput.valueAsNumber;
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

    fetch(url).then(jsonOrThrow).then((json) => {
      this._update(json);
    }).catch((err) => {
      errorMessage("Failed to load status: " + err.response);
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
    var url = window.location.pathname + "/json/manual";
    var req: ManualRollRequest = {
      result: "",
      revision: rev,
      requester: "",
      rollerName: "",
      status: "",
      timestamp: "",
      url: "",
    };
    fetch(url, {
      method: "POST",
      body: JSON.stringify(req),
      headers: {
        'Content-Type': 'application/json',
      },
    }).then(jsonOrThrow).then((json) => {
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
      if (this._manualRollRevInput) {
        this._manualRollRevInput.value = "";
      }
      this._render();
      errorMessage("Successfully requested manual roll.");
    }, (err) => {
      errorMessage("Failed to request manual roll: " + err.response);
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
    if (this._strategySelect?.value == this._status.strategy.strategy) {
      return;
    }
    if (!this._editRights) {
      errorMessage("You must be logged in with an @google.com account to set the ARB strategy.");
      return
    }
    this._strategyChangeDialog?.showModal();
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
    fetch(url, {method: "POST"}).then(() => {
      errorMessage("Successfully unthrottled the roller. May take a minute or so to start up.")
    }).catch((err) => {
      errorMessage("Failed to unthrottle: " + err.response);
    });
  }

  _update(status: Status) {
    var modeButtons = [];
    for (var i = 0; i < status.validModes.length; i++) {
      var m = status.validModes[i];
      if (m != status.mode.mode) {
        modeButtons.push({
          "label": this._getModeButtonLabel(status.mode.mode, m),
          "value": m,
        });
      }
    }
    this._modeButtons = modeButtons;

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
    this._rollCandidates = rollCandidates;
    this._lastLoaded = new Date();
    this._resetTimeout();
    console.log("Reloaded status.");
  }

  _render() {
    render(template(this, this._status), this, {eventContext: this});
  }
}

define('arb-status-sk', ARBStatusSk);