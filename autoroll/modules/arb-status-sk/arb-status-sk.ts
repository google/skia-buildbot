/**
 * @module autoroll/modules/arb-status-sk
 * @description <h2><code>arb-status-sk</code></h2>
 *
 * <p>
 * This element displays the status of a single Autoroller.
 * </p>
 */

import { html } from 'lit-html'

import { $$ } from 'common-sk/modules/dom';
import { diffDate, localeTime } from 'common-sk/modules/human';

import { define } from 'elements-sk/define'
import 'elements-sk/styles/buttons';
import 'elements-sk/styles/select';
import 'elements-sk/styles/table';
import 'elements-sk/tabs-panel-sk';
import 'elements-sk/tabs-sk';

import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { LoginTo } from '../../../infra-sk/modules/login';
import { truncate } from '../../../infra-sk/modules/string';

import {
  AutoRollConfig,
  AutoRollCL,
  AutoRollRPCs,
  AutoRollStatus,
  GetAutoRollRPCs,
  ManualRollRequest,
  ModeChange,
  Revision,
  StrategyChange,
  TryResult,
} from '../rpc';

interface RollCandidate{
  revision: Revision;
  roll: ManualRollRequest | null;
}

export class ARBStatusSk extends ElementSk {
  private static template = (ele: ARBStatusSk) => !ele.status? html`` : html`
  <tabs-sk>
    <button value="status">Roller Status</button>
    <button value="manual">Trigger Manual Rolls</button>
  </tabs-sk>
  ${!ele.editRights ? html`
    <div id="pleaseLoginMsg" class="big">${ele.pleaseLoginMsg}</div>
  ` : html``}
  <tabs-panel-sk selected="0">
    <div class="status">
      <div id="loadstatus">
        Reload (s)
        <input
            id="refreshInterval"
            type="number"
            value="${ele.refreshInterval}"
            label="Reload (s)"
            @input=${ele.reloadChanged}
            ></input>
        Last loaded at <span>${localeTime(ele.lastLoaded)}</span>
      </div>
      <table>
        ${ele.status.config?.parentwaterfall ? html`
          <tr>
            <td class="nowrap">Parent Repo Build Status</td>
            <td class="nowrap unknown">
              <span>
                <a href="${ele.status.config.parentwaterfall}" target="_blank">
                  ${ele.status.config.parentwaterfall}
                </a>
              </span>
            </td>
          </tr>
        ` : html``}
        <tr>
          <td class="nowrap">Current Mode:</td>
          <td class="nowrap unknown">
            <span class="big">${ele.status.mode?.mode}</span>
          </td>
        </tr>
        <tr>
          <td class="nowrap">Set By:</td>
          <td class="nowrap unknown">
            ${ele.status.mode?.user}${ele.status.mode?.message
                ? html`: ${ele.status.mode.message}`: html``}
          </td>
        </tr>
        <tr>
          <td class="nowrap">Change Mode:</td>
          <td class="nowrap">
            ${ele.status.validmodes?.map((mode: string) => mode == ele.status?.mode?.mode
                  ? "" : html`
              <button
                  @click="${() => {ele.modeButtonPressed(mode)}}"
                  ?disabled="${!ele.editRights || ele.modeChangePending}"
                  title="${ele.editRights ? "Change the mode." : ele.pleaseLoginMsg}"
                  value="${mode}">
                ${ele.status?.mode?.mode ? ele.getModeButtonLabel(ele.status.mode.mode, mode) : ""}
              </button>
            `)}
          </td>
        </tr>
        <tr>
          <td class="nowrap">Status:</td>
          <td class="nowrap">
            <span class="${ele.statusClass(ele.status.status)}">
              <span class="big">${ele.status.status}</span>
            </span>
            ${ele.status.status.indexOf("throttle") >= 0 ? html`
              <span>until ${localeTime(new Date(ele.status.throttleduntil * 1000))}</span>
              <button
                  @click="${ele.unthrottle}"
                  ?disabled="${!ele.editRights}"
                  title="${ele.editRights ? "Unthrottle the roller." : ele.pleaseLoginMsg}">
                Force Unthrottle
              </button>
            ` : html``}
            ${ele.status.status.indexOf("waiting for roll window") >= 0 ? html`
              <span>until ${localeTime(ele.rollWindowStart)}</span>
            ` : html``}
          </td>
        </tr>
        ${ele.editRights && ele.status.error ? html`
          <tr>
            <td class="nowrap">Error:</td>
            <td><pre>${ele.status.error}</pre></td>
          </tr>
        ` : html``}
        <tr>
          <td class="nowrap">Current Roll:</td>
          <td>
            <div>
              ${ele.status.currentroll ? html`
                <a href="${ele.issueURL(ele.status.currentroll)}" class="big" target="_blank">
                  ${ele.status.currentroll.subject}
                </a>
              ` : html`<span>(none)</span>`}
            </div>
            <div>
              ${ele.status.currentroll && ele.status.currentroll.tryresults ?
                  ele.status.currentroll.tryresults.map((tryResult) => html`
                <div class="trybot">
                  ${tryResult.url ? html`
                    <a href="${tryResult.url}"
                        class="${ele.trybotClass(tryResult)}"
                        target="_blank">
                      ${tryResult.name}
                    </a>
                  ` : html`
                    <span class="nowrap"
                        class="${ele.trybotClass(tryResult)}">
                      ${tryResult.name}
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
        ${ele.status.lastroll ? html`
          <tr>
            <td class="nowrap">Previous roll result:</td>
            <td>
              <span class="${ele.rollClass(ele.status.lastroll)}">
                ${ele.rollResult(ele.status.lastroll)}
              </span>
              <a href="${ele.issueURL(ele.status.lastroll)}" target="_blank" class="small">
                (detail)
              </a>
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
              ${ele.status.recent?.map((roll: AutoRollCL) => html`
                <tr>
                  <td><a href="${ele.issueURL(roll)}" target="_blank">${roll.subject}</a></td>
                  <td>${diffDate(roll.modified * 1000)} ago</td>
                  <td><span class="${ele.rollClass(roll)}">${ele.rollResult(roll)}</span></td>
                </tr>
              `)}
            </table>
          </td>
        </tr>
        <tr>
          <td class="nowrap">Full History:</td>
          <td>
            <a href="${ele.status.fullhistoryurl}" target="_blank">
              ${ele.status.fullhistoryurl}
            </a>
          </td>
        </tr>
        <tr>
          <td class="nowrap">Strategy for choosing next roll revision:</td>
          <td class="nowrap">
            <select
                id="strategySelect"
                ?disabled="${!ele.editRights || ele.strategyChangePending}"
                title="${ele.editRights
                    ? "Change the strategy for choosing the next revision to roll."
                    : ele.pleaseLoginMsg}"
                @change="${ele.selectedStrategyChanged}">
              ${ele.status?.validstrategies?.map((strategy: string) => html`
                <option
                    value="${strategy}"
                    ?selected="${strategy == ele.status?.strategy?.strategy}">
                  ${strategy}
                </option>
              `)}
            </select>
          </td>
        </tr>
        <tr>
          <td class="nowrap">Set By:</td>
          <td class="nowrap unknown">
            ${ele.status.strategy?.user}${ele.status.strategy?.message ?
                html`: ${ele.status.strategy.message}` : html``}
          </td>
        </tr>
      </table>
    </div>
    <div class="manual">
      <table>
        ${ele.status.config?.supportsmanualrolls ? html`
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
                  <a href="${rollCandidate.revision.url}" target="_blank">
                    ${rollCandidate.revision.display}
                  </a>
                ` : html`
                  ${rollCandidate.revision.display}
                `}
              </td>
              <td>${!!rollCandidate.revision.description ?
                        truncate(rollCandidate.revision.description, 100) : html``}
              </td>
              <td>${localeTime(new Date(rollCandidate.revision.time))}</td>
              <td>${rollCandidate.roll ? rollCandidate.roll.requester : html``}</td>
              <td>${rollCandidate.roll ?
                        localeTime(new Date(rollCandidate.roll.timestamp)): html``}</td>
              <td>
                ${rollCandidate.roll && rollCandidate.roll.url ? html`
                  <a href="${rollCandidate.roll.url}", target="_blank">
                    ${rollCandidate.roll.url}
                  </a>
                ` : html``}
                ${!!rollCandidate.roll && !rollCandidate.roll.url && rollCandidate.roll.status ?
                      rollCandidate.roll.status : html``}
                ${!rollCandidate.roll ? html`
                  <button
                      @click="${() => {ele.requestManualRoll(rollCandidate.revision.id)}}"
                      class="requestRoll"
                      ?disabled=${!ele.editRights}
                      title="${ele.editRights ?
                                   "Request a roll to this revision." : ele.pleaseLoginMsg}">
                    Request Roll
                  </button>
                ` : html``}
                ${!!rollCandidate.roll && !!rollCandidate.roll.result ? html`
                  <span class="${ele.reqResultClass(rollCandidate.roll)}">
                      ${rollCandidate.roll.result}
                  </span>
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
                    ele.requestManualRoll($$<HTMLInputElement>("#manualRollRevInput")!.value);
                  }}"
                  class="requestRoll"
                  ?disabled=${!ele.editRights}
                  title="${ele.editRights ?
                               "Request a roll to this revision." : ele.pleaseLoginMsg}">
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
  <dialog id="modeChangeDialog" class=surface-themes-sk>
    <h2>Enter a message:</h2>
    <input type="text" id="modeChangeMsgInput"></input>
    <button @click="${() => {ele.changeMode(false)}}">Cancel</button>
    <button @click="${() => {ele.changeMode(true)}}">Submit</button>
  </dialog>
  <dialog id="strategyChangeDialog" class=surface-themes-sk>
    <h2>Enter a message:</h2>
    <input type="text" id="strategyChangeMsgInput"></input>
    <button @click="${() => {ele.changeStrategy(false)}}">Cancel</button>
    <button @click="${() => {ele.changeStrategy(true)}}">Submit</button>
  </dialog>
`;

  private editRights: boolean = false;
  private lastLoaded: Date = new Date(0);
  private modeChangePending: boolean = false;
  private readonly pleaseLoginMsg = "Please login to make changes.";
  private refreshInterval = 60;
  private rollCandidates: RollCandidate[] = [];
  private rollWindowStart: Date = new Date(0);
  private rpc: AutoRollRPCs = GetAutoRollRPCs(this);
  private selectedMode: string = "";
  private status: AutoRollStatus | null = null;
  private strategyChangePending: boolean = false;
  private timeout: number = 0;

  constructor() {
    super(ARBStatusSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._upgradeProperty("roller");
    this._render();
    LoginTo("/loginstatus/").then((loginstatus: any) => {
      this.editRights = loginstatus.IsAGoogler;
      this._render();
    });
    this.reload();
  }

  get roller() {
    return <string>this.getAttribute('roller');
  }
  set roller(v: string) {
    this.setAttribute('roller', v);
    this.reload();
  }

  private modeButtonPressed(mode: string) {
    if (mode == this.status?.mode?.mode) {
      return;
    }
    this.selectedMode = mode;
    $$<HTMLDialogElement>("#modeChangeDialog", this)!.showModal();
  }

  private fetch(input: RequestInfo, init?: RequestInit | undefined): Promise<any> {
    console.log("fetching");
    console.log(input);
    return fetch(input, init);
  }

  private changeMode(submit: boolean) {
    $$<HTMLDialogElement>("#modeChangeDialog", this)!.close();
    if (!submit) {
      this.selectedMode = "";
      return;
    }
    const modeChangeMsgInput = <HTMLInputElement>$$("#modeChangeMsgInput", this);
    if (!modeChangeMsgInput) {
      return;
    }
    this.modeChangePending = true;
    this.rpc.edit_SetMode({
      message: modeChangeMsgInput.value,
      mode: this.selectedMode,
      roller: this.roller,
    }).then((status: AutoRollStatus) => {
      this.modeChangePending = false;
      modeChangeMsgInput.value = "";
      this.update(status);
    }, () => {
      this.modeChangePending = false;
      this._render();
    });
  }

  private changeStrategy(submit: boolean) {
    $$<HTMLDialogElement>("#strategyChangeDialog", this)!.close();
    const strategySelect = <HTMLSelectElement>$$("#strategySelect");
    const strategyChangeMsgInput = <HTMLInputElement>$$("#strategyChangeMsgInput");
    if (!submit) {
      if (!!strategySelect && !!this.status?.strategy) {
        strategySelect.value = this.status?.strategy.strategy;
      }
      return;
    }
    if (!strategyChangeMsgInput || !strategySelect) {
      return;
    }
    this.strategyChangePending = true;
    this.rpc.edit_SetStrategy({
      message: strategyChangeMsgInput.value,
      roller: this.roller,
      strategy: strategySelect.value,
    }).then((status: AutoRollStatus) => {
      this.strategyChangePending = false;
      strategyChangeMsgInput.value = "";
      this.update(status);
    }, () => {
      this.strategyChangePending = false;
      if (this.status?.strategy?.strategy) {
        strategySelect!.value = this.status.strategy.strategy;
      }
      this._render();
    });
  }

  // computeRollWindowStart returns a string indicating when the configured
  // roll window will start. If errors are encountered, in particular those
  // relating to parsing the roll window, the returned string will contain
  // the error.
  private computeRollWindowStart(config: AutoRollConfig) {
    if (!config || !config.timewindow) {
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
    const split = config.timewindow.split(";");
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
      const dayOffsetMs = (w.day - next.getUTCDay()) * 24 * 60 * 60 * 1000;
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

  private issueURL(roll: AutoRollCL): string {
    if (roll) {
      return (this.status?.issueurlbase || "") + roll.id;
    }
    return "";
  }

  private getModeButtonLabel(currentMode: string, mode: string) {
    // TODO(borenet): This is a hack; it doesn't respect this.validmodes.
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

  private reloadChanged() {
    const refreshIntervalInput = <HTMLInputElement>$$("refreshIntervalInput", this);
    if (refreshIntervalInput) {
      this.refreshInterval = refreshIntervalInput.valueAsNumber;
      this.resetTimeout();
    }
  }

  private resetTimeout() {
    if (this.timeout) {
      window.clearTimeout(this.timeout);
    }
    if (this.refreshInterval > 0) {
      this.timeout = window.setTimeout(() => {
        this.reload();
      }, this.refreshInterval * 1000);
    }
  }

  private reload() {
    console.log("Loading status...");
    this.rpc.view_GetStatus({
      roller: this.roller,
    }).then((status: AutoRollStatus) => {
      this.update(status);
      this.resetTimeout();
    }).catch((err: any) => {
      this.resetTimeout();
    });
  }

  private reqResultClass(req: ManualRollRequest) {
    if (!req) {
      return "";
    }
    const manualRequestResultClass: {[key:string]:string} = {
      "SUCCESS": "fg-success",
      "FAILURE": "fg-failure",
    }
    return manualRequestResultClass[req.result || ""];
  }

  private requestManualRoll(rev: string) {
    this.rpc.edit_CreateManualRoll({
      revision: rev,
      roller: this.roller,
    }).then((req: ManualRollRequest) => {
      const exist = this.rollCandidates.find((r) => r.revision.id == req.revision);
      if (!!exist) {
        exist.roll = req;
      } else {
        this.rollCandidates.push({
          revision: {
            description: "",
            display: req.revision,
            id: req.revision,
            time: 0,
            url: "",
          },
          roll: req,
        });
      }
      const manualRollRevInput = <HTMLInputElement>$$("#manualRollRevInput");
      if (!!manualRollRevInput) {
        manualRollRevInput.value = "";
      }
      this._render();
    });
  }

  private rollClass(roll: AutoRollCL) {
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

  private rollResult(roll: AutoRollCL) {
    if (!roll) {
      return "unknown";
    }
    return roll.result;
  }

  private statusClass(status: string) {
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

  private selectedStrategyChanged() {
    if ($$<HTMLSelectElement>("strategySelect", this)!.value == this.status?.strategy?.strategy) {
      return;
    }
    $$<HTMLDialogElement>("#strategyChangeDialog", this)!.showModal();
  }

  private trybotClass(trybot: TryResult) {
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

  private unthrottle() {
    this.rpc.edit_Unthrottle({
      roller: this.roller,
    });
  }

  private update(status: AutoRollStatus) {
    const rollCandidates: RollCandidate[] = [];
    const manualByRev: {[key:string]:ManualRollRequest} = {};
    if (status.notrolledrevisions) {
      if (status.manualrequests) {
        for (let i = 0; i < status.manualrequests.length; i++) {
          const req = status.manualrequests[i];
          manualByRev[req.revision] = req;
        }
      }
      for (let i = 0; i < status.notrolledrevisions.length; i++) {
        const rev = status.notrolledrevisions[i];
        const candidate: RollCandidate = {
          revision: rev,
          roll: null,
        };
        let req = manualByRev[rev.id];
        delete manualByRev[rev.id];
        if (!req && status.currentroll && status.currentroll.rollingto == rev.id) {
          req = {
            dryrun: false,
            id: "",
            noemail: false,
            noresolverevision: false,
            requester: "autoroller",
            result: "",
            roller: "",
            revision: "",
            status: "STARTED",
            timestamp: status.currentroll.created,
            url: this.issueURL(status.currentroll),
          };
        }
        candidate.roll = req;
        rollCandidates.push(candidate);
      }
    }
    for (const key in manualByRev) {
      const req = manualByRev[key];
        const rev: Revision = {
          description: "",
          display: req.revision,
          id: req.revision,
          time: 0,
          url: "",
        };
      rollCandidates.push({
        revision: rev,
        roll: req,
      });
    };
    this.lastLoaded = new Date();
    this.rollCandidates = rollCandidates;
    if (status.config) {
      this.rollWindowStart = this.computeRollWindowStart(status.config);
    }
    this.status = status;
    console.log("Reloaded status.");
    this._render();
  }
}

define('arb-status-sk', ARBStatusSk);