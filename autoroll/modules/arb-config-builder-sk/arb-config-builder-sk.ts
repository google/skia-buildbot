/**
 * @module autoroll/modules/arb-config-builder-sk
 * @description <h2><code>arb-config-builder-sk</code></h2>
 *
 * <p>
 * This element provides UI for editing the configuration of a roller.
 * </p>
 */

import { html, TemplateResult } from 'lit-html';
import { repeat } from 'lit-html/directives/repeat';

import { $$ } from 'common-sk/modules/dom';

import { define } from 'elements-sk/define';
import 'elements-sk/icon/add-icon-sk';
import 'elements-sk/icon/delete-icon-sk';
import 'elements-sk/icon/help-icon-sk';
import 'elements-sk/icon/navigate-before-icon-sk';
import 'elements-sk/icon/navigate-next-icon-sk';
import 'elements-sk/styles/buttons';
import 'elements-sk/styles/select';
import 'elements-sk/tabs-panel-sk';
import 'elements-sk/tabs-sk';

import { ElementSk } from '../../../infra-sk/modules/ElementSk';

import { Config, GerritConfig, GerritConfig_Config, GitHubConfig, ParentChildRepoManagerConfig } from '../config';

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

// TODO(borenet): It'd be nice to have help tooltips for all of these.
enum parentType {
  GITILES = "Gitiles Repo",
  GITHUB = "GitHub Repo",
  GIT_CHECKOUT = "Git Repo with Local Checkout",
}

enum childType {
  GITILES = "Gitiles Repo",
  GIT_CHECKOUT = "Git Repo with Local Checkout",
  CIPD = "CIPD Package",
  FUCHSIA_SDK = "Fuchsia SDK",
  SEMVER_GCS = "Semantic-versioned Asset in Cloud Storage",
}

const childTypeSupported = (parent: parentType, child: childType) => {
  if (parent == parentType.GITILES) {
    return true;
  } else if (parent == parentType.GITHUB) {
    return true;
  } else if (parent = parentType.GIT_CHECKOUT) {
    return true;
  }
}

enum rollMethod {
  DEPS = "DEPS File",
  COPY = "Copy Child Contents into Parent",
  FILE = "Other Version File",
}

const rollMethodSupported = (parent: parentType, method: rollMethod) => {
  if (parent == parentType.GITILES) {
    return true;
  } else if (parent == parentType.GITHUB) {
    if (method == rollMethod.COPY) {
      return false;
    } else {
      return true;
    }
  } else if (parent == parentType.GIT_CHECKOUT) {
    if (method == rollMethod.DEPS) {
      return true;
    } else {
      return false;
    }
  }
}

class SlideData {
  private _data: any | null = null;
  private _label: string;
  private _helpString: string;
  private _required: boolean;
  private tmpl: (_: SlideData) => TemplateResult;
  private _parent: SlideSet | null = null;

  constructor(label: string, helpString: string, required: boolean, tmpl: (_: SlideData) => TemplateResult) {
    this._label = label;
    this._helpString = helpString;
    this._required = required;
    this.tmpl = tmpl;
  }

  render(): TemplateResult {
    return this.tmpl(this);
  }

  get wrappedData() {
    return this._data;
  }
  set wrappedData(data: any) {
    this._data = data;
    this.notify();
  }

  get label() {
    return this._label;
  }

  get helpString() {
    return this._helpString;
  }

  get isSet(): boolean {
    return this._data !== null;
  }

  get required(): boolean {
    return this._required;
  }

  set parent(parent: SlideSet) {
    this._parent = parent;
  }

  notify() {
    if (this._parent) {
      this._parent.render();
    }
  }
}

class StringSlideData extends SlideData {
  constructor(label: string, helpString: string, required: boolean) {
    super(label, helpString, required, (slideData: SlideData) => html`
      <input
        type="text"
        value="${(<StringSlideData>slideData).data}"
        placeholder="${label}"
        @change=${(ev: InputEvent) => {
        (<StringSlideData>slideData).data = (<HTMLInputElement>ev.target).value;
      }
      }>
      </input>
    `);
  }

  get data(): string | null {
    return this.wrappedData;
  }
  set data(data: string | null) {
    this.wrappedData = data;
  }
}

class EnumSlideData extends SlideData {
  constructor(label: string, helpString: string, required: boolean, options: string[]) {
    super(label, helpString, required, (slideData: SlideData) => html`
      <select @change=${(ev: InputEvent) => {
        const target = <HTMLSelectElement>ev.target;
        (<EnumSlideData>slideData).data = options[target.selectedIndex - 1];
      }}>
        <option hidden selected>Choose an option...</option>
        ${options.map((option: string) => html`
          <option>${option}</option>
        `)}
      </select>
    `);
  }

  get data(): string | null {
    return this.wrappedData;
  }
  set data(data: string | null) {
    this.wrappedData = data;
  }
}

class Slide {
  private static template = (slide: Slide) => html`
    <div>
      <div><h2>${slide.title}</h2></div>
      <table>
        ${slide.data.map((data: SlideData) => html`
          <tr>
            <td>${label(data.label, data.helpString)}${data.required ? html`*` : html``}</td>
            <td>${data.render()}</td>
          </tr>
        `)}
      </table>
    </div>
  `;

  private title: string;
  private _data: SlideData[];

  constructor(title: string, data: SlideData[]) {
    this.title = title;
    this._data = data;
  }

  get canMoveForward(): boolean {
    return this.data.every((data: SlideData) => !data.required || data.isSet);
  }

  get data() {
    return this._data;
  }

  render() {
    return Slide.template(this);
  }
}

class SlideSet {
  private static template = (slideSet: SlideSet) => html`
    <div>
      <button
          ?disabled=${slideSet.canMoveBackward()}
          @click=${() => {
      slideSet.currentIndex--;
    }}
          >
        <navigate-before-icon-sk></navigate-before-icon-sk>
      </button>
      <button
          ?disabled=${slideSet.canMoveForward()}
          @click=${() => {
      slideSet.currentIndex++;
    }}
          >
        <navigate-next-icon-sk></navigate-next-icon-sk>
      </button>
    </div>
    <div>
      ${slideSet.currentSlide ? slideSet.currentSlide.render() : html``}
    </div>
  `;

  private _currentIndex: number = 0;
  private slides: Slide[];

  constructor(slides: Slide[]) {
    slides.forEach((slide: Slide) => {
      slide.data.forEach((data: SlideData) => {
        data.parent = this;
      });
    });
    this.slides = slides;
  }

  get currentIndex() {
    return this._currentIndex;
  }
  set currentIndex(index: number) {
    if (index < 0) {
      index = 0;
    } else if (index >= this.slides.length) {
      index = this.slides.length - 1;
    }
    this._currentIndex = index;
    this.render();
  }

  get valid(): boolean {
    return this.currentIndex >= 0 && this.currentIndex < this.slides.length;
  }

  canMoveBackward(): boolean {
    const rv = this.currentIndex > 0;
    console.log("canMoveBackward? " + rv);
    return rv;
  }

  canMoveForward(): boolean {
    const rv = this.currentSlide ? this.currentSlide.canMoveForward : false;
    console.log("canMoveForward? " + rv);
    return rv;
  }

  get currentSlide(): Slide | null {
    return this.valid ? this.slides[this.currentIndex] : null;
  }

  render() {
    // TODO(borenet): The "disabled" attribute on the buttons does not update!
    console.log("SlideSet.render");
    return SlideSet.template(this);
  }
}

export class ARBConfigBuilderSk extends ElementSk {
  private static template = (ele: ARBConfigBuilderSk) => html`
  <div id="editor">
    <div>
      ${ele.slides.render()}
    </div>
    <div>
      <div>Raw Configuration</div>
        <textarea
          id="configInput"
          label="Edit the roller config."
          rows=${ele.configJSON.split('\n').length}
          cols=120
          >${ele.configJSON}</textarea>
      </div>
    </div>
  </div>
  <div style = "display:none" >
    <form id="configForm" action="/config" method="post">
      <textarea id="configJson" name="configJson"></textarea>
    </form>
  </div>
  `;

  private _parentType: parentType | null = null;
  private _childType: childType | null = null;
  private _rollMethod: rollMethod | null = null;

  private slides: SlideSet = new SlideSet([
    new Slide("Parent Project", [
      new EnumSlideData("Parent Project Type", "Type of project which the roller will roll into.", true, Object.values(parentType)),
      new EnumSlideData("Roll Method", "Describes how the child project is rolled into the parent.", true, Object.values(rollMethod)),
    ]),
  ]);

  private config: Config = {} as Config;
  private configJSON: string = "";

  constructor() {
    super(ARBConfigBuilderSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  get parentType() {
    return this._parentType;
  }
  set parentType(typ: parentType | null) {
    this._parentType = typ;
    if (this._parentType === null) {
      const parentSelect = $$<HTMLSelectElement>("#parentSelect");
      if (parentSelect) {
        parentSelect.selectedIndex = 0;
      }
    }
    this.childType = null;
    this.rollMethod = null;
    this.updateConfig();
  }

  get childType() {
    return this._childType;
  }
  set childType(typ: childType | null) {
    this._childType = typ;
    if (this._childType === null) {
      const childSelect = $$<HTMLSelectElement>("#childSelect");
      if (childSelect) {
        childSelect.selectedIndex = 0;
      }
    }
    this.rollMethod = null;
    this.updateConfig();
  }

  get rollMethod() {
    return this._rollMethod;
  }
  set rollMethod(typ: rollMethod | null) {
    this._rollMethod = typ;
    if (this._rollMethod === null) {
      const methodSelect = $$<HTMLSelectElement>("#methodSelect");
      if (methodSelect) {
        methodSelect.selectedIndex = 0;
      }
    }
    this.updateConfig();
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
      }
      }>
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
      }
      }> ${configObj[targetPath] ? configObj[targetPath] : ""} </textarea>
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
      }
      }>
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
    )
      }
</table>
  <button @click=${(_: MouseEvent) => {
        if (!configObj[targetPath]) {
          configObj[targetPath] = [];
        }
        configObj[targetPath].push(null);
        this.updateConfig();
      }
      }>
  <add-icon-sk> </add-icon-sk>
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
      }
      }
        >
  ${options.map((option: string) => html`
          <option
              value="${option}"
              ?selected=${configObj[targetPath] === option}
              >${option}</option>
        `)
      }
</select>
  `;
  }

  enumField(configObj: any, targetPath: string, labelString: string, helpString: string, options: string[]) {
    return tr(
      label(labelString, helpString),
      this.inputEnum(configObj, targetPath, options),
    );
  }

  private updateConfig() {
    if (!!this.parentType && !!this.childType && !!this.rollMethod) {
      const rm: ParentChildRepoManagerConfig = {};
      const gerrit: GerritConfig = {
        url: "",
        project: "",
        config: GerritConfig_Config.CHROMIUM_BOT_COMMIT,
      };
      const github: GitHubConfig = {
        repoOwner: "",
        repoName: "",
        checksWaitFor: [],
      }

      switch (this.parentType) {
        case parentType.GITHUB:
          this.config.github = github;
          switch (this.rollMethod) {
            case rollMethod.DEPS:
              rm.depsLocalGithubParent = {
                depsLocal: {
                  gitCheckout: {},
                  childPath: "",
                  childSubdir: "",
                  checkoutPath: "",
                  gclientSpec: "",
                  runHooks: false,
                },
                forkRepoUrl: "",
                github: github,
              };
              break;
            case rollMethod.FILE:
              rm.gitCheckoutGithubFileParent = {
                gitCheckout: {
                  gitCheckout: {
                    gitCheckout: {
                      branch: "",
                      repoUrl: "",
                      revLinkTmpl: "",
                      dependencies: [],
                    },
                    dep: {
                      primary: {
                        id: "",
                        path: "",
                      },
                    },
                  },
                  forkRepoUrl: "",
                },
              };
              break;
          }
          break;
        case parentType.GITILES:
          this.config.gerrit = gerrit;
          switch (this.rollMethod) {
            case rollMethod.COPY:
              rm.copyParent = {
                gitiles: {
                  gitiles: {
                    branch: "",
                    repoUrl: "",
                    dependencies: [],
                  },
                  dep: {},
                  gerrit: gerrit,
                },
                copies: [],
              };
              break;
            default:
              rm.gitilesParent = {
                gitiles: {
                  branch: "",
                  repoUrl: "",
                  dependencies: [],
                },
                dep: {},
                gerrit: gerrit,
              };
              break;
          }
          break;
        case parentType.GIT_CHECKOUT:
          this.config.gerrit = gerrit;
          rm.depsLocalGerritParent = {
            depsLocal: {
              gitCheckout: {},
              childPath: "",
              childSubdir: "",
              checkoutPath: "",
              gclientSpec: "",
              runHooks: false,
            },
            gerrit: gerrit,
          };
          break;
      }
      switch (this.childType) {
        case childType.GITILES:
          rm.gitilesChild = {
            gitiles: {
              branch: "",
              repoUrl: "",
              dependencies: [],
            },
            path: "",
          };
          break;
        case childType.GIT_CHECKOUT:
          rm.gitCheckoutChild = {
            gitCheckout: {
              branch: "",
              repoUrl: "",
              revLinkTmpl: "",
              dependencies: [],
            },
          };
          break;
        case childType.CIPD:
          rm.cipdChild = {
            name: "",
            tag: "",
            gitilesRepo: "",
          };
          break;
        case childType.FUCHSIA_SDK:
          rm.fuchsiaSdkChild = {
            includeMacSdk: false,
          };
          break;
        case childType.SEMVER_GCS:
          rm.semverGcsChild = {
            gcs: {
              gcsBucket: "",
              gcsPath: "",
            },
            shortRevRegex: "",
            versionRegex: "",
          };
          break;
      }
      this.config.parentChildRepoManager = rm;
    } else {
      delete this.config.parentChildRepoManager;
      delete this.config.gerrit;
      delete this.config.github;
    }
    this.configJSON = JSON.stringify(this.config, null, 2);
    this._render();
  }

  private submit() {
    // TODO(borenet): This is goofy because we have two textareas which both
    // contain the config in JSON format, but eventually we'll have UI for
    // editing the config.
    const configJSON = $$<HTMLTextAreaElement>('#configInput', this)!.value;
    const config = JSON.parse(configJSON) as Config;
    $$<HTMLTextAreaElement>('#configJson', this)!.value = configJSON;
    $$<HTMLFormElement>("#configForm", this)!.submit();
  }
}

define('arb-config-builder-sk', ARBConfigBuilderSk);
