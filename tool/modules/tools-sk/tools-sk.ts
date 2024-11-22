/**
 * @module modules/tools-sk
 * @description <h2><code>tools-sk</code></h2>
 *
 * Main page for the tools application.
 *
 */
import { TemplateResult, html } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import '../../../infra-sk/modules/app-sk';
import '../../../infra-sk/modules/theme-chooser-sk';
import '../../../elements-sk/modules/error-toast-sk';
import '../../../elements-sk/modules/spinner-sk';
import {
  Tool,
  Domains,
  Audiences,
  AdoptionStage,
  Phases,
  CreateOrUpdateResponse,
} from '../json';
import { stateReflector } from '../../../infra-sk/modules/stateReflector';
import { HintableObject } from '../../../infra-sk/modules/hintable';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { errorMessage } from '../../../elements-sk/modules/errorMessage';
import { $$ } from '../../../infra-sk/modules/dom';
import { SpinnerSk } from '../../../elements-sk/modules/spinner-sk/spinner-sk';

/** The different views that ToolsSk can show. */
type view = 'bygroup' | 'individual' | 'edit';

// The following Records map the Go enums to display strings, which will also
// catch if any new enum values are added or removed from the Go code.
const adoptionStageDisplay: Record<AdoptionStage, string> = {
  All: 'All customers welcome',
  No: 'No new customers',
  Conditionally: 'Some new customers',
};

const audiencesDisplay: Record<Audiences, string> = {
  Any: 'Any',
  Chrome: 'Chrome',
  ChromeOS: 'Chrome OS',
  Android: 'Android',
  PEEPSI: 'PEEP Shared Infra',
  Skia: 'Skia',
};

const phasesDisplay: Record<Phases, string> = {
  GA: 'Generally Available',
  Deprecated: 'Deprecated',
  Preview: 'Preview',
};

const domainDisplay: Record<Domains, string> = {
  Build: 'Build',
  Debugging: 'Debugging',
  Development: 'Development',
  Logging: 'Logging',
  Other: 'Other',
  Release: 'Release',
  Security: 'Security',
  Source: 'Source',
  Testing: 'Testing',
};

const toolInstanceForCreate: Tool = {
  id: '',
  domain: 'Build',
  display_name: '',
  description: '',
  phase: 'GA',
  teams_id: '',
  code_path: [],
  audience: [],
  adoption_stage: 'All',
  landing_page: '',
  docs: {},
  feedback: {},
  resources: {},
};

/**
 * State of the app, which is reflected into the URL.
 */
class State {
  view: view = 'bygroup';

  audience: Audiences = 'Any';

  toolID: string = '';
}

/**
 * Returns true if `a` is in `arr`, or if `a` === 'Any'.
 */
const inAudiences = (a: Audiences, arr: Audiences[] | null): boolean => {
  if (a === 'Any') {
    return true;
  }
  if (arr == null) {
    return false;
  }
  return arr.indexOf(a) !== -1;
};

export class ToolsSk extends ElementSk {
  private state: State = new State();

  private tools: Tool[] = [];

  private byDomain: Map<Domains, Tool[]> = new Map<Domains, Tool[]>();

  private allAudiences: Audiences[] = [];

  private stateHasChanged: (() => void) | null = null;

  constructor() {
    super(ToolsSk.template);
  }

  private static template = (ele: ToolsSk) => html`
    <app-sk>
      <header>
        <h1><a href="/">Tools</a></h1>
        <div class="spacer"></div>
        <theme-chooser-sk></theme-chooser-sk>
      </header>
      <main>
        <div>${ele.view()}</div>
        <error-toast-sk></error-toast-sk>
      </main>
    </app-sk>
  `;

  /**
   * Displays the main view, which is determined by the State of the application.
   */
  view(): TemplateResult {
    switch (this.state.view) {
      case 'bygroup':
        return this.byGroupView();
        break;
      case 'individual':
        return this.indView();
        break;
      case 'edit':
        return this.editView();
        break;
      default:
        return html``;
    }
  }

  /**
   * Displays all the Tools for a given audience. Allows selecting which audience to display.
   */
  byGroupView(): TemplateResult {
    return html`
      <div class="topbar">
        <p class="intro">
          Tools lists the common and recommended tools for solving developer
          problems for the following audiences: ${this.allAudiences.join(', ')}.
        </p>
        <button
          id="new-tool"
          @click=${() => this.createNew()}
          title="Create a new tool entry.">
          New
        </button>
      </div>
      <div class="selectAudience">
        <label for="audience">Audience</label>
        <select
          name="audience"
          id="audience"
          @input=${(e: InputEvent) => this.audienceChanged(e)}>
          ${this.allAudiences.map(
            (a: Audiences) =>
              html` <option value="${a}" ?selected=${this.state.audience === a}>
                ${a}
              </option>`
          )}
        </select>
      </div>
      ${this.domains()}
    `;
  }

  /**
   * Displays all the information for a single Tool.
   */
  indView(): TemplateResult {
    const tool: Tool | undefined = this.tools.find(
      (t: Tool): boolean => t.id === this.state.toolID
    );
    if (!tool) {
      return html`<h2>Not Found</h2>
        <p class="error">No tool with that ID was found.</p> `;
    }
    return html`
      <h1>
        ${tool.display_name}
        <button
          id=ind-edit-button
          class=button-like
          @click="${() => this.clickEdit()}"
          title="Edit the data for this tool."
          >
          Edit
      </button>
      </h1>
      <p class="description">${tool.description}</p>
      <p>
        <div class="info">
          <a href="${tool.landing_page}">Landing Page</a>
        </div>
        <div class="info">
          <a href="http://team/${tool.teams_id}">Team</a>
        </div>
        <div class="info">Domain: ${tool.domain}</div>
        <div class="info">Phase:
          <span class="${tool.phase}">${tool.phase} </span>
        </div>
        <div class="info">
          Customer Adoption Stage: ${tool.adoption_stage}
        </div>
      </p>
      <p>
        ${this.displayMap('Documentation', tool.docs)}
        ${this.displayMap('Feedback', tool.feedback)}
        ${this.displayMap('Resources', tool.resources)} ${this.codePaths(tool)}
      </p>
    `;
  }

  /**
   * Displays an HTML form for editing an existing Tool, or creating a new one.
   */
  editView(): TemplateResult {
    let tool: Tool | undefined = this.tools.find(
      (t: Tool): boolean => t.id === this.state.toolID
    );
    if (!tool) {
      tool = toolInstanceForCreate;
    }

    return html`
      <p>
        Submitting this form will create a new CL that adds or updates the JSON file that describes the tool. Once
        reviewed and submitted the Tools UI will update with
        the new information in a few minutes.
      </p>
      <form @submit=${(e: SubmitEvent) => this.editSubmit(e)} id=editForm>
        <div>
          <label for="id">ID: <span class=instructions>A short unique id for this tool, used as a file name.</span> </label>
          <input
            type="text"
            name="id"
            id="id"
            pattern="\\w+"
            required
            value="${tool.id}" />
        </div>

        <div>
          <label for="domain">Domain: <span class=instructions>The general category this tool occupies.</span></label>
          <select name="domain" id="domain">
            ${Object.keys(domainDisplay).map(
              (key): TemplateResult => html`
                <option
                  value="${key}"
                  ?selected=${tool!.domain === (key as Domains)}>
                  ${domainDisplay[key as Domains]}
                </option>
              `
            )}
          </select>
        </div>

        <div>
          <label for="display_name">Display Name: </label>
          <input
            type="text"
            name="display_name"
            id="display_name"
            pattern=".+"
            required
            value="${tool.display_name}" />
        </div>

        <div>
          <label for="description">Description: </label>
          <textarea
            rows=5
            cols=120
            name="description"
            id="description"
            required
            value="">${tool.description}</textarea>

        <div>
          <label for="phase">Phase: </label>
          <select name="phase" id="phase">
            ${Object.keys(phasesDisplay).map(
              (key): TemplateResult => html`
                <option
                  value="${key}"
                  ?selected=${tool!.phase === (key as Phases)}>
                  ${phasesDisplay[key as Phases]}
                </option>
              `
            )}
          </select>
        </div>

        <div>
          <label for="teams_id">Teams ID: </label>
          <input
            type="text"
            name="teams_id"
            id="teams_id"
            pattern="\\d+"
            required
            value="${tool.teams_id}" />
        </div>


        <div>
          <label for="audience">Audience: <span class=instructions>Groups that use this tool. CTRL+click to select multiple audiences.</span></label>
          <select
            name="audience"
            id="audience"
            multiple
            size=${Object.keys(audiencesDisplay).length}>
            ${Object.keys(audiencesDisplay).map(
              (key): TemplateResult => html`
                <option
                  value="${key}"
                  ?selected=${inAudiences(key as Audiences, tool!.audience)}>
                  ${audiencesDisplay[key as Audiences]}
                </option>
              `
            )}
          </select>
        </div>

        <div>
          <label for="adoption_stage">Adoption Stage: <span class=instructions>Is the tool accepting new customers.</span></label>
          <select name="adoption_stage" id="adoption_stage">
            ${Object.keys(adoptionStageDisplay).map(
              (key): TemplateResult => html`
                <option
                  value="${key}"
                  ?selected=${tool!.adoption_stage === (key as AdoptionStage)}>
                  ${adoptionStageDisplay[key as AdoptionStage]}
                </option>
              `
            )}
          </select>
        </div>

        <div>
          <label for="landing_page">Landing Page: </label>
          <input
            type="text"
            name="landing_page"
            id="landing_page"
            pattern=".+"
            required
            value="${tool.landing_page}" />
        </div>

        <div>
          <label for="docs">Documentation: <span class=instructions>One line per entry. Each entry
            is the display name, followed by a colon, and then the URL. E.g. 'Introduction:https://example.com/intro'</span></label>
          <textarea rows="${
            Object.keys(tool.docs || {}).length + 1
          }" cols="120" id="docs" name="docs">
${Object.entries(tool.docs!).map((entry: [string, string]) => {
  const [key, value] = entry;
  return `${key}:${value}\n`;
})}</textarea>
        </div>

        <div>
          <label for="feedback">Feedback: <span class=instructions>One line per entry. Each entry
            is the display name, followed by a colon, and then the URL. E.g. 'Introduction:https://example.com/intro'</span> </label>
          <textarea rows="${
            Object.keys(tool.feedback || {}).length + 1
          }" cols="120" id="feedback" name="feedback">
${Object.entries(tool.feedback!).map((entry: [string, string]) => {
  const [key, value] = entry;
  return `${key}:${value}\n`;
})}</textarea>
        </div>

        <div>
          <label for="resources">Resources: <span class=instructions>One line per entry. Each entry
            is the display name, followed by a colon, and then the URL. E.g. 'Introduction:https://example.com/intro'</span></label>
          <textarea rows="${
            Object.keys(tool.resources || {}).length + 1
          }" cols="120" id="resources" name="resources">
${Object.entries(tool.resources!).map((entry: [string, string]) => {
  const [key, value] = entry;
  return `${key}:${value}\n`;
})}</textarea>
        </div>

        <div>
          <label for="code_path">Code paths: <span class=instructions>One URL per line.</span> </label>
          <textarea rows="${
            (tool.code_path || []).length + 1
          }" cols="120" id="code_path" name="code_path">${(
            tool.code_path || []
          ).map((value: string) => html`${value} `)}</textarea>
        </div>

        <div class=submit>
          <input type="submit" value="Update" class="button-like action"
          title="Submit the form values to create CL that creates or updates the tool config." />
          <spinner-sk></spinner-sk>
        </div>
      </form>
    `;
  }

  /**
   * Opens the edit form to edit the current Tool
   */
  clickEdit(): void {
    this.state.view = 'edit';
    this.stateHasChanged!();
    this._render();
  }

  /**
   * Open the edit form for creating a new Tool.
   */
  createNew(): void {
    this.state.view = 'edit';
    this.state.toolID = '';
    this.stateHasChanged!();
    this._render();
  }

  /**
   * Displays a `{ [key: string]: string } | null` with the given header.
   */
  displayMap(
    header: string,
    map: { [key: string]: string } | null
  ): TemplateResult {
    if (!map || Object.keys(map).length === 0) {
      return html``;
    }
    return html`
      <section class="outline">
        <h3>${header}</h3>
        ${Object.keys(map).map(
          (key: string) => html` <div><a href="${map[key]}">${key}</a></div> `
        )}
      </section>
    `;
  }

  /**
   * Displays all the code_paths for a Tool.
   */
  codePaths(tool: Tool): TemplateResult {
    if (!tool.code_path) {
      return html``;
    }
    return html`
      <section class="outline">
        <h3>Code Paths</h3>
        ${tool.code_path.map(
          (cp: string): TemplateResult =>
            html`<div>
              <code>
                <a href="${cp}">${cp}</a>
              </code>
            </div>`
        )}
      </section>
    `;
  }

  /**
   * Displays all the Tools for the given audience grouped by Domains.
   */
  domains(): TemplateResult[] {
    const ret: TemplateResult[] = [];
    const domainKeys = [...this.byDomain.keys()].sort();
    domainKeys.forEach((key: Domains) => {
      // Only show tools relevant for the audience.
      const tools: Tool[] = (this.byDomain.get(key) || []).filter(
        (t: Tool): boolean => inAudiences(this.state.audience, t.audience)
      );

      // Don't display the Domain section if it will be empty.
      if (tools.length === 0) {
        return;
      }
      ret.push(html`
        <h2>${key}</h2>
        <table>
          ${tools.map(
            (t: Tool) => html`
              <tr>
                <td>
                  <a
                    id="link-${t.id}"
                    href=""
                    @click=${(e: Event) => this.clickIndividual(e, t.id)}>
                    ${t.display_name}
                  </a>
                </td>
                <td><span class="${t.phase}">${t.phase} </span></td>
                <td>${t.description}</td>
              </tr>
            `
          )}
        </table>
      `);
    });

    return ret;
  }

  /**
   * Handles a click on a tool by switching into the individual view
   * with just that one Tool presented.
   *
   * @param e - Event
   * @param id - id of the Tool that was clicked.
   */
  clickIndividual(e: Event, id: string): void {
    e.preventDefault();
    this.state.toolID = id;
    this.state.view = 'individual';
    this.stateHasChanged!();
    this._render();
  }

  /**
   * Handles the `submit` event on the form by parsing the form values,
   * validating them, and the posting a JSON serialized Tool to the backend.
   *
   * @param e - SubmitEvent sent when the user presses Submit.
   */
  async editSubmit(e: SubmitEvent): Promise<void> {
    e.preventDefault();

    const form = $$<HTMLFormElement>('#editForm')!;
    const spinner = $$<SpinnerSk>('spinner-sk', form)!;
    try {
      spinner.active = true;
      const tool: Tool = this.formToTool(form);
      const resp = await fetch('/_/put', {
        method: 'POST',
        credentials: 'same-origin',
        body: JSON.stringify(tool, null, '  '),
      });
      const r = (await jsonOrThrow(resp)) as CreateOrUpdateResponse;
      window.location.replace(r.url);
    } catch (message: any) {
      errorMessage(message);
    } finally {
      spinner.active = false;
    }
  }

  /**
   * formToTool parses the given form and returns a Tool from the values found
   * in the controls.
   *
   * @param form - The HTML form to parse.
   * @returns A Tool with values from the controls.
   */
  formToTool(form: HTMLFormElement): Tool {
    const ret: Tool = {
      id: $$<HTMLInputElement>('#id', form)!.value,
      domain: $$<HTMLSelectElement>('#domain', form)!.value as Domains,
      display_name: $$<HTMLInputElement>('#display_name', form)!.value,
      description: $$<HTMLTextAreaElement>('#description', form)!.textContent!,
      phase: $$<HTMLSelectElement>('#phase', form)!.value as Phases,
      teams_id: $$<HTMLInputElement>('#teams_id', form)!.value,
      code_path: $$<HTMLTextAreaElement>('#code_path', form)!
        .textContent!.trim()
        .split('\n'),
      audience: this.audienceControlToAudienceArray(form),
      adoption_stage: $$<HTMLSelectElement>('#adoption_stage', form)!
        .value as AdoptionStage,
      landing_page: $$<HTMLInputElement>('#landing_page', form)!.value,
      docs: this.textAreaToObject(form, '#docs'),
      feedback: this.textAreaToObject(form, '#feedback'),
      resources: this.textAreaToObject(form, '#resources'),
    };

    return ret;
  }

  /**
   * Returns all the values from the audience select control as an array of
   * Audiences.
   *
   * @param form The form element containing the audience control.
   * @returns An array of Audiences.
   */
  audienceControlToAudienceArray(form: HTMLFormElement): Audiences[] {
    const ret: Audiences[] = [];
    const collection = $$<HTMLSelectElement>(
      '#audience',
      form
    )!.selectedOptions;
    for (let i = 0; i < collection.length; i++) {
      ret.push(collection[i].value as Audiences);
    }
    return ret;
  }

  /**
   * textAreaToObject parses a textarea value and converts it to an object.
   *
   * The lines of the text must be are in the form of `key:value`, where the
   * value must be a valid URL.
   *
   * @param form - The form element containing the control.
   * @param id - The query that targets the textarea control in the form, e.g.
   * "#foo".
   * @returns An object that maps all the keys to their URL values.
   */
  textAreaToObject(
    form: HTMLFormElement,
    id: string
  ): { [key: string]: string } {
    const ret: { [key: string]: string } = {};
    $$<HTMLTextAreaElement>(id, form)!
      .value!.trim()
      .split('\n')
      .forEach((line: string) => {
        line = line.trim();
        if (line === '') {
          return;
        }
        // Parse out the lines, which are in the form of `key:value`, but where
        // the value could have colons in it.
        const parts = line.split(':');
        if (parts.length < 2) {
          throw Error(`Invalid format: ${parts}`);
        }
        const key = parts.shift()!;
        const value = parts.join(':');
        try {
          // Ensure that this is a valid URL.
          const u = new URL(value);
          ret[key] = u.toString();
        } catch (error) {
          throw Error(`Invalid URL: ${value}`);
        }
      });

    return ret;
  }

  async connectedCallback(): Promise<void> {
    super.connectedCallback();
    this._render();

    this.stateHasChanged = stateReflector(
      () => this.state as unknown as HintableObject,
      (state) => {
        this.state = state as unknown as State;
        this._render();
      }
    );

    try {
      // Fetch the configs from the backend.
      const req = await fetch('/_/configs');
      this.tools = (await jsonOrThrow(req)) as Tool[];

      const audiences = new Set<Audiences>(['Any']);

      // Group the tools by domain for easy presentation.
      this.tools.forEach((t: Tool) => {
        const domain = t.domain;
        const tools = this.byDomain.get(domain) || [];
        tools.push(t);
        this.byDomain.set(domain, tools);
        (t.audience || []).forEach((a: Audiences) => {
          audiences.add(a);
        });
      });

      audiences.forEach((a: Audiences) => this.allAudiences.push(a));

      this._render();
    } catch (error: any) {
      errorMessage(error);
    }
  }

  /**
   * Re-renders the page when the audience select control value is changed.
   */
  audienceChanged(e: InputEvent): void {
    this.state.audience = (e.target as HTMLSelectElement).value as Audiences;
    this.stateHasChanged!();
    this._render();
  }
}

define('tools-sk', ToolsSk);
