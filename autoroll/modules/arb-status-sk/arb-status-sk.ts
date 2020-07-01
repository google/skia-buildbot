/**
 * @module autoroll/modules/arb-status-sk
 * @description <h2><code>arb-status-sk</code></h2>
 *
 * <p>
 * This element displays the status of a single Autoroller.
 * </p>
 */
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { define } from 'elements-sk/define'
import { errorMessage } from 'elements-sk/errorMessage';
import { upgradeProperty } from 'elements-sk/upgradeProperty'
import { Login } from '../../../infra-sk/modules/login'
import { html, render } from 'lit-html'

const template = (ele: ARBStatusSk, status: ARBStatus) => html`
  <style include="iron-flex iron-flex-alignment iron-positioning styles-sk">
    :host {
      font-family: sans-serif;
    }
    #loadstatus {
      font-size: 0.8em;
      padding: 0px 15px;
    }
    #tabsContainer {
      width: 400px;
    }
    #pleaseLoginMsg {
      color: red;
      padding: 10px;
    }
    a,a.visited {
      color: #1f78b4;
    }

    paper-tabs {
      background: #A6CEE3;
      color: #555;
    }
    paper-tab.iron-selected {
      background: #1F78B4;
      color: white;
      border: #1F78B4 solid 2px;
    }
    paper-tab {
      border: white solid 2px;
    }

    .big {
      font-size: 1.3em;
    }
    .small {
      font-size: 0.8em;
    }
    .hidden {
      display: none;
    }
    .nowrap {
      white-space: nowrap;
    }
    .rollCandidate {
      height: 80px;
    }
    .trybot {
      margin: 5px;
    }
  </style>
    <div id="tabsContainer">
    <paper-tabs id="tabs" attr-for-selected="id" selected="{{selectedTab}}" no-bar>
      <paper-tab id="status">Roller Status</paper-tab>
      <paper-tab id="manual">Trigger Manual Rolls</paper-tab>
    </paper-tabs>
    </div>
    ${ele._editRights ? html`
      <div id="pleaseLoginMsg">Please log in with an @google.com account to make changes.</div>
      ` : html``}
    <div id="statusDisplay">
      <div class="horizontal layout center" id="loadstatus">
        <paper-input type="number" value="{{reload}}" label="Reload (s)" prevent-invalid-input></paper-input>
        <div class="flex"></div>
        <div>Last loaded at <span>${ele._lastLoaded}</span></div>
      </div>
      <div class="table">
        ${ele._parentWaterfall ? html`
          <div class="tr">
            <div class="td nowrap">Parent Repo Build Status</div>
            <div class="td nowrap unknown">
              <span class="big"><a href="${ele._parentWaterfall}" target="_blank">${ele._parentWaterfall}</a></span>
            </div>
          </div>
        ` : html``}
        <div class="tr">
          <div class="td nowrap">Current Mode:</div>
          <div class="td nowrap unknown">
            <span class="big">${ele._mode}</span>
          </div>
        </div>
        <div class="tr">
          <div class="td nowrap">Set By:</div>
          <div class="td nowrap unknown">
            ${ele._modeChangeBy}${ele._modeChangeMsg ? html`: ${ele._modeChangeMsg}`: html``}
          </div>
        </div>
        <div class="tr">
          <div class="td nowrap">Change Mode:</div>
          <div class="td nowrap">
            ${ele._modeButtons.map((button) => html`
              <button
                  class="${button.class}"
                  on-tap="_modeButtonPressed"
                  disabled=${ele._computeModeChangeDisabled()}
                  title="${ele._computeTitle('Change the mode.')}"
                  value="${button.value}">
                ${button.label}
              </button>
            `)}
            <paper-spinner active="${ele._modeChangePending}"></paper-spinner>
          </div>
        </div>
        <div class="tr">
          <div class="td nowrap">Status:</div>
          <div class="td nowrap">
            <span class="${ele._statusClass(ele._status)}"><span class="big">${ele._status}</span></span>
            ${ele._isThrottled(ele._status) ? html`
              <span>until <human-date-sk date="${ele._throttledUntil}" seconds></human-date-sk></span>
              <button on-tap="_unthrottle" disabled="${!ele._editRights}" title="${ele._computeTitle(ele._editRights, 'Unthrottle the roller.')}">Force Unthrottle</button>
            ` : html``}
            ${ele._isWaitingForRollWindow(ele._status) ? html`
              <span>until <human-date-sk date="${ele._rollWindowStart}"></human-date-sk></span>
            ` : html``}
          </div>
        </div>
        ${ele._computeShowError(ele._editRights) ? html`
          <div class="tr">
            <div class="td nowrap">Error:</div>
            <div class="td"><pre>${ele._error}</pre></div>
          </div>
        ` : html``}
        <div class="tr">
          <div class="td nowrap">Current Roll:</div>
          <div class="td">
            <div>
              ${ele._currentRoll ? html`
                <a href="${ele._issueURL(ele._currentRoll)}" class="big" target="_blank">${ele._currentRoll.subject}</a>
              ` : html`<span>(none)</span>`}
            </div>
            <div>
              ${ele._currentRoll ? ele._currentRoll.tryResults.map((tryResult) => html`
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
                  ${ele._isCQ(tryResult) ? html`` : html`
                    <span class="nowrap small">(${tryResult.category})</span>
                  `}
                </div>
              `) : html``}
            </div>
          </div>
        </div>
        ${ele._lastRoll ? html`
          <div class="tr">
            <div class="td nowrap">Previous roll result:</div>
            <div class="td">
              <span class="${ele._rollClass(ele._lastRoll)}">${ele._rollResult(ele._lastRoll)}</span>
              <a href="${ele._issueURL(ele._lastRoll)}" target="_blank" class="small">(detail)</a>
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
              ${ele._recent.map((item) => html`
                <div class="tr">
                  <div class="td"><a href="${ele._issueURL(item)}" target="_blank">${item.subject}</a></div>
                  <div class="td"><human-date-sk date="${item.modified}" diff></human-date-sk> ago</div>
                  <div class="td"><span class="${ele._rollClass(item)}">${ele._rollResult(item)}</span></div>
                </div>
              `)}
            </div>
          </div>
        </div>
        <div class="tr">
          <div class="td nowrap">Full History:</div>
          <div class="td">
            <a href="${ele._fullHistoryUrl}" target="_blank">
              ${ele._fullHistoryUrl}
            </a>
          </div>
        </div>
        <div class="tr">
          <div class="td nowrap">Strategy for choosing next roll revision:</div>
          <div class="td nowrap">
            <paper-dropdown-menu
                id="strategyDropDown"
                disabled="${!ele._editRights}"
                title="${ele._computeTitle(ele._editRights, 'Change the strategy for choosing the next revision to roll.')}">
              <paper-listbox id="strategyMenu" class="dropdown-content" selected="${ele._selectedStrategy}" id="strategy_listbox" attr-for-selected="value">
                ${ele._validStrategies.map((strategy) => html`
                  <paper-item value="${strategy}">${strategy}</paper-item>
                `)}
              </paper-listbox>
            </paper-dropdown-menu>
          </div>
        </div>
        <div class="tr">
          <div class="td nowrap">Set By:</div>
          <div class="td nowrap unknown">
            ${ele._strategyChangeBy}${ele._strategyChangeMsg ? html`: ${ele._strategyChangeMsg}` : html``}
          </div>
        </div>
      </div>
    </div>
    <div id="rollCandidates" class="hidden">
      <div class="table">
        ${ele._supportsManualRolls ? html`
          ${!ele._rollCandidates ? html`
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
          ${ele._rollCandidates.map((rollCandidate) => html`
            <div class="tr rollCandidate">
              <div class="td">
                ${rollCandidate.url ? html`
                  <a href="${rollCandidate.url}" target="_blank">${rollCandidate.display}</a>
                ` : html`
                  ${rollCandidate.display}
                `}
              </div>
              <div class="td">${!!rollCandidate.description ? sk.truncate(rollCandidate.description, 100) : html``}</div>
              <div class="td"><human-date-sk date="${rollCandidate.timestamp}"></human-date-sk></div>
              <div class="td">${rollCandidate.roll ? rollCandidate.roll.requester : html``}</div>
              <div class="td">${rollCandidate.roll ? html`<human-date-sk date="${rollCandidate.roll.timestamp}"></human-date-sk>` : html``}</div>
              <div class="td">
                ${rollCandidate.roll && rollCandidate.roll.url ? html`
                  <a href="${rollCandidate.roll.url}", target="_blank">${rollCandidate.roll.url}</a>
                ` : html``}
                ${!!rollCandidate.roll && !rollCandidate.roll.url && rollCandidate.roll.status ? rollCandidate.roll.status : html``}
                ${!rollCandidate.roll ? html`
                  <button
                      on-tap="_requestManualRoll"
                      data-rev="${rollCandidate.id}"
                      class="requestRoll"
                      disabled=${!ele._editRights}
                      title="${ele._computeTitle(ele._editRights, 'Request a roll to this revision.')}">
                    Request Roll
                  </button>
                ` : html``}
                ${!!rollCandidate.roll && !!rollCandidate.roll.result ? html`
                  <span class="${ele._reqResultClass(rollCandidate.roll)}">${rollCandidate.roll.result}</span>
                ` : html``}
              </div>
            </div>
          </template>
          `)}
          <div class="tr rollCandidate">
            <div class="td">
              <paper-input label="type revision/ref" value="{{_manualRollRevInput}}"></paper-input>
            </div>
            <div class="td"><!-- no description        --></div>
            <div class="td"><!-- no revision timestamp --></div>
            <div class="td"><!-- no requester          --></div>
            <div class="td"><!-- no request timestamp  --></div>
            <div class="td">
              <button
                  on-tap="_requestManualRoll"
                  data-rev="(input)"
                  class="requestRoll"
                  disabled=${!ele._editRights}
                  title="${ele._computeTitle(ele._editRights, 'Request a roll to this revision.')}">
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
    </div>
    <!-- Warning for future travelers: paper-dialog doesn't like to be inside of
         divs. It's best left just inside the top-level <template> -->
    <paper-dialog id="mode_change_dialog" modal on-iron-overlay-closed="_changeMode">
      <h2>Enter a message:</h2>
      <paper-input type="text" id="mode_change_msg"></paper-input>
      <paper-button dialog-dismiss>Cancel</paper-button>
      <paper-button dialog-confirm>Submit</paper-button>
    </paper-dialog>
    <paper-dialog id="strategy_change_dialog" modal on-iron-overlay-closed="_changeStrategy">
      <h2>Enter a message:</h2>
      <paper-input type="text" id="strategy_change_msg"></paper-input>
      <paper-button dialog-dismiss>Cancel</paper-button>
      <paper-button dialog-confirm>Submit</paper-button>
    </paper-dialog>
    <url-param-sk name="tab" value="{{selectedTab}}" default="status"></url-param-sk>
  </template>
`;

class ARBStatus {

}

class ARBStatusSk extends HTMLElement {
  private _editRights: boolean
  private _modeChangePending: boolean
  private _status: ARBStatus

  constructor() {
    super();
    this._editRights = false;
    this._modeChangePending = false;
    this._status = {};
  }

  ready() {
    Login.then((loginstatus: any) => {
      this._editRights = loginstatus.IsAGoogler;
    });
    this._reload();
  }

  _modeButtonPressed(e) {
    if (!this._editRights) {
      errorMessage("You must be logged in with an @google.com account to set the ARB mode.");
      return
    }
    var mode = e.srcElement.value;
    if (mode == this.mode) {
      return;
    }
    this._selectedMode = mode;
    this.$.mode_change_dialog.open();
  }

  _changeMode(e) {
    if (e.detail.canceled || !e.detail.confirmed) {
      this._selectedMode = "";
      return;
    }
    errorMessage("Mode change in progress. This may take some time.");
    this._modeChangePending = true;
    fetch(window.location.pathname + "/json/mode", {
      method: "POST",
      body: JSON.stringify({
        "message": this.$.mode_change_msg.value,
        "mode": this._selectedMode,
      }),
      headers: {
        'Content-Type': 'application/json',
      },
    }).then(jsonOrThrow).then((json) => {
      this._update(json);
      this._modeChangePending = false;
      this.$.mode_change_msg.value;
      errorMessage("Success!");
    }, (err) => {
      this._modeChangePending = false;
      errorMessage("Failed to change the mode: " + err.response);
    });
  }

  _changeStrategy(e) {
    if (e.detail.canceled || !e.detail.confirmed) {
      return;
    }
    errorMessage("Strategy change in progress. This may take some time.");
    fetch(window.location.pathname + "/json/strategy", {
      method: "POST",
      body: JSON.stringify({
        "message": this.$.strategy_change_msg.value,
        "strategy": this._selectedStrategy,
      }),
      headers: {
        'Content-Type': 'application/json',
      },
    }).then(jsonOrThrow).then((json) => {
      this._update(json);
      this.$.strategy_change_msg.value = "";
      errorMessage("Success!");
    }, (err) => {
      errorMessage("Failed to change the strategy: " + err.response);
    });
  }

  _computeModeChangeDisabled(editRights, modeChangePending) {
    return !editRights || modeChangePending;
  }

  _computeParentWaterfall(config) {
    if (!config) {
      return "";
    }
    return config.parentWaterfall;
  }

  // _computeRollWindowStart returns a string indicating when the configured
  // roll window will start. If errors are encountered, in particular those
  // relating to parsing the roll window, the returned string will contain
  // the error.
  _computeRollWindowStart(config) {
    if (!config || !config.timeWindow) {
      return "";
    }
    // TODO(borenet): This duplicates code in the go/time_window package.

    // parseDayTime returns a 2-element array containing the hour and
    // minutes as ints. Throws an error (string) if the given string cannot
    // be parsed as hours and minutes.
    const parseDayTime = function(s) {
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

  _computeShowError(editRights, error) {
    return editRights && error;
  }

  _computeSupportsManualRolls(config) {
    if (!config) {
      return false;
    }
    return config.supportsManualRolls;
  }

  _computeTitle(editRights, title) {
    if (!editRights) {
      return this._pleaseLoginMsg;
    }
    return title;
  }

  _issueURL(issue) {
    if (issue) {
      return this.issueUrlBase + issue.issue;
    }
  }

  _exists(obj) {
    return !!obj;
  }

  _getModeButtonLabel(currentMode, mode) {
    // TODO(borenet): This is a hack; it doesn't respect this.validModes.
    return {
      "running": {
        "stopped": "stop",
        "dry run": "switch to dry run",
      }
      "stopped": {
        "running": "resume",
        "dry run": "switch to dry run",
      }
      "dry run": {
        "running": "switch to normal mode",
        "stopped": "stop",
      }
    }[currentMode][mode];
  }

  _isCQ(tryjob) {
    return tryjob.category === "cq";
  }

  _isThrottled(status) {
    return status.indexOf("throttle") >= 0;
  }

  _isWaitingForRollWindow(status) {
    return status.indexOf("waiting for roll window") >= 0;
  }

  _reloadChanged() {
    this._resetTimeout();
  }

  _resetTimeout() {
    if (this._timeout) {
      window.clearTimeout(this._timeout);
    }
    if (this.reload > 0) {
      this._timeout = window.setTimeout(function () {
        this._reload();
      }.bind(this), this.reload * 1000);
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

  _reqResultClass(req) {
    if (!req) {
      return "";
    }
    return {
      "SUCCESS": "fg-success",
      "FAILURE": "fg-failure",
    }[req.result];
  }

  _requestManualRoll(e) {
    var url = window.location.pathname + "/json/manual";
    var rev = sk.findParent(e.target, "BUTTON").dataset.rev;
    if (rev == "(input)") {
      rev = this._manualRollRevInput;
    }
    var req = {
        "revision": rev,
    };
    fetch(url, {
      method: "POST",
      body: JSON.stringify(req),
      headers: {
        'Content-Type': 'application/json',
      },
    }).then(jsonOrThrow).then((json) => {
      this.push("rollCandidates", {
        id: req.revision,
        display: req.revision,
        roll: req,
      });
      this.set("_manualRollRevInput", "");
      errorMessage("Successfully requested manual roll.");
    }, (err) => {
      errorMessage("Failed to request manual roll: " + err.response);
    });
  }

  _rollClass(roll) {
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

  _rollResult(roll) {
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

  _selectedStrategyChanged(e) {
    if (!this._selectedStrategy || this._selectedStrategy == this.strategy) {
      return;
    }
    if (!this._editRights) {
      errorMessage("You must be logged in with an @google.com account to set the ARB strategy.");
      return
    }
    this.$.strategy_change_dialog.open();
  }

  tabChanged() {
    if (this.selectedTab == "status") {
      this.$.statusDisplay.classList.remove("hidden");
      this.$.rollCandidates.classList.add("hidden");
    } else if (this.selectedTab == "manual") {
      this.$.statusDisplay.classList.add("hidden");
      this.$.rollCandidates.classList.remove("hidden");
    }
  }

  _trybotClass(trybot) {
    if (trybot.status == "STARTED") {
      return "fg-unknown";
    } else if (trybot.status == "COMPLETED") {
      return {
        "CANCELED": "fg-failure",
        "FAILURE": "fg-failure",
        "SUCCESS": "fg-success",
      }[trybot.result] || "";
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

  _update(json) {
    var modeButtons = [];
    for (var i = 0; i < this.validModes.length; i++) {
      var m = this.validModes[i];
      if (m != this.mode) {
        modeButtons.push({
          "label": this._getModeButtonLabel(this.mode, m),
          "value": m,
        });
      }
    }
    this._setModeButtons(modeButtons);

    var rollCandidates = null;
    if (json.notRolledRevs) {
      rollCandidates = [];
      var manualByRev = {};
      if (json.manualRequests) {
        for (var i = 0; i < json.manualRequests.length; i++) {
          var req = json.manualRequests[i];
          manualByRev[req.revision] = req;
        }
      }
      for (var i = 0; i < json.notRolledRevs.length; i++) {
        var rev = json.notRolledRevs[i];
        var req = manualByRev[rev.id];
        delete manualByRev[rev.id];
        if (!req && this.currentRoll && this.currentRoll.rollingTo == rev.id) {
          req = {
            "requester": "autoroller",
            "status":    "STARTED",
            "timestamp": this.currentRoll.created,
            "url":       this._issueURL(this.currentRoll),
          };
        }
        rev.roll = req;
        rollCandidates.push(rev);
      }
    }
    for (var key in manualByRev) {
      const req = manualByRev[key];
      rollCandidates.push({
        id: req.revision,
        display: req.revision,
        roll: req,
      });
    };
    this.set("rollCandidates", rollCandidates);

    this._lastLoaded = new Date().toLocaleTimeString();
    this._resetTimeout();
    this._selectedStrategy = json.strategy.strategy
    console.log("Reloaded status.");
  }

  _render() {
    render(template(this._status), this, {eventContext: this});
  }
}

define('arb-status-sk', ArbStatusSk);