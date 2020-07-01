<!--
  The common.js file must be included before this file.

  This in an HTML Import-able file that contains the definition
  of the following elements:

    <arb-status-sk>

  To use this file import it:

    <link href="/res/imp/arb-status-sk.html" rel="import" />

  Usage:

    <arb-status-sk></arb-status-sk>

  Properties:
    reload - How often (in seconds) to reload data.

  Methods:
    None.

  Events:
    None
-->
<link rel="import" href="/res/common/imp/human-date-sk.html">
<link rel="import" href="/res/common/imp/styles-sk.html">
<link rel="import" href="/res/common/imp/url-params-sk.html">
<link rel="import" href="/res/imp/bower_components/iron-flex-layout/iron-flex-layout-classes.html">
<link rel="import" href="/res/imp/bower_components/paper-button/paper-button.html">
<link rel="import" href="/res/imp/bower_components/paper-dialog/paper-dialog.html">
<link rel="import" href="/res/imp/bower_components/paper-dropdown-menu/paper-dropdown-menu.html">
<link rel="import" href="/res/imp/bower_components/paper-input/paper-input.html">
<link rel="import" href="/res/imp/bower_components/paper-item/paper-item.html">
<link rel="import" href="/res/imp/bower_components/paper-listbox/paper-listbox.html">
<link rel="import" href="/res/imp/bower_components/paper-spinner/paper-spinner.html">
<link rel="import" href="/res/imp/bower_components/paper-tabs/paper-tabs.html">
<link rel="stylesheet" href="/res/common/css/md.css">
<dom-module id="arb-status-sk">
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
  <template>
    <div id="tabsContainer">
    <paper-tabs id="tabs" attr-for-selected="id" selected="{{selectedTab}}" no-bar>
      <paper-tab id="status">Roller Status</paper-tab>
      <paper-tab id="manual">Trigger Manual Rolls</paper-tab>
    </paper-tabs>
    </div>
    <template is="dom-if" if="[[!_editRights]]">
      <div id="pleaseLoginMsg">[[_pleaseLoginMsg]]</div>
    </template>
    <div id="statusDisplay">
      <div class="horizontal layout center" id="loadstatus">
        <paper-input type="number" value="{{reload}}" label="Reload (s)" prevent-invalid-input></paper-input>
        <div class="flex"></div>
        <div>Last loaded at <span>[[_lastLoaded]]</span></div>
      </div>
      <div class="table">
        <template is="dom-if" if="[[_exists(_parentWaterfall)]]">
          <div class="tr">
            <div class="td nowrap">Parent Repo Build Status</div>
            <div class="td nowrap unknown">
              <span class="big"><a href$="[[_parentWaterfall]]" target="_blank">[[_parentWaterfall]]</a></span>
            </div>
          </div>
        </template>
        <div class="tr">
          <div class="td nowrap">Current Mode:</div>
          <div class="td nowrap unknown">
            <span class="big">[[mode]]</span>
          </div>
        </div>
        <div class="tr">
          <div class="td nowrap">Set By:</div>
          <div class="td nowrap unknown">
            <!-- No line break below, or we get a space before the colon, eg.
                 user@google.com : Mode change message -->
            <span>[[modeChangeBy]]</span><template is="dom-if" if="[[modeChangeMsg]]"><span>: [[modeChangeMsg]]</span></template>
          </div>
        </div>
        <div class="tr">
          <div class="td nowrap">Change Mode:</div>
          <div class="td nowrap">
            <template is="dom-repeat" items="[[modeButtons]]">
              <button class$="[[_buttonClass(item)]]" on-tap="_modeButtonPressed" disabled$="[[_computeModeChangeDisabled(_editRights,_modeChangePending)]]" title$="[[_computeTitle(_editRights, 'Change the mode.')]]" value="[[item.value]]">[[item.label]]</button>
            </template>
            <paper-spinner active$="[[_modeChangePending]]"></paper-spinner>
          </div>
        </div>
        <div class="tr">
          <div class="td nowrap">Status:</div>
          <div class="td nowrap">
            <span class$="[[_statusClass(status)]]"><span class="big">[[status]]</span></span>
            <template is="dom-if" if="[[_isThrottled(status)]]">
              <span>until <human-date-sk date="[[throttledUntil]]" seconds></human-date-sk></span>
              <button on-tap="_unthrottle" disabled$="[[!_editRights]]" title$="[[_computeTitle(_editRights, 'Unthrottle the roller.')]]">Force Unthrottle</button>
            </template>
            <template is="dom-if" if="[[_isWaitingForRollWindow(status)]]">
              <span>until <human-date-sk date="[[_rollWindowStart]]"></human-date-sk></span>
            </template>
          </div>
        </div>
        <template is="dom-if" if="[[_computeShowError(_editRights,error)]]">
          <div class="tr">
            <div class="td nowrap">Error:</div>
            <div class="td"><pre>[[error]]</pre></div>
          </div>
        </template>
        <div class="tr">
          <div class="td nowrap">Current Roll:</div>
          <div class="td">
            <div>
              <template is="dom-if" if="[[_exists(currentRoll)]]">
                <a href="[[_issueURL(currentRoll)]]" class="big" target="_blank">[[currentRoll.subject]]</a>
              </template>
              <template is="dom-if" if="[[!_exists(currentRoll)]]">
                <span>(none)</span>
              </template>
            </div>
            <div>
              <template is="dom-repeat" items="[[currentRoll.tryResults]]">
                <div class="trybot">
                  <template is="dom-if" if="[[_exists(item.url)]]">
                    <a href="[[item.url]]" class$="[[_trybotClass(item)]]" target="_blank">[[item.builder]]</a>
                  </template>
                  <template is="dom-if" if="[[!_exists(item.url)]]">
                    <span class="nowrap" class$="[[_trybotClass(item)]]">[[item.builder]]</span>
                  </template>
                  <template is="dom-if" if="[[!_isCQ(item)]]">
                    <span class="nowrap small">([[item.category]])</span>
                  </template>
                </div>
              </template>
            </div>
          </div>
        </div>
        <template is="dom-if" if="[[_exists(lastRoll)]]">
          <div class="tr">
            <div class="td nowrap">Previous roll result:</div>
            <div class="td">
              <span class$="[[_rollClass(lastRoll)]]">[[_rollResult(lastRoll)]]</span>
              <a href="[[_issueURL(lastRoll)]]" target="_blank" class="small">(detail)</a>
            </div>
          </div>
        </template>
        <div class="tr">
          <div class="td nowrap">History:</div>
          <div class="td">
            <div class="table">
              <div class="tr">
                <div class="th">Roll</div>
                <div class="th">Last Modified</div>
                <div class="th">Result</div>
              </div>
              <template is="dom-repeat" items="[[recent]]">
                <div class="tr">
                  <div class="td"><a href="[[_issueURL(item)]]" target="_blank">[[item.subject]]</a></div>
                  <div class="td"><human-date-sk date="[[item.modified]]" diff></human-date-sk> ago</div>
                  <div class="td"><span class$="[[_rollClass(item)]]">[[_rollResult(item)]]</span></div>
                </div>
              </template>
            </div>
          </div>
        </div>
        <div class="tr">
          <div class="td nowrap">Full History:</div>
          <div class="td">
            <a href$="[[fullHistoryUrl]]" target="_blank">
              [[fullHistoryUrl]]
            </a>
          </div>
        </div>
        <div class="tr">
          <div class="td nowrap">Strategy for choosing next roll revision:</div>
          <div class="td nowrap">
            <paper-dropdown-menu id="strategyDropDown" disabled$="[[!_editRights]]" title$="[[_computeTitle(_editRights, 'Change the strategy for choosing the next revision to roll.')]]">
              <paper-listbox id="strategyMenu" class="dropdown-content" selected="{{_selectedStrategy}}" id="strategy_listbox" attr-for-selected="value">
                <template is="dom-repeat" items="[[validStrategies]]">
                  <paper-item value="[[item]]">[[item]]</paper-item>
                </template>
              </paper-listbox>
            </paper-dropdown-menu>
          </div>
        </div>
        <div class="tr">
          <div class="td nowrap">Set By:</div>
          <div class="td nowrap unknown">
            <!-- No line break below, or we get a space before the colon, eg.
                 user@google.com : Strategy change message -->
            <span>[[strategyChangeBy]]</span><template is="dom-if" if="[[strategyChangeMsg]]"><span>: [[strategyChangeMsg]]</span></template>
          </div>
        </div>
      </div>
    </div>
    <div id="rollCandidates" class="hidden">
      <div class="table">
        <template is="dom-if" if="[[!_supportsManualRolls]]">
          This roller does not support manual rolls. If you want this feature,
          update the config file for the roller to enable it. Note that some
          rollers cannot support manual rolls for technical reasons.
        </template>
        <template is="dom-if" if="[[_supportsManualRolls]]">
          <template is="dom-if" if="[[!rollCandidates]]">
            The roller is up to date; there are no revisions which could be manually rolled.
          </template>
          <div class="tr">
            <div class="th">Revision</div>
            <div class="th">Description</div>
            <div class="th">Timestamp</div>
            <div class="th">Requester</div>
            <div class="th">Requested at</div>
            <div class="th">Roll</div>
          </div>
          <template is="dom-repeat" items="[[rollCandidates]]">
            <div class="tr rollCandidate">
              <div class="td">
                <template is="dom-if" if="[[item.url]]">
                  <a href="[[item.url]]" target="_blank">[[item.display]]</a>
                </template>
                <template is="dom-if" if="[[!item.url]]">
                  [[item.display]]
                </template>
              </div>
              <div class="td">[[_revDescription(item.description)]]</div>
              <div class="td"><human-date-sk date="[[item.timestamp]]"></human-date-sk></div>
              <div class="td">[[item.roll.requester]]</div>
              <div class="td"><human-date-sk date="[[item.roll.timestamp]]"></human-date-sk></div>
              <div class="td">
                <template is="dom-if" if="[[_reqHasURL(item.roll)]]">
                  <a href="[[item.roll.url]]", target="_blank">[[item.roll.url]]</a>
                </template>
                <template is="dom-if" if="[[_reqHasStatus(item.roll)]]">
                  [[item.roll.status]]
                </template>
                <template is="dom-if" if="[[_reqHasButton(item.roll)]]">
                  <button on-tap="_requestManualRoll" data-rev$="[[item.id]]" class="requestRoll" disabled$="[[!_editRights]]" title$="[[_computeTitle(_editRights, 'Request a roll to this revision.')]]">Request Roll</button>
                </template>
                <template is="dom-if" if="[[_reqHasResult(item.roll)]]">
                  <span class$="[[_reqResultClass(item.roll)]]">[[item.roll.result]]</span>
                </template>
              </div>
            </div>
          </template>
          <div class="tr rollCandidate">
            <div class="td">
              <paper-input label="type revision/ref" value="{{_manualRollRevInput}}"></paper-input>
            </div>
            <div class="td"><!-- no description        --></div>
            <div class="td"><!-- no revision timestamp --></div>
            <div class="td"><!-- no requester          --></div>
            <div class="td"><!-- no request timestamp  --></div>
            <div class="td">
              <button on-tap="_requestManualRoll" data-rev="(input)" class="requestRoll" disabled$="[[!_editRights]]" title$="[[_computeTitle(_editRights, 'Request a roll to this revision.')]]">Request Roll</button>
            </div>
          </div>
        </template>
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
  <script>
    Polymer({
      is: 'arb-status-sk',
      properties: {
        config: {
          type: Object,
          value: null,
          readOnly: true,
        },
        mode: {
          type: String,
          value: "(not yet loaded)",
          readOnly: true,
        },
        modeChangeBy: {
          type: String,
          value: "",
          readOnly: true,
        },
        modeChangeMsg: {
          type: String,
          value: "",
          readOnly: true,
        },
        status: {
          type: String,
          value: "(not yet loaded)",
          readOnly: true,
        },
        currentRoll: {
          type: Object,
          value: null,
          readOnly: true,
        },
        error: {
          type: String,
          value: null,
          readOnly: true,
        },
        fullHistoryUrl: {
          type: String,
          value: "",
          readOnly: true,
        },
        issueUrlBase: {
          type: String,
          value: "",
          readOnly: true,
        },
        lastRoll: {
          type: Object,
          value: null,
          readOnly: true,
        },
        recent: {
          type: Array,
          value: function() { return []; },
          readOnly: true,
        },
        reload: {
          type: Number,
          observer: "_reloadChanged",
          value: 60,
        },
        initialSelectedMode: {
          type: Number,
          value: 0,
          readOnly: true,
        },
        modeButtons: {
          type: Array,
          value: function() { return []; },
          readOnly: true,
        },
        rollCandidates: {
          type: Array,
          value: null,
        },
        selectedTab: {
          type: String,
          observer: "tabChanged",
        },
        strategy: {
          type: String,
          value: "(not yet loaded)",
          readOnly: true,
        },
        strategyChangeBy: {
          type: String,
          value: "",
          readOnly: true,
        },
        strategyChangeMsg: {
          type: String,
          value: "",
          readOnly: true,
        },
        throttledUntil: {
          type: Number,
          value: 0,
          readOnly: true,
        },
        validModes: {
          type: Array,
          value: function() { return []; },
          readOnly: true,
        },
        validStrategies: {
          type: Array,
          value: function() { return []; },
          readOnly: true,
        },
        _editRights: {
          type: Boolean,
          value: false,
        },
        _lastLoaded: {
          type: String,
          value: "not yet loaded",
        },
        _manualRollRevInput: {
          type: String,
        },
        _modeChangePending: {
          type: Boolean,
          value: false,
        },
        _parentWaterfall: {
          type: String,
          computed: "_computeParentWaterfall(config)",
        },
        _pleaseLoginMsg: {
          type: String,
          value: "Please log in with an @google.com account to make changes.",
        },
        _rollWindowStart: {
          type: String,
          computed: "_computeRollWindowStart(config)",
        },
        _selectedMode: {
          type: String,
          value: "",
        },
        _selectedStrategy: {
          type: String,
          value: "",
          observer: "_selectedStrategyChanged",
        },
        _supportsManualRolls: {
          type: Boolean,
          computed: "_computeSupportsManualRolls(config)",
        },
        _timeout: {
          type: Object,
          value: null,
        },
      },

      ready: function() {
        sk.Login.then(function(status) {
          this._editRights = status.IsAGoogler;
        }.bind(this));
        this._reload();
      },

      _buttonClass: function(mode) {
        return mode.class;
      },

      _modeButtonPressed: function(e) {
        if (!this._editRights) {
          sk.errorMessage("You must be logged in with an @google.com account to set the ARB mode.");
          return
        }
        var mode = e.srcElement.value;
        if (mode == this.mode) {
          return;
        }
        this._selectedMode = mode;
        this.$.mode_change_dialog.open();
      },

      _changeMode: function(e) {
        if (e.detail.canceled || !e.detail.confirmed) {
          this._selectedMode = "";
          return;
        }
        var url = window.location.pathname + "/json/mode";
        var body = JSON.stringify({
            "message": this.$.mode_change_msg.value,
            "mode": this._selectedMode,
        });
        sk.errorMessage("Mode change in progress. This may take some time.");
        this._modeChangePending = true;
        sk.post(url, body).then(JSON.parse).then(function (json) {
          this._update(json);
          this._modeChangePending = false;
          this.$.mode_change_msg.value;
          sk.errorMessage("Success!");
        }.bind(this), function(err) {
          this._modeChangePending = false;
          sk.errorMessage("Failed to change the mode: " + err.response);
        });
      },

      _changeStrategy: function(e) {
        if (e.detail.canceled || !e.detail.confirmed) {
          return;
        }
        var url = window.location.pathname + "/json/strategy";
        var body = JSON.stringify({
            "message": this.$.strategy_change_msg.value,
            "strategy": this._selectedStrategy,
        });
        sk.errorMessage("Strategy change in progress. This may take some time.");
        sk.post(url, body).then(JSON.parse).then(function (json) {
          this._update(json);
          this.$.strategy_change_msg.value = "";
          sk.errorMessage("Success!");
        }.bind(this), function(err) {
          sk.errorMessage("Failed to change the strategy: " + err.response);
        });
      },

      _computeModeChangeDisabled: function(editRights, modeChangePending) {
        return !editRights || modeChangePending;
      },

      _computeParentWaterfall: function(config) {
        if (!config) {
          return "";
        }
        return config.parentWaterfall;
      },

      // _computeRollWindowStart returns a string indicating when the configured
      // roll window will start. If errors are encountered, in particular those
      // relating to parsing the roll window, the returned string will contain
      // the error.
      _computeRollWindowStart: function(config) {
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
      },

      _computeShowError: function(editRights, error) {
        return editRights && error;
      },

      _computeSupportsManualRolls: function(config) {
        if (!config) {
          return false;
        }
        return config.supportsManualRolls;
      },

      _computeTitle: function(editRights, title) {
        if (!editRights) {
          return this._pleaseLoginMsg;
        }
        return title;
      },

      _issueURL: function(issue) {
        if (issue) {
          return this.issueUrlBase + issue.issue;
        }
      },

      _exists: function(obj) {
        return !!obj;
      },

      _getModeButtonLabel: function(currentMode, mode) {
        // TODO(borenet): This is a hack; it doesn't respect this.validModes.
        return {
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
        }[currentMode][mode];
      },

      _isCQ: function(tryjob) {
        return tryjob.category === "cq";
      },

      _isThrottled: function(status) {
        return status.indexOf("throttle") >= 0;
      },

      _isWaitingForRollWindow: function(status) {
        return status.indexOf("waiting for roll window") >= 0;
      },

      _reloadChanged: function() {
        this._resetTimeout();
      },

      _resetTimeout: function() {
        if (this._timeout) {
          window.clearTimeout(this._timeout);
        }
        if (this.reload > 0) {
          this._timeout = window.setTimeout(function () {
            this._reload();
          }.bind(this), this.reload * 1000);
        }
      },

      _reload: function() {
        var url = window.location.pathname + "/json/status";
        console.log("Loading status from " + url);

        sk.get(url).then(JSON.parse).then(function(json) {
          this._update(json);
        }.bind(this)).catch(function(err) {
          sk.errorMessage("Failed to load status: " + err.response);
          this._resetTimeout();
        }.bind(this));
      },

      _reqHasURL: function(req) {
        return !!req && !!req.url;
      },

      _reqHasStatus: function(req) {
        return !!req && !req.url && req.status;
      },

      _reqHasButton: function(req) {
        return !req;
      },

      _reqHasResult: function(req) {
        return !!req && !!req.result;
      },

      _reqResultClass: function(req) {
        if (!req) {
          return "";
        }
        return {
          "SUCCESS": "fg-success",
          "FAILURE": "fg-failure",
        }[req.result];
      },

      _reqResultText: function(req) {
        if (!req) {
          return "";
        }
        return req.result;
      },

      _requestManualRoll: function(e) {
        var url = window.location.pathname + "/json/manual";
        var rev = sk.findParent(e.target, "BUTTON").dataset.rev;
        if (rev == "(input)") {
          rev = this._manualRollRevInput;
        }
        var req = {
            "revision": rev,
        };
        var body = JSON.stringify(req);
        sk.post(url, body).then(JSON.parse).then(function(json) {
          this.push("rollCandidates", {
            id: req.revision,
            display: req.revision,
            roll: req,
          });
          this.set("_manualRollRevInput", "");
          sk.errorMessage("Successfully requested manual roll.");
        }.bind(this), function(err) {
          sk.errorMessage("Failed to request manual roll: " + err.response);
        });
      },

      _revDescription: function(desc) {
        if (!desc) {
          return "";
        }
        return sk.truncate(desc, 100);
      },

      _rollClass: function(roll) {
        if (!roll) {
          return "unknown";
        }
        return {
          "succeeded": "fg-success",
          "failed": "fg-failure",
          "in progress": "fg-unknown",
          "dry run succeeded": "fg-success",
          "dry run failed": "fg-failure",
          "dry run in progress": "fg-unknown",
        }[roll.result] || "fg-unknown";
      },

      _rollResult: function(roll) {
        if (!roll) {
          return "unknown";
        }
        return roll.result;
      },

      _statusClass: function(status) {
        return {
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
        }[status] || "";
      },

      _selectedStrategyChanged: function(e) {
        if (!this._selectedStrategy || this._selectedStrategy == this.strategy) {
          return;
        }
        if (!this._editRights) {
          sk.errorMessage("You must be logged in with an @google.com account to set the ARB strategy.");
          return
        }
        this.$.strategy_change_dialog.open();
      },

      tabChanged: function() {
        if (this.selectedTab == "status") {
          this.$.statusDisplay.classList.remove("hidden");
          this.$.rollCandidates.classList.add("hidden");
        } else if (this.selectedTab == "manual") {
          this.$.statusDisplay.classList.add("hidden");
          this.$.rollCandidates.classList.remove("hidden");
        }
      },

      _trybotClass: function(trybot) {
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
      },

      _unthrottle: function() {
        var url = window.location.pathname + "/json/unthrottle";
        sk.post(url).then(function(json) {
          sk.errorMessage("Successfully unthrottled the roller. May take a minute or so to start up.")
        }.bind(this)).catch(function(err) {
          sk.errorMessage("Failed to unthrottle: " + err.response);
        }.bind(this));
      },

      _update: function(json) {
        this._setConfig(json.config);
        this._setCurrentRoll(json.currentRoll);
        this._setError(json.error);
        this._setFullHistoryUrl(json.fullHistoryUrl);
        this._setIssueUrlBase(json.issueUrlBase);
        this._setLastRoll(json.lastRoll);
        this._setMode(json.mode.mode);
        this._setModeChangeBy(json.mode.user);
        this._setModeChangeMsg(json.mode.message);
        this._setRecent(json.recent);
        this._setInitialSelectedMode(json.validModes.indexOf(json.mode).toString());
        this._setStatus(json.status);
        this._setStrategy(json.strategy.strategy);
        this._setStrategyChangeBy(json.strategy.user);
        this._setStrategyChangeMsg(json.strategy.message);
        this._setThrottledUntil(json.throttledUntil);
        this._setValidModes(json.validModes);
        this._setValidStrategies(json.validStrategies);
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
      },
    });
  </script>
</dom-module>
