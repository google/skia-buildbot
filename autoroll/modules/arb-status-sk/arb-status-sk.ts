/**
 * @module autoroll/modules/arb-status-sk
 * @description <h2><code>arb-status-sk</code></h2>
 *
 * <p>
 * This element displays the status of a single Autoroller.
 * </p>
 */

import { html } from 'lit/html.js';

import { $$ } from '../../../infra-sk/modules/dom';
import { localeTime } from '../../../infra-sk/modules/human';

import { define } from '../../../elements-sk/modules/define';
import '../../../elements-sk/modules/tabs-panel-sk';
import '../../../elements-sk/modules/tabs-sk';
import '../../../elements-sk/modules/icons/launch-icon-sk';

import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { LoggedIn } from '../../../infra-sk/modules/alogin-sk/alogin-sk';
import { truncate } from '../../../infra-sk/modules/string';
import '../../../infra-sk/modules/human-date-sk';

import {
  AddCleanupRequestResponse,
  AutoRollCL,
  AutoRollCL_Result,
  AutoRollConfig,
  AutoRollService,
  AutoRollStatus,
  CreateManualRollResponse,
  GetAutoRollService,
  GetStatusResponse,
  ManualRoll,
  ManualRoll_Result,
  ManualRoll_Status,
  Mode,
  Revision,
  SetModeResponse,
  SetStrategyResponse,
  Strategy,
  TryJob,
  TryJob_Result,
} from '../rpc';
import { LastCheckInSpan } from '../utils';
import { Status } from '../../../infra-sk/modules/json';

interface RecentRoll {
  class: string;
  subject: string;
  rollingTo: string;
  timestamp: string;
  result: string;
  url: string;
}

export class ARBStatusSk extends ElementSk {
  private static template = (ele: ARBStatusSk) =>
    !ele.status
      ? html``
      : html`
  ${
    !ele.editRights
      ? html` <div id="pleaseLoginMsg" class="big">${ele.pleaseLoginMsg}</div> `
      : html``
  }
  <tabs-sk style="width: 100%">
    <button value="status">Roller Status</button>
    <button value="manual">Trigger Manual Rolls</button>
    <button value="config">View Roller Config</button>
    <div style="flex-grow: 1"></div>
    <div style="display: flex; align-items: center">
      <a
        href="https://pantheon.corp.google.com/logs/query;query=labels.k8s-pod%2Fapp%3D%22autoroll-be-${
          ele.roller
        }%22?project=${ele.project}"
        target="_blank"
        rel="noopener noreferrer"
        >Logs <launch-icon-sk></launch-icon-sk>
      </a>
    </div>
  </tabs-sk>
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
        <tr>
          <td>Status</td>
          <td><span class="big ${ele.statusClass(
            ele.status
          )}">${ele.statusMessage(ele.status)}</span></td>
        </tr>
        <tr>
          <td>Last successful roll</td>
          <td>
            <human-date-sk .date="${ele.status.miniStatus!
              .lastSuccessfulRollTimestamp!}" .diff="${true}"></human-date-sk>
          </td>
        </tr>
        ${
          ele.status.config?.parentWaterfall
            ? html`
                <tr>
                  <td class="nowrap">Parent Repo Build Status</td>
                  <td class="nowrap unknown">
                    <span>
                      <a
                        href="${ele.status.config.parentWaterfall}"
                        target="_blank">
                        ${ele.status.config.parentWaterfall}
                      </a>
                    </span>
                  </td>
                </tr>
              `
            : html``
        }
        <tr>
          <td class="nowrap">Mode:</td>
          <td class="nowrap unknown">
            <span class="big ${ele.modeClass(
              ele.status.mode!.mode
            )}">${ele.status.mode?.mode.toLowerCase().replace('_', ' ')}</span>
            <br/>
            Set by ${ele.status.mode?.user}
            ${
              ele.status.mode
                ? html`(<human-date-sk
                      .date="${ele.status.mode?.time}"
                      .diff="${true}"></human-date-sk
                    >)`
                : html``
            }
                  ${
                    ele.status.mode?.message
                      ? html`: ${ele.status.mode.message}`
                      : html``
                  }
            <br/>
            <a href="/r/${
              ele.roller
            }/mode-history" class="small"><button>History</button></a>
            <button
                @click="${() => {
                  ele.modeChangeDialog();
                }}"
                ?disabled="${!ele.editRights || ele.modeChangePending}"
                >Update</button>
          </td>
        </tr>
        <tr>
          <td class="nowrap">State:</td>
          <td class="nowrap">
            <span class="${ele.stateClass(ele.status.status)}">
              <span class="big">${ele.status.status}</span>
            </span>
            ${
              ele.status.status.indexOf('throttle') >= 0
                ? html`
                    <span
                      >until
                      ${localeTime(new Date(ele.status.throttledUntil!))}</span
                    >
                    <button
                      @click="${ele.unthrottle}"
                      ?disabled="${!ele.editRights}"
                      title="${ele.editRights
                        ? 'Unthrottle the roller.'
                        : ele.pleaseLoginMsg}">
                      Force Unthrottle
                    </button>
                  `
                : html``
            }
            ${
              ele.status.status.indexOf('waiting for roll window') >= 0
                ? html` <span>until ${localeTime(ele.rollWindowStart)}</span> `
                : html``
            }
            ${LastCheckInSpan(ele.status?.miniStatus)}
          </td>
        </tr>
        ${
          ele.editRights && ele.status.error
            ? html`
                <tr>
                  <td class="nowrap">Error:</td>
                  <td><pre>${ele.status.error}</pre></td>
                </tr>
              `
            : html``
        }
        ${
          ele.status.config?.childBugLink
            ? html`
                <tr>
                  <td class="nowrap">
                    File a bug in ${ele.status.miniStatus?.childName}
                  </td>
                  <td>
                    <a
                      href="${ele.status.config.childBugLink}"
                      target="_blank"
                      class="small">
                      file bug
                    </a>
                  </td>
                </tr>
              `
            : html``
        }
        ${
          ele.status.config?.parentBugLink
            ? html`
                <tr>
                  <td class="nowrap">
                    File a bug in ${ele.status.miniStatus?.parentName}
                  </td>
                  <td>
                    <a
                      href="${ele.status.config.parentBugLink}"
                      target="_blank"
                      class="small">
                      file bug
                    </a>
                  </td>
                </tr>
              `
            : html``
        }
        <tr>
          <td class="nowrap">Current Roll:</td>
          <td>
            <div>
              ${
                ele.status.currentRoll
                  ? html`
                      <a
                        href="${ele.issueURL(
                          ele.status,
                          ele.status.currentRoll
                        )}"
                        class="big"
                        target="_blank">
                        ${ele.status.currentRoll.subject}
                      </a>
                    `
                  : html`<span>(none)</span>`
              }
            </div>
            <div>
              ${
                ele.status.currentRoll && ele.status.currentRoll.tryJobs
                  ? ele.status.currentRoll.tryJobs.map(
                      (tryResult) => html`
                        <div class="trybot">
                          ${tryResult.url
                            ? html`
                                <a
                                  href="${tryResult.url}"
                                  class="${ele.trybotClass(tryResult)}"
                                  target="_blank">
                                  ${tryResult.name}
                                </a>
                              `
                            : html`
                                <span
                                  class="nowrap"
                                  class="${ele.trybotClass(tryResult)}">
                                  ${tryResult.name}
                                </span>
                              `}
                          ${tryResult.category === 'cq'
                            ? html``
                            : html`
                                <span class="nowrap small"
                                  >(${tryResult.category})</span
                                >
                              `}
                        </div>
                      `
                    )
                  : html``
              }
            </div>
          </td>
        </tr>
        ${
          ele.status.lastRoll
            ? html`
                <tr>
                  <td class="nowrap">Previous roll result:</td>
                  <td>
                    <span class="${ele.rollClass(ele.status.lastRoll)}">
                      ${ele.rollResult(ele.status.lastRoll)}
                    </span>
                    <a
                      href="${ele.issueURL(ele.status, ele.status.lastRoll)}"
                      target="_blank"
                      class="small">
                      (detail)
                    </a>
                  </td>
                </tr>
              `
            : html``
        }
        <tr>
          <td class="nowrap">History:</td>
          <td>
            <table>
              <tr>
                <th>Roll</th>
                <th>Creation Time</th>
                <th>Result</th>
              </tr>
              ${ele.recentRolls.map(
                (roll: RecentRoll) => html`
                  <tr>
                    <td>
                      ${roll.url
                        ? html`
                            <a href="${roll.url}" target="_blank"
                              >${roll.subject}</a
                            >
                          `
                        : html` ${roll.subject} (no CL created) `}
                    </td>
                    <td>
                      <human-date-sk
                        .date="${roll.timestamp}"
                        .diff="${true}"></human-date-sk>
                    </td>
                    <td>
                      <span class="${roll.class}">${roll.result}</span>
                    </td>
                  </tr>
                `
              )}
            </table>
            <a href="/r/${
              ele.roller
            }/roll-history" class="small"><button>More</button></a>
          </td>
        </tr>
        <tr>
          <td class="nowrap">All CLs by owner:</td>
          <td>
            <a href="${ele.status.fullHistoryUrl}" target="_blank">
              ${ele.status.fullHistoryUrl}
            </a>
          </td>
        </tr>
        <tr>
          <td class="nowrap">Strategy for choosing next roll revision:</</td>
          <td class="nowrap unknown">
            <span class="big">${ele.status.strategy?.strategy
              .toLowerCase()
              .replace('_', ' ')}</span>
            <br/>
            Set by ${ele.status.strategy?.user}
            ${
              ele.status.mode
                ? html`(<human-date-sk
                      .date="${ele.status.strategy?.time}"
                      .diff="${true}"></human-date-sk
                    >)`
                : html``
            }
                  ${
                    ele.status.strategy?.message
                      ? html`: ${ele.status.strategy.message}`
                      : html``
                  }
            <br/>
            <a href="/r/${
              ele.roller
            }/strategy-history" class="small"><button>History</button></a>
            <button
                @click="${() => {
                  ele.strategyChangeDialog();
                }}"
                ?disabled="${!ele.editRights || ele.strategyChangePending}"
                >Update</button>
          </td>
        </tr>
        <tr>
          <td class="nowrap">Cleanup Local Data</td>
          <td class="nowrap">
            ${
              ele.status.cleanupRequested
                ? html`
                    ${ele.status.cleanupRequested.user} requested cleanup
                    <human-date-sk
                      .date="${ele.status.cleanupRequested.timestamp}"
                      .diff="${true}"></human-date-sk
                    >: ${ele.status.cleanupRequested.justification}
                  `
                : html`
                    <button
                      @click="${() => {
                        ele.requestCleanupDialog();
                      }}"
                      ?disabled="${!ele.editRights ||
                      ele.requestCleanupPending}">
                      Request Cleanup
                    </button>
                  `
            }
          </td>
        </tr>
      </table>
    </div>
    <div class="manual">
        ${
          ele.status.config?.supportsManualRolls
            ? html`
          ${
            !!ele.manualRolls && ele.manualRolls.length > 0
              ? html`
                  <h2>Recent Manual Rolls</h2>
                  <table>
                    <tr>
                      <th>Revision</th>
                      <th>Requested By</th>
                      <th>Requested At</th>
                      <th>Link</th>
                      <th>Result</th>
                    </tr>
                    ${ele.manualRolls.map(
                      (roll) => html`
                        <tr>
                          <td>${ele.shortRev(roll.revision)}</td>
                          <td>${roll.requester}</td>
                          <td>
                            <human-date-sk
                              .date="${roll.timestamp!}"></human-date-sk>
                          </td>
                          <td>
                            <a href="${roll.url}" , target="_blank">
                              ${roll.url}
                            </a>
                          </td>
                          <td>
                            <span class="${ele.manualRollResultClass(roll)}">
                              ${roll.result === ManualRoll_Result.UNKNOWN
                                ? ''
                                : roll.result}
                            </span>
                          </td>
                        </tr>
                      `
                    )}
                  </table>
                  <h2>Trigger Manual Rolls</h2>
                  <table></table>
                `
              : html``
          }
          ${
            !ele.notYetRolledRevisions
              ? html`
                  The roller is up to date; there are no revisions which could
                  be manually rolled.
                `
              : html``
          }
          <tr>
            <th>Request</th>
            <th>Revision</th>
            <th>Description</th>
            <th>Timestamp</th>
            <th>Invalid Reason</th>
          </tr>
          <tr class="rollCandidate">
            <td>
              <button
                  @click="${() => {
                    ele.requestManualRoll(
                      $$<HTMLInputElement>('#manualRollRevInput')!.value,
                      true
                    );
                  }}"
                  class="requestRoll"
                  ?disabled=${!ele.editRights}
                  title="${
                    ele.editRights
                      ? 'Request a dry-run to this revision.'
                      : ele.pleaseLoginMsg
                  }">
                Dry-Run
              </button>

              <button
                  @click="${() => {
                    ele.requestManualRoll(
                      $$<HTMLInputElement>('#manualRollRevInput')!.value,
                      false
                    );
                  }}"
                  class="requestRoll"
                  ?disabled=${!ele.editRights}
                  title="${
                    ele.editRights
                      ? 'Request a roll to this revision.'
                      : ele.pleaseLoginMsg
                  }">
                Roll
              </button>
            </td>
            <td>
              <input id="manualRollRevInput" placeholder="custom revision/ref"></input>
            </td>
            <td><!-- no description        --></td>
            <td><!-- no revision timestamp --></td>
            <td><!-- no invalid reason     --></td>
          </tr>
          ${ele.notYetRolledRevisions.map(
            (revision) => html`
              <tr class="rollCandidate">
                <td>
                  <button
                    @click="${() => {
                      ele.requestManualRoll(revision.id, true);
                    }}"
                    class="requestRoll"
                    ?disabled=${!ele.editRights}
                    title="${ele.editRights
                      ? 'Request a dry-run to this revision.'
                      : ele.pleaseLoginMsg}">
                    Dry-Run
                  </button>
                  <button
                    @click="${() => {
                      ele.requestManualRoll(revision.id, false);
                    }}"
                    class="requestRoll"
                    ?disabled=${!ele.editRights}
                    title="${ele.editRights
                      ? 'Request a roll to this revision.'
                      : ele.pleaseLoginMsg}">
                    Roll
                  </button>
                </td>
                <td>
                  ${revision.url
                    ? html`
                        <a href="${revision.url}" target="_blank">
                          ${revision.display}
                        </a>
                      `
                    : html` ${revision.display} `}
                </td>
                <td>
                  ${revision.description
                    ? truncate(revision.description, 100)
                    : html``}
                </td>
                <td>
                  <human-date-sk .date="${revision.time!}"></human-date-sk>
                </td>
                <td>${revision.invalidReason}</td>
              </tr>
            `
          )}
        `
            : html`
                This roller does not support manual rolls. If you want this
                feature, update the config file for the roller to enable it.
                Note that some rollers cannot support manual rolls for technical
                reasons.
              `
        }
      </table>
    </div>
    <div class="config">
      <code style="white-space: pre;">${JSON.stringify(
        ele.status.config,
        null,
        2
      )}</code>
    </div>
  </tabs-panel-sk>
  <dialog id="modeChangeDialog" class=surface-themes-sk>
    <h2>Update Mode</h2>
    <table>
      <tr>
        <td>Mode:</td>
        <td>
          <select id="modeSelect">
            ${ele.validModes.map(
              (mode: string) => html`
                <option
                  value="${mode}"
                  ?selected="${mode === ele.status?.mode?.mode}"
                  title="${ele.modeTooltip(Mode[<keyof typeof Mode>mode])}">
                  ${mode.toLowerCase().replace('_', ' ')}
                </option>
              `
            )}
          </select>
        </td>
      </tr>
      <tr>
        <td>Message:</td>
        <td>
          <textarea id="modeChangeMsgInput" rows="4" cols="50"></textarea>
        </td>
      </tr>
    </table>
    <button @click="${() => {
      ele.changeMode(false);
    }}">Cancel</button>
    <button @click="${() => {
      ele.changeMode(true);
    }}">Submit</button>
  </dialog>
  <dialog id="strategyChangeDialog" class=surface-themes-sk>
    <h2>Update Strategy</h2>
    <table>
      <tr>
        <td>Strategy:</td>
        <td>
          <select id="strategySelect">
            ${Object.keys(Strategy).map(
              (strategy: string) => html`
                <option
                  value="${strategy}"
                  ?selected="${strategy === ele.status?.strategy?.strategy}"
                  title="${ele.strategyTooltip(
                    Strategy[<keyof typeof Strategy>strategy]
                  )}">
                  ${strategy.toLowerCase().replace('_', ' ')}
                </option>
              `
            )}
          </select>
        </td>
      </tr>
      <tr>
        <td>Message:</td>
        <td>
          <textarea id="strategyChangeMsgInput" rows="4" cols="50"></textarea>
        </td>
      </tr>
    </table>
    <button @click="${() => {
      ele.changeStrategy(false);
    }}">Cancel</button>
    <button @click="${() => {
      ele.changeStrategy(true);
    }}">Submit</button>
  </dialog>
  <dialog id="requestCleanupDialog" class=surface-themes-sk>
  <h2>Request Cleanup</h2>
  <table>
    <tr>
      <td>Justification:</td>
      <td>
        <textarea id="requestCleanupJustification" rows="4" cols="50"></textarea>
      </td>
    </tr>
  </table>
  <button @click="${() => {
    ele.requestCleanup(false);
  }}">Cancel</button>
  <button @click="${() => {
    ele.requestCleanup(true);
  }}">Submit</button>
</dialog>
`;

  private requestCleanupPending: boolean = false;

  private editRights: boolean = false;

  private lastLoaded: Date = new Date(0);

  private manualRolls: ManualRoll[] = [];

  private modeChangePending: boolean = false;

  private notYetRolledRevisions: Revision[] = [];

  private readonly pleaseLoginMsg = 'Please login to make changes.';

  private recentRolls: RecentRoll[] = [];

  private refreshInterval = 60;

  private rollWindowStart: Date = new Date(0);

  private rpc: AutoRollService = GetAutoRollService(this);

  private status: AutoRollStatus | null = null;

  private strategyChangePending: boolean = false;

  private timeout: number = 0;

  private validModes: Mode[] = Object.keys(Mode).map(
    (key) => Mode[key as keyof typeof Mode]
  );

  constructor() {
    super(ARBStatusSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._upgradeProperty('roller');
    this._upgradeProperty('project');
    this._render();
    LoggedIn().then((loginstatus: Status) => {
      this.editRights = loginstatus.roles!.includes('editor');
      this._render();
    });
    this.reload();
  }

  get roller() {
    return this.getAttribute('roller') || '';
  }

  set roller(v: string) {
    this.setAttribute('roller', v);
    this.reload();
  }

  get project() {
    return this.getAttribute('project') || '';
  }

  set project(v: string) {
    this.setAttribute('project', v);
    this.reload();
  }

  private modeChangeDialog() {
    $$<HTMLDialogElement>('#modeChangeDialog', this)!.showModal();
  }

  private changeMode(submit: boolean) {
    $$<HTMLDialogElement>('#modeChangeDialog', this)!.close();
    const modeChangeSelect = <HTMLSelectElement>$$('#modeSelect');
    const modeChangeMsgInput = <HTMLInputElement>(
      $$('#modeChangeMsgInput', this)
    );
    if (!submit) {
      if (!!modeChangeSelect && !!this.status?.mode) {
        modeChangeSelect.value = this.status?.mode.mode;
      }
      return;
    }
    if (!modeChangeMsgInput || !modeChangeSelect) {
      return;
    }
    this.modeChangePending = true;
    this.rpc
      .setMode({
        message: modeChangeMsgInput.value,
        mode: Mode[
          <keyof typeof Mode>(
            modeChangeSelect.options[modeChangeSelect.selectedIndex].value
          )
        ],
        rollerId: this.roller,
      })
      .then(
        (resp: SetModeResponse) => {
          this.modeChangePending = false;
          modeChangeMsgInput.value = '';
          this.update(resp.status!);
        },
        () => {
          this.modeChangePending = false;
          this._render();
        }
      );
  }

  private strategyChangeDialog() {
    $$<HTMLDialogElement>('#strategyChangeDialog', this)!.showModal();
  }

  private changeStrategy(submit: boolean) {
    $$<HTMLDialogElement>('#strategyChangeDialog', this)!.close();
    const strategySelect = <HTMLSelectElement>$$('#strategySelect');
    const strategyChangeMsgInput = <HTMLInputElement>(
      $$('#strategyChangeMsgInput')
    );
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
    this.rpc
      .setStrategy({
        message: strategyChangeMsgInput.value,
        rollerId: this.roller,
        strategy: Strategy[<keyof typeof Strategy>strategySelect.value],
      })
      .then(
        (resp: SetStrategyResponse) => {
          this.strategyChangePending = false;
          strategyChangeMsgInput.value = '';
          this.update(resp.status!);
        },
        () => {
          this.strategyChangePending = false;
          if (this.status?.strategy?.strategy) {
            strategySelect!.value = this.status.strategy.strategy;
          }
          this._render();
        }
      );
  }

  private requestCleanupDialog() {
    $$<HTMLDialogElement>('#requestCleanupDialog', this)!.showModal();
  }

  private requestCleanup(submit: boolean) {
    $$<HTMLDialogElement>('#requestCleanupDialog', this)!.close();
    const requestCleanupJustificationInput = <HTMLInputElement>(
      $$('#requestCleanupJustification')
    );
    if (!submit) {
      return;
    }
    if (!requestCleanupJustificationInput) {
      return;
    }
    this.requestCleanupPending = true;
    this.rpc
      .addCleanupRequest({
        rollerId: this.roller,
        justification: requestCleanupJustificationInput.value,
      })
      .then(
        (resp: AddCleanupRequestResponse) => {
          this.requestCleanupPending = false;
          requestCleanupJustificationInput.value = '';
          this.update(resp.status!);
        },
        () => {
          this.requestCleanupPending = false;
          this._render();
        }
      );
  }

  // computeRollWindowStart returns a string indicating when the configured
  // roll window will start. If errors are encountered, in particular those
  // relating to parsing the roll window, the returned string will contain
  // the error.
  private computeRollWindowStart(config: AutoRollConfig): Date {
    if (!config || !config.timeWindow) {
      return new Date();
    }
    // TODO(borenet): This duplicates code in the go/time_window package.

    // parseDayTime returns a 2-element array containing the hour and
    // minutes as ints. Throws an error (string) if the given string cannot
    // be parsed as hours and minutes.
    const parseDayTime = function (s: string) {
      const timeSplit = s.split(':');
      if (timeSplit.length !== 2) {
        throw `Expected time format "hh:mm", not ${s}`;
      }
      const hours = parseInt(timeSplit[0]);
      if (hours < 0 || hours >= 24) {
        throw `Hours must be between 0-23, not ${timeSplit[0]}`;
      }
      const minutes = parseInt(timeSplit[1]);
      if (minutes < 0 || minutes >= 60) {
        throw `Minutes must be between 0-59, not ${timeSplit[1]}`;
      }
      return [hours, minutes];
    };

    // Parse multiple day/time windows, eg. M-W 00:00-04:00; Th-F 00:00-02:00
    const windows = [];
    const split = config.timeWindow.split(';');
    for (let i = 0; i < split.length; i++) {
      const dayTimeWindow = split[i].trim();
      // Parse individual day/time window, eg. M-W 00:00-04:00
      const windowSplit = dayTimeWindow.split(' ');
      if (windowSplit.length !== 2) {
        console.error(`expected format "D hh:mm", not ${dayTimeWindow}`);
        return new Date();
      }
      const dayExpr = windowSplit[0].trim();
      const timeExpr = windowSplit[1].trim();

      // Parse the starting and ending times.
      const timeExprSplit = timeExpr.split('-');
      if (timeExprSplit.length !== 2) {
        console.error(`expected format "hh:mm-hh:mm", not ${timeExpr}`);
        return new Date();
      }
      let startTime;
      try {
        startTime = parseDayTime(timeExprSplit[0]);
      } catch (e) {
        console.error(e);
        return new Date();
      }
      let endTime;
      try {
        endTime = parseDayTime(timeExprSplit[1]);
      } catch (e) {
        console.error(e);
        return new Date();
      }

      // Parse the day(s).
      const allDays = ['Su', 'M', 'Tu', 'W', 'Th', 'F', 'Sa'];
      const days = [];

      // "*" means every day.
      if (dayExpr === '*') {
        days.push(...allDays.map((_, i) => i));
      } else {
        const rangesSplit = dayExpr.split(',');
        for (let i = 0; i < rangesSplit.length; i++) {
          const rangeSplit = rangesSplit[i].split('-');
          if (rangeSplit.length === 1) {
            const day = allDays.indexOf(rangeSplit[0]);
            if (day === -1) {
              console.error(`Unknown day ${rangeSplit[0]}`);
              return new Date();
            }
            days.push(day);
          } else if (rangeSplit.length === 2) {
            const startDay = allDays.indexOf(rangeSplit[0]);
            if (startDay === -1) {
              console.error(`Unknown day ${rangeSplit[0]}`);
              return new Date();
            }
            let endDay = allDays.indexOf(rangeSplit[1]);
            if (endDay === -1) {
              console.error(`Unknown day ${rangeSplit[1]}`);
              return new Date();
            }
            if (endDay < startDay) {
              endDay += 7;
            }
            for (let day = startDay; day <= endDay; day++) {
              days.push(day % 7);
            }
          } else {
            console.error(`Invalid day expression ${rangesSplit[i]}`);
            return new Date();
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
        next = new Date(next.getTime() + 7 * 24 * 60 * 60 * 1000);
      }
      return next;
    });

    // Pick the next window.
    openTimes.sort((a, b) => a.getTime() - b.getTime());
    const rollWindowStart = openTimes[0].toString();
    return openTimes[0];
  }

  private issueURL(status: AutoRollStatus, roll: AutoRollCL): string {
    if (roll) {
      return (status?.issueUrlBase || '') + roll.id;
    }
    return '';
  }

  private modeTooltip(mode: Mode): string {
    switch (mode) {
      case Mode.RUNNING:
        return 'RUNNING is the typical operating mode of the autoroller. It will upload and land CLs as new revisions appear in the Child.';
      case Mode.DRY_RUN:
        return 'DRY_RUN is similar to RUNNING but does not land the roll CLs after the commit queue finishes. Instead, the active roll is left open until new revisions appear in the child, at which point the roll is closed and a new one is uploaded.';
      case Mode.STOPPED:
        return 'STOPPED prevents the autoroller from uploading any CLs. The roller will continue to update any local checkouts to prevent them from getting too far out of date, and any requested manual rolls will be fulfilled.';
      case Mode.OFFLINE:
        return 'OFFLINE is similar to STOPPED, but the roller does not update its checkouts and requests for manual rolls are ignored.';
      default:
        return '';
    }
  }

  private strategyTooltip(strategy: Strategy): string {
    switch (strategy) {
      case Strategy.BATCH:
        return 'BATCH rolls all new revisions in a single CL';
      case Strategy.N_BATCH:
        return 'N_BATCH rolls multiple new revisions in a single CL with a limit on the number of revisions';
      case Strategy.SINGLE:
        return 'SINGLE rolls one revision per CL';
      default:
        return '';
    }
  }

  private reloadChanged() {
    const refreshIntervalInput = <HTMLInputElement>(
      $$('refreshIntervalInput', this)
    );
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
    if (!this.roller) {
      return;
    }
    console.log(`Loading status for ${this.roller}...`);
    this.rpc
      .getStatus({
        rollerId: this.roller,
      })
      .then((resp: GetStatusResponse) => {
        this.update(resp.status!);
        this.resetTimeout();
      })
      .catch(() => {
        this.resetTimeout();
      });
  }

  private manualRollResultClass(req: ManualRoll) {
    if (!req) {
      return '';
    }
    switch (req.result) {
      case ManualRoll_Result.SUCCESS:
        return 'fg-success';
      case ManualRoll_Result.FAILURE:
        return 'fg-failure';
      default:
        return '';
    }
  }

  private requestManualRoll(rev: string, dryRun: boolean) {
    // Make sure the user wants to proceed.
    let confirmMsg = 'Proceed with requesting manual ';
    if (dryRun) {
      confirmMsg += 'dry-run?';
    } else {
      confirmMsg += 'roll?';
    }
    const confirmed = window.confirm(confirmMsg);
    if (!confirmed) {
      return;
    }

    this.rpc
      .createManualRoll({
        revision: rev,
        rollerId: this.roller,
        dryRun: dryRun,
      })
      .then((resp: CreateManualRollResponse) => {
        this.manualRolls.unshift(resp.roll!);
        const manualRollRevInput = <HTMLInputElement>$$('#manualRollRevInput');
        if (manualRollRevInput) {
          manualRollRevInput.value = '';
        }
        this._render();
      });
  }

  private rollClass(roll: AutoRollCL) {
    if (!roll) {
      return 'unknown';
    }
    switch (roll.result) {
      case AutoRollCL_Result.SUCCESS:
        return 'fg-success';
      case AutoRollCL_Result.FAILURE:
        return 'fg-failure';
      case AutoRollCL_Result.IN_PROGRESS:
        return 'fg-unknown';
      case AutoRollCL_Result.DRY_RUN_SUCCESS:
        return 'fg-success';
      case AutoRollCL_Result.DRY_RUN_FAILURE:
        return 'fg-failure';
      case AutoRollCL_Result.DRY_RUN_IN_PROGRESS:
        return 'fg-unknown';
      default:
        return 'fg-unknown';
    }
  }

  private rollResult(roll: AutoRollCL) {
    if (!roll) {
      return 'unknown';
    }
    return roll.result.toLowerCase().replace('_', ' ');
  }

  private manualRollResult(roll: ManualRoll) {
    if (!roll) {
      return 'unknown';
    }
    if (roll.status === ManualRoll_Status.COMPLETED) {
      return roll.result.toLowerCase();
    }
    if (roll.status === ManualRoll_Status.STARTED) {
      return AutoRollCL_Result.IN_PROGRESS;
    }
    if (roll.status === ManualRoll_Status.PENDING) {
      return roll.status.toLowerCase();
    }
    return 'unknown';
  }

  private modeClass(mode: Mode): string {
    switch (mode) {
      case Mode.RUNNING:
        return 'fg-running';
      case Mode.DRY_RUN:
        return 'fg-dry-run';
      case Mode.STOPPED:
        return 'fg-stopped';
      case Mode.OFFLINE:
        return 'fg-offline';
      default:
        return '';
    }
  }

  private stateClass(state: string) {
    // TODO(borenet): State could probably be an enum.
    const statusClassMap: { [key: string]: string } = {
      idle: 'fg-unknown',
      active: 'fg-unknown',
      success: 'fg-success',
      failure: 'fg-failure',
      throttled: 'fg-failure',
      'dry run idle': 'fg-unknown',
      'dry run active': 'fg-unknown',
      'dry run success': 'fg-success',
      'dry run success; leaving open': 'fg-success',
      'dry run failure': 'fg-failure',
      'dry run throttled': 'fg-failure',
      stopped: 'fg-stopped',
      offline: 'fg-offline',
    };
    return statusClassMap[state] || '';
  }

  private statusClass(status: AutoRollStatus) {
    if (status.miniStatus?.numBehind === 0) {
      return 'fg-success';
    }
    if (
      status.notRolledRevisions?.every(
        (rev: Revision) => rev.invalidReason !== ''
      )
    ) {
      return 'fg-success';
    }
    return 'fg-unknown';
  }

  private statusMessage(status: AutoRollStatus) {
    if (status.miniStatus?.numBehind === 0) {
      return 'up to date';
    }
    if (
      status.notRolledRevisions?.every(
        (rev: Revision) => rev.invalidReason !== ''
      )
    ) {
      return `up to date (${status.notRolledRevisions.length} not-yet-rolled revisions are invalid)`;
    }
    if (status.notRolledRevisions?.length === 1) {
      return '1 revision behind';
    }
    return `${status.notRolledRevisions?.length} revisions behind`;
  }

  private trybotClass(tryjob: TryJob) {
    switch (tryjob.result) {
      case TryJob_Result.SUCCESS:
        return 'fg-success';
      case TryJob_Result.FAILURE:
        return 'fg-failure';
      case TryJob_Result.CANCELED:
        return 'fg-failure';
      default:
        return 'fg-unknown';
    }
  }

  private unthrottle() {
    this.rpc.unthrottle({
      rollerId: this.roller,
    });
  }

  private update(status: AutoRollStatus) {
    if (status.notRolledRevisions) {
      this.notYetRolledRevisions = status.notRolledRevisions;
    }
    if (status.manualRolls) {
      this.manualRolls = status.manualRolls;
    }
    // Interleave regular rolls with manual rolls for display in the table.
    this.recentRolls = (status.recentRolls || [])
      .map((cl: AutoRollCL) => ({
        class: this.rollClass(cl),
        timestamp: cl.created!,
        result: this.rollResult(cl),
        rollingTo: cl.rollingTo,
        subject: cl.subject,
        url: this.issueURL(status, cl),
      }))
      .concat(
        (status.manualRolls || []).map((cl: ManualRoll) => ({
          class: this.manualRollResultClass(cl),
          timestamp: cl.timestamp!,
          result: this.manualRollResult(cl),
          rollingTo: cl.revision,
          subject: `${cl.canary ? 'Canary' : 'Manual'} roll to ${cl.revision}`,
          url: cl.url,
        }))
      );
    this.recentRolls.sort(
      (a: RecentRoll, b: RecentRoll) =>
        new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime()
    );
    const numRecentRolls = Math.max((status.recentRolls || []).length, 10);
    if (this.recentRolls.length > numRecentRolls) {
      this.recentRolls.length = numRecentRolls;
    }

    // Set the favicon.
    const link = document.createElement('link');
    link.id = 'dynamicFavicon';
    link.rel = 'shortcut icon';
    link.href = ((lastRoll: AutoRollCL | undefined, mode: Mode) => {
      if (mode === Mode.STOPPED) {
        return '/dist/img/favicon-stopped.svg';
      }
      if (!lastRoll) {
        return '/dist/img/favicon-unknown.svg';
      }
      switch (lastRoll.result) {
        case AutoRollCL_Result.SUCCESS:
          return '/dist/img/favicon-success.svg';
        case AutoRollCL_Result.FAILURE:
          return '/dist/img/favicon-failure.svg';
        case AutoRollCL_Result.IN_PROGRESS:
          return '/dist/img/favicon-unknown.svg';
        case AutoRollCL_Result.DRY_RUN_SUCCESS:
          return '/dist/img/favicon-success.svg';
        case AutoRollCL_Result.DRY_RUN_FAILURE:
          return '/dist/img/favicon-failure.svg';
        case AutoRollCL_Result.DRY_RUN_IN_PROGRESS:
          return '/dist/img/favicon-unknown.svg';
        default:
          return '/dist/img/favicon-unknown.svg';
      }
    })(status.lastRoll, status.mode!.mode);
    const head = document.getElementsByTagName('head')[0];
    const oldIcon = document.getElementById(link.id);
    if (oldIcon) {
      head.removeChild(oldIcon);
    }
    head.appendChild(link);

    this.lastLoaded = new Date();
    this.validModes = Object.keys(Mode).map(
      (key) => Mode[key as keyof typeof Mode]
    );
    if (status.config) {
      this.rollWindowStart = this.computeRollWindowStart(status.config);
      if ((status.config.validModes || []).length > 0) {
        this.validModes = status.config.validModes!;
      }
    }
    this.status = status;
    console.log('Loaded status.');
    this._render();
  }

  private shortRev(revision: string) {
    // If the revision looks like a Git commit hash, shorten it. Otherwise, we
    // can't guarantee that we won't cut off some critical information.
    const re = /[a-fA-F0-9]{40}/;
    if (re.test(revision)) {
      return revision.substring(0, 12);
    }
    return revision;
  }
}

define('arb-status-sk', ARBStatusSk);
