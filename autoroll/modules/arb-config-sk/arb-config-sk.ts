/**
 * @module autoroll/modules/arb-config-sk
 * @description <h2><code>arb-config-sk</code></h2>
 *
 * <p>
 * This element provides UI for editing the configuration of a roller.
 * </p>
 */

import { html, TemplateResult } from 'lit-html';
import { repeat } from 'lit-html/directives/repeat';

import { $$ } from 'common-sk/modules/dom';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';

import { define } from 'elements-sk/define';
import 'elements-sk/icon/add-icon-sk';
import 'elements-sk/icon/delete-icon-sk';
import 'elements-sk/icon/help-icon-sk';
import 'elements-sk/styles/buttons';
import 'elements-sk/styles/select';
import 'elements-sk/tabs-panel-sk';
import 'elements-sk/tabs-sk';

import { ElementSk } from '../../../infra-sk/modules/ElementSk';

import { CommitMsgConfig, CommitMsgConfig_BuiltIn, Config, GerritConfig_Config } from '../config';

const inputID = 'configInput';

const tr = (...cells: TemplateResult[]) => html`
  <tr>
    ${cells.map((cell: TemplateResult) => html`
      <td>${cell}</td>
    `)}
  </tr>
`;

const table = (...rows: TemplateResult[]) => html`
  <table>
    ${rows.map((row: TemplateResult) => row)}
  </table>
`;

const label = (text: string, helpMsg: string) => html`
  ${text}
  <div class="help-tooltip">
    <help-icon-sk></help-icon-sk>
    <span class="help-tooltip-text surface-themes-sk">${helpMsg}</span>
  </div>
`;

const configSection = (title: string, rows: TemplateResult[]) => html`
  <div>
    <h2>${title}</h2>
    ${table(...rows)}
  </div>
`;

class ConfigSelectOption {
  key: string = "";
  displayName: string = "";
  fn: () => TemplateResult[] = () => [];
  default: any = {};
}

export class ARBConfigSk extends ElementSk {
  private static template = (ele: ARBConfigSk) =>
    !ele.config
      ? html``
      : html`
  <div>
    <button
      @click="${() => {
          $$<HTMLTextAreaElement>('#' + inputID, ele)!.value = ele.configJSON;
        }}"
      title="Revert to the checked-in config."
      >Revert</button>
    <button
      @click="${() => ele.submit()}"
      title="Update the roller config."
    >Submit</button>
  </div>
  <div id="editor">
    <div>
    ${configSection("Core Configuration", [
          ele.stringField(ele.config, "rollerName", "Roller Name", "Unique identifier for this roller."),
          ele.stringField(ele.config, "childDisplayName", "Child Display Name", "Human-friendly display name for the child project which is being rolled."),
          ele.stringField(ele.config, "parentDisplayName", "Parent Display Name", "Human-friendly display name for the parent project into which the child is being rolled."),
          ele.stringField(ele.config, "parentWaterfall", "Parent Waterfall URL", "URL of the waterfall display for the parent project.  This is to provide a convenient link to determine whether rolls have broken the parent project."),
          ele.stringField(ele.config, "ownerPrimary", "Primary Owner", "Primary owner of the roller.  This is someone on the Skia Infrastructure team who will be responsible for managing the roller."),
          ele.stringField(ele.config, "ownerSecondary", "Secondary Owner", "Secondary owner of the roller.  This is someone on the Skia Infrastructure team who will be responsible for managing the roller when the primary owner is unavailable."),
          ele.listField(ele.config, "reviewer", "Reviewers", "People who should review CLs uploaded by the roller. These can either be email addresses or URL(s) from which to obtain reviewers, eg. on-call services."),
        ])}
    ${configSection("Commit Message", ele.commitMsgConfig(ele.config, "commitMsg"))}
    ${configSection("Code Review", ele.codeReviewConfig(ele.config))}
    ${configSection("Repo Manager", ele.repoManagerConfig(ele.config))}
    </div>
    <div>
      <div>Raw Configuration</div>
      <textarea
        id="${inputID}"
        label="Edit the roller config."
        rows=${ele.configJSON.split('\n').length}
        cols=120
        >${ele.configJSON}</textarea>
    </div>
  </div>
  <div style="display:none">
    <form id="configForm" action="/config" method="post">
      <textarea id="configJson" name="configJson"></textarea>
    </form>
  </form>
`;

  private config: Config = {} as Config;
  private configJSON: string = "";

  constructor() {
    super(ARBConfigSk.template);
  }

  inputText(configObj: any, targetPath: string, placeholder: string) {
    return html`
    <input
        id="${targetPath + "_" + (configObj[targetPath] || "")}"
        type="text"
        value="${configObj[targetPath] ? configObj[targetPath] : ""}"
        placeholder="${placeholder}"
        @change=${(ev: InputEvent) => {
        configObj[targetPath] = (<HTMLInputElement>ev.target).value;
        this.updateConfig();
      }}>
    </input>
  `;
  }

  stringField(configObj: any, targetPath: string, labelString: string, helpString: string) {
    return tr(
      label(labelString, helpString),
      this.inputText(configObj, targetPath, labelString)
    );
  }

  inputTextArea(configObj: any, targetPath: string, rows: number) {
    return html`
    <textarea
        rows="${rows}"
        width="100%"
        @change=${(ev: InputEvent) => {
        configObj[targetPath] = (<HTMLTextAreaElement>ev.target).value;
        this.updateConfig();
      }}>${configObj[targetPath] ? configObj[targetPath] : ""}</textarea>
  `;
  }

  textAreaField(configObj: any, targetPath: string, labelString: string, helpString: string, rows: number) {
    return tr(
      label(labelString, helpString),
      this.inputTextArea(configObj, targetPath, rows)
    );
  }

  inputBool(configObj: any, targetPath: string) {
    return html`
    <input
        type="checkbox"
        ?checked=${configObj[targetPath]}
        @change=${(ev: InputEvent) => {
        configObj[targetPath] = (<HTMLInputElement>ev.target).checked;
        this.updateConfig();
      }}>
    </input>
    `;
  }

  boolField(configObj: any, targetPath: string, labelString: string, helpString: string) {
    return tr(
      label(labelString, helpString),
      this.inputBool(configObj, targetPath),
    );
  }

  inputList(configObj: any, targetPath: string, placeholder: string) {
    return html`<table>
      ${repeat(configObj[targetPath] || [],
      (item: string, index: number) => item + "_" + index,
      (_: string, index: number) => tr(
        this.inputText(configObj[targetPath], "" + index, placeholder),
        html`
            <button @click=${(_: MouseEvent) => {
            configObj[targetPath].splice(index, 1);
            this.updateConfig();
          }}>
              <delete-icon-sk></delete-icon-sk>
            </button>
        `,
      )
    )}
  </table>
  <button @click=${(_: MouseEvent) => {
        if (!configObj[targetPath]) {
          configObj[targetPath] = [];
        }
        configObj[targetPath].push(null);
        this.updateConfig();
      }
      }>
  <add-icon-sk></add-icon-sk>
  </button>
    `;
  }

  listField(configObj: any, targetPath: string, labelString: string, helpString: string) {
    return tr(
      label(labelString, helpString),
      this.inputList(configObj, targetPath, labelString),
    );
  }

  inputEnum(configObj: any, targetPath: string, options: string[]) {
    return html`
      <select
        @change=${(ev: InputEvent) => {
        const select = <HTMLSelectElement>ev.target;
        configObj[targetPath] = options[select.selectedIndex];
        this.updateConfig();
      }}
        >
        ${options.map((option: string) => html`
          <option
              value="${option}"
              ?selected=${configObj[targetPath] === option}
              >${option}</option>
        `)}
    </select>
    `;
  }

  enumField(configObj: any, targetPath: string, labelString: string, helpString: string, options: string[]) {
    return tr(
      label(labelString, helpString),
      this.inputEnum(configObj, targetPath, options),
    );
  }

  commitMsgConfig(configObj: any, targetPath: string): TemplateResult[] {
    let commitMsg = <CommitMsgConfig>configObj[targetPath] || {};
    return [
      this.stringField(
        commitMsg,
        "bugProject",
        "Bug Project",
        "Monorail project name for the parent project. Optional. If not specified, bug links will not be carried over from revisions of the child project into the roll CLs.",
      ),
      this.stringField(
        commitMsg,
        "childLogUrlTmpl",
        "Log URL Template for Child Project",
        "Golang-style text template used for creating a link to the revision log of the child project for a given roll.",
      ),
      this.boolField(
        commitMsg,
        "cqDoNotCancelTrybots",
        "Disable Tryjob Cancellation",
        "By default, tryjobs are canceled when a new patchset is uploaded or the change is abandoned.  If this is checked, they are not canceled.",
      ),
      this.boolField(
        commitMsg,
        "includeLog",
        "Include Revision Log",
        "If checked, include the revision log in the commit message.",
      ),
    ].concat(
      this.configSelect(commitMsg, "Commit Message Template", "Commit message template to use; either a built-in template or a completely custom template may be provided.", [
        {
          key: "builtIn",
          displayName: "Built In",
          fn: () => {
            return [
              this.enumField(commitMsg, "builtIn", "Built-in Template", "Pre-defined commit message template.", Object.keys(CommitMsgConfig_BuiltIn)),
            ];
          },
          default: "DEFAULT",
        },
        {
          key: "custom",
          displayName: "Custom",
          fn: () => {
            return [
              this.textAreaField(commitMsg, "custom", "Custom Template", "Enter a custom commit message template.", 50)
            ];
          },
          default: "Enter a custom commit message template.",
        }
      ]));
  }

  configSelect(configObj: any, labelString: string, helpString: string, options: ConfigSelectOption[]): TemplateResult[] {
    let currentOption = options.find((option: ConfigSelectOption) => !!configObj[option.key]);
    if (!currentOption) {
      currentOption = options[0];
    }
    const cfg = currentOption.fn();
    return [
      tr(
        label(labelString, helpString),
        html`
        <select
        @change=${(ev: InputEvent) => {
            const select = <HTMLSelectElement>ev.target;
            options.forEach((option: ConfigSelectOption, index: number) => {
              if (index == select.selectedIndex) {
                configObj[option.key] = option.default;
              } else {
                delete configObj[option.key];
              }
            });
            // TODO(borenet): What about config objects which are shared?
            this.updateConfig();
          }}
        >
          ${options.map((option: ConfigSelectOption) => html`
            <option value="${option.key}">${option.displayName}</option>
          `)}
        </select>
        `
      )
    ].concat(cfg);
  }

  codeReviewConfig(configObj: any): TemplateResult[] {
    const options: ConfigSelectOption[] = [
      {
        key: "gerrit",
        displayName: "Gerrit",
        fn: () => this.gerritConfig(configObj["gerrit"] || {}),
        default: {},
      },
      {
        key: "github",
        displayName: "GitHub",
        fn: () => this.githubConfig(configObj["github"] || {}),
        default: {},
      },
      {
        key: "google3",
        displayName: "Google3",
        fn: () => this.google3Config(configObj["google3"] || {}),
        default: {},
      },
    ];
    return this.configSelect(configObj, "Code Review System", "Configuration for the code review system used for CLs created by the roller.", options);
  }

  gerritConfig(configObj: any) {
    return [
      this.stringField(configObj, "url", "URL", "URL of the Gerrit server."),
      this.stringField(configObj, "project", "Project", "Name of the Gerrit project associated with the repository in question."),
      this.enumField(configObj, "config", "Label Config", "Pre-defined Gerrit label configuration.", Object.keys(GerritConfig_Config)),
    ];
  }

  githubConfig(configObj: any) {
    return [
      this.stringField(configObj, "repoOwner", "Repo Owner", "GitHub user name of the owner of the repo."),
      this.stringField(configObj, "repoName", "Repo Name", "Name of the GitHub repo."),
      this.listField(configObj, "checksWaitFor", "Checks to Wait For", "TODO"),
    ];
  }

  google3Config(_: any) {
    return [html``];
  }

  repoManagerConfig(configObj: any) {
    return [];
    const options: ConfigSelectOption[] = [
      /*"parentChildRepoManager": this.parentChildRepoManagerConfig.bind(this),
      "androidRepoManager": this.androidRepoManagerConfig.bind(this),
      "commandRepoManager": this.commandRepoManagerConfig.bind(this),
      "freetypeRepoManager": this.freetypeRepoManagerConfig.bind(this),
      "fuchsiaSDKAndroidRepoManager": this.fuchsiaSDKAndroidRepoManagerConfig.bind(this),
      "google3RepoManager": this.google3RepoManagerConfig.bind(this),*/
    ]
    return this.configSelect(configObj, "Type", "TODO", options);
  }

  connectedCallback() {
    super.connectedCallback();
    const params = new URLSearchParams(window.location.search);
    const rollerName = params.get('roller');
    if (rollerName) {
      this.loadConfig(rollerName);
    }
    this._render();
  }

  private loadConfig(roller: string) {
    console.log('Loading config for ' + roller + '...');

    fetch('/r/' + roller + '/config').then(jsonOrThrow).then((cfg: Config) => {
      this.config = cfg;
      this.updateConfig();
    });
  }

  private updateConfig() {
    this.configJSON = JSON.stringify(this.config, null, 2);
    this._render();
  }

  private submit() {
    // TODO(borenet): This is goofy because we have two textareas which both
    // contain the config in JSON format, but eventually we'll have UI for
    // editing the config.
    const configJSON = $$<HTMLTextAreaElement>('#' + inputID, this)!.value;
    const config = JSON.parse(configJSON) as Config;
    $$<HTMLTextAreaElement>('#configJson', this)!.value = configJSON;
    $$<HTMLFormElement>("#configForm", this)!.submit();
  }
}

define('arb-config-sk', ARBConfigSk);
