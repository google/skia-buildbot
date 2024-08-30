import { html } from 'lit/html.js';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { stateReflector } from '../../../infra-sk/modules/stateReflector';
import { define } from '../../../elements-sk/modules/define';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { GetScreenshotsRPCResponse, Screenshot } from '../rpc_types';

export class ScreenshotsViewerSk extends ElementSk {
  private static template = (el: ScreenshotsViewerSk) => html`
    <div class="main body-sk font-sk">
      <h1>Puppeteer screenshots viewer</h1>

      <div class="filter">
        <input
          type="text"
          placeholder="Filter screenshots by name (fuzzy)"
          .value=${el.filter}
          @input=${(e: Event) => el.onFilterInput(e)} />
        <button @click=${() => el.onClearClick()}>Clear</button>
      </div>

      ${ScreenshotsViewerSk.screenshotsTemplate(el)}
    </div>
  `;

  private static screenshotsTemplate = (el: ScreenshotsViewerSk) => {
    if (!el.loaded) {
      return html`<p class="loading">Loading...</p>`;
    }
    if (el.getApplications().length === 0) {
      if (el.filter !== '') {
        return html`<p class="no-results">
          No screenshots match "${el.filter}".
        </p>`;
      }
      return html`<p class="no-results">
        No screenshots found. Try re-running your tests.
      </p>`;
    }

    return html`
      <div class="applications">
        ${el
          .getApplications()
          .map((app: string) =>
            ScreenshotsViewerSk.applicationTemplate(el, app)
          )}
      </div>
    `;
  };

  private static applicationTemplate = (
    el: ScreenshotsViewerSk,
    app: string
  ) => html`
    <div class="application">
      <h2 class="application-name">${app}</h2>

      ${el
        .getScreenshotsForApplication(app)
        .map((screenshot: Screenshot) =>
          ScreenshotsViewerSk.screenshotTemplate(screenshot)
        )}
    </div>
  `;

  private static screenshotTemplate = (screenshot: Screenshot) => html`
    <figure class="screenshot">
      <figcaption class="test-name">${screenshot.test_name}</figcaption>
      <img title="${screenshot.test_name}" src="${screenshot.url}" />
    </figure>
  `;

  private loaded = false;

  private screenshotsByApplication: { [key: string]: Screenshot[] } = {};

  private filter = '';

  private readonly stateChanged: () => void;

  constructor() {
    super(ScreenshotsViewerSk.template);

    // stateReflector will trigger on DomReady.
    this.stateChanged = stateReflector(
      /* getState */ () => ({
        filter: this.filter,
      }),
      /* setState */ (newState) => {
        if (!this._connected) {
          return;
        }
        this.filter = (newState.filter as string) || '';
        this._render();
      }
    );
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
    this.fetch();
  }

  private getApplications(): string[] {
    return Object.keys(this.screenshotsByApplication).filter(
      (app: string) => this.getScreenshotsForApplication(app).length > 0
    );
  }

  private getScreenshotsForApplication(app: string): Screenshot[] {
    // Performs a VSCode-style fuzzy search. See https://stackoverflow.com/a/69860312.
    const regex = new RegExp(
      this.filter === '' ? '.*' : `${this.filter.split('').join('+?.*')}`,
      'i'
    );

    return this.screenshotsByApplication[app].filter((screenshot: Screenshot) =>
      regex.test(`${app}_${screenshot.test_name}`)
    );
  }

  private onFilterInput(e: Event) {
    const element = e.target as HTMLInputElement;
    this.filter = element.value;
    this.stateChanged();
    this._render();
  }

  private onClearClick() {
    this.querySelector<HTMLInputElement>('.filter input')!.value = '';
    this.filter = '';
    this.stateChanged();
    this._render();
  }

  private fetch() {
    fetch('/rpc/get-screenshots', { method: 'GET' })
      .then(jsonOrThrow)
      .then((r: GetScreenshotsRPCResponse) => {
        this.screenshotsByApplication = r.screenshots_by_application;
        this.loaded = true;
        this._render();
        this.dispatchEvent(new CustomEvent('loaded', { bubbles: true })); // Needed for testing.
      });
  }
}

define('screenshots-viewer-sk', ScreenshotsViewerSk);
