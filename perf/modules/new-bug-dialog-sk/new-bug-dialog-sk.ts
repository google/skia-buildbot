/**
 * @module modules/new-bug-dialog-sk
 * @description <h2><code>new-bug-dialog-sk</code></h2>
 *
 * Dialog to show when user wants to create a new bug on an untriaged anomaly.
 *
 * Takes the following inputs:
 *   - Title
 *   - Description
 *   - Labels
 *   - Components
 *   - Owner
 *   - CC's
 *
 * Once a validated user submits this dialog, there'll be an attempt to create a new
 * Buganizer bug. If succesful, user is re-directed to the bug created. If unsuccesful,
 * an error message toast will appear.
 *
 */

import { html, TemplateResult, LitElement, PropertyValues } from 'lit';
import { customElement, property, state, query } from 'lit/decorators.js';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { Anomaly } from '../json';

import { errorMessage } from '../../../elements-sk/modules/errorMessage';
import { LoggedIn } from '../../../infra-sk/modules/alogin-sk/alogin-sk';
import { Status } from '../../../infra-sk/modules/json';

import '../../../elements-sk/modules/icons/close-icon-sk';
import '../../../elements-sk/modules/spinner-sk';

@customElement('new-bug-dialog-sk')
export class NewBugDialogSk extends LitElement {
  @query('#new-bug-dialog')
  private _dialog!: HTMLDialogElement;

  @query('#loading-popup')
  private _loadingPopup!: HTMLDialogElement;

  @query('#title')
  private _titleInput!: HTMLInputElement;

  @query('#description')
  private _descriptionInput!: HTMLTextAreaElement;

  @query('#assignee')
  private _assigneeInput!: HTMLInputElement;

  @query('#ccs')
  private _ccsInput!: HTMLInputElement;

  private _bugUrl: string = '';

  @query('#new-bug-form')
  private _form!: HTMLFormElement;

  @property({ attribute: false })
  anomalies: Anomaly[] = [];

  @property({ attribute: false })
  traceNames: string[] = [];

  @state()
  private _user: string = '';

  private INFINITY_PERCENT_CHANGE: string = 'zero-to-nonzero';

  private isDragging = false;

  private xOffset = 0;

  private yOffset = 0;

  createRenderRoot() {
    return this;
  }

  render() {
    return html`
    <dialog id="new-bug-dialog"
    @mousedown=${this.onMousedown}
    @mousemove=${this.onMouseMove}
    @mouseup=${this.onMouseUp}>
      <h2>File New Bug</h2>
      <button id="closeIcon" @click=${this.closeDialog}>
        <close-icon-sk></close-icon-sk>
      </button>
      <form id="new-bug-form" @submit=${this.onFormSubmit}>
        <label for="title">Title</label>
        <input
          id="title"
          type="text"
          required
          value=${this.getBugTitle()}>
        </input>
        <label for="description">Description</label>
        <textarea id="description" rows="10"></textarea>
        ${this.hasLabels() ? html`<h3>Labels</h3>` : ''}
        ${this.getLabelCheckboxes()}
        <h3>Component</h3>
        ${this.getComponentRadios()}
        <label for="assignee">Assignee</label>
        <input
          id="assignee"
          type="text"
          >
        </input>
        <label for="ccs">CC's (comma-separated e-mails)</label>
        <input
          id="ccs"
          type="text"
          value=${this._user}>
        </input>
      </form>
    </dialog>
    <dialog id="loading-popup">
      <p>New bug creating in process. Waiting...</p>
    </dialog>
  `;
  }

  private onFormSubmit = (e: Event) => {
    e.preventDefault();
    this.fileNewBug();
  };

  connectedCallback() {
    super.connectedCallback();
    document.addEventListener('mousedown', this.onMousedown);
  }

  disconnectedCallback() {
    super.disconnectedCallback();
    document.removeEventListener('mousedown', this.onMousedown);
  }

  /**
   * Checks if any of the anomalies have labels.
   */
  hasLabels(): boolean {
    return this.anomalies.some((anomaly) => anomaly.bug_labels && anomaly.bug_labels!.length > 0);
  }

  /**
   * Gather the labels for all anomalies and present unique checkboxes in the dialog.
   *
   * These will all be checked by default.
   */
  getLabelCheckboxes(): TemplateResult[] {
    const checkboxes: TemplateResult[] = [];

    // Use a Set to keep track of unique labels and not show duplicates.
    const uniqueLabels = new Set<string>();

    // Use counter to set id of each checkbox to be unique.
    let counter = 0;
    this.anomalies.forEach((anomaly) => {
      anomaly.bug_labels?.forEach((label) => {
        if (!uniqueLabels.has(label)) {
          uniqueLabels.add(label);
          checkboxes.push(
            html`
              <div>
                <input
                  type="checkbox"
                  class="buglabel"
                  id=${`label-checkbox-${counter}`}
                  checked
                  value="${label}">
                </input>
                <label for=${`label-checkbox-${counter}`}>${label}</label>
              </div>
            `
          );
          counter += 1;
        }
      });
    });
    return checkboxes;
  }

  /**
   * Gather the components for all anomalies and present unique radios in the dialog.
   *
   * The first radio is always selected. A radio selection is required.
   */
  getComponentRadios(): TemplateResult[] {
    const radios: TemplateResult[] = [];

    // Use a Set to keep track of unique components
    const uniqueComponents = new Set<string>();

    // Check if this is the first radio created to mark it as checked.
    let isFirst = true;
    let counter = 0;

    this.anomalies.forEach((anomaly) => {
      const component = anomaly.bug_component;
      // Only add the radio button if the component is not already in the Set
      if (!uniqueComponents.has(component)) {
        uniqueComponents.add(component);
        const radioId = `component-radio-${counter}`;
        radios.push(
          html`
            <div>
              <input
                type="radio"
                required
                id=${radioId}
                name="component"
                ?checked=${isFirst}
                value="${component}">
              </input>
              <label for=${radioId}>${component}</label>
            </div>
          `
        );
        isFirst = false;
        counter++;
      }
    });
    return radios;
  }

  /**
   * Use anomaly medians to calculate the percent change.
   */
  getPercentChangeForAnomaly(anomaly: Anomaly): number {
    if (anomaly.median_before_anomaly === 0.0) {
      return Number.MAX_VALUE;
    }

    const difference = anomaly.median_after_anomaly - anomaly.median_before_anomaly;
    return (100 * difference) / anomaly.median_before_anomaly;
  }

  /**
   * Gets the percent change of an anomaly and makes it readable.
   *
   * If percentChange is infinite or undefined, set it to this.INFINITY_PERCENT_CHANGE.
   */
  getDisplayPercentChanged(anomaly: Anomaly): string {
    if (Math.abs(this.getPercentChangeForAnomaly(anomaly)) === Number.MAX_VALUE) {
      return this.INFINITY_PERCENT_CHANGE;
    }
    return `${Math.abs(this.getPercentChangeForAnomaly(anomaly)).toFixed(1)}%`;
  }

  /**
   * Mimics getSuiteNameForAlert function in Legacy Chromeperf UI.
   *
   * There are special cases for displaying the names of benchmarks.
   */
  getSuiteNameForAlert(anomaly: Anomaly) {
    const test_path_parts = anomaly.test_path.split('/');
    const testsuite = test_path_parts[2];
    const SUITES_WITH_SUBTEST_ENTRY = ['rendering.desktop', 'rendering.mobile', 'v8'];
    if (!SUITES_WITH_SUBTEST_ENTRY.includes(testsuite)) {
      return testsuite;
    }
    return `${testsuite}/${test_path_parts[3]}`;
  }

  /**
   * Generates a Bug Title based on anomaly data.
   *
   * This tries to mimic the getBugTitleForAnomaly function in LegacyChromeperf UI.
   */
  getBugTitle() {
    if (this.anomalies.length === 0) {
      return '';
    }

    let type = 'improvement';
    let percentMin = Infinity;
    let percentMax = -Infinity;
    let maxRegressionFound = false;
    let startRev = Infinity;
    let endRev = -Infinity;

    for (let i = 0; i < this.anomalies.length; i++) {
      const anomaly = this.anomalies[i];
      if (!anomaly.is_improvement) {
        type = 'regression';
      }
      let percent = Infinity;
      if (
        this.getDisplayPercentChanged(anomaly) === this.INFINITY_PERCENT_CHANGE &&
        !maxRegressionFound
      ) {
        maxRegressionFound = true;
      } else {
        percent = Math.abs(parseFloat(this.getDisplayPercentChanged(anomaly)));
      }
      if (percent < percentMin) {
        percentMin = percent;
      }
      if (percent > percentMax) {
        percentMax = percent;
      }
      if (anomaly.start_revision < startRev) {
        startRev = anomaly.start_revision;
      }
      if (anomaly.end_revision > endRev) {
        endRev = anomaly.end_revision;
      }
    }

    // Round the percentages to 1 decimal place.
    percentMin = Math.round(percentMin * 10) / 10;
    percentMax = Math.round(percentMax * 10) / 10;

    let minMax = `${percentMin}%-${percentMax}%`;
    if (maxRegressionFound) {
      if (percentMin === Infinity) {
        // Both percentMin and percentMax were at Infinity.
        // Record a huge (TM) regression.
        minMax = `A ${this.INFINITY_PERCENT_CHANGE}`;
      } else {
        // Regressions ranged from Infinity to some other lower percentage.
        minMax = `A ${this.INFINITY_PERCENT_CHANGE} to ${percentMin}%`;
      }
    } else if (percentMin === percentMax) {
      minMax = `${percentMin}%`;
    }

    const suiteTitle = this.getSuiteNameForAlert(this.anomalies[0]);
    const summary = '{{range}} {{type}} in {{suite}} at {{start}}:{{end}}'
      .replace('{{range}}', minMax)
      .replace('{{type}}', type)
      .replace('{{suite}}', suiteTitle)
      .replace('{{start}}', startRev.toString())
      .replace('{{end}}', endRev.toString());

    return summary;
  }

  /**
   * Reads the form inputs and attempts to file a new bug.
   *
   * CCs value is transformed from a comma-separated string to a list.
   * Upon success, we redirect the user in a new tab to the new bug.
   * Upon failure, we keep the dialog open and show an error toast.
   */
  async fileNewBug(): Promise<void> {
    await this.updateComplete;
    this._loadingPopup.showModal();

    // Extract title.
    const title = this._titleInput;

    // Extract description.
    const description = this._descriptionInput;

    // Extract assignee
    const assignee = this._assigneeInput;

    //  Extract CCs
    const ccs_value = this._ccsInput.value;
    const ccs = ccs_value.split(',').map((s: string) => s.trim());

    // Extract labels.
    const label_fields = this.querySelectorAll('input.buglabel');
    const labels: string[] = [];
    label_fields.forEach((field) => {
      if ((field as HTMLInputElement).checked) {
        labels.push((field as HTMLInputElement).value);
      }
    });

    // Extract component.
    const component_fields = this.querySelectorAll('input[name=component]');
    let component = '';
    component_fields.forEach((field) => {
      if ((field as HTMLInputElement).checked) {
        component = (field as HTMLInputElement).value;
      }
    });

    const keys: string[] = this.anomalies.map((a) => a.id);

    const body = {
      title: title.value,
      description: description.value,
      assignee: assignee.value,
      ccs: ccs,
      labels: labels,
      component: component,
      keys: keys,
      trace_names: this.traceNames,
    };

    fetch('/_/triage/file_bug', {
      method: 'POST',
      body: JSON.stringify(body),
      headers: {
        'Content-Type': 'application/json',
      },
    })
      .then(jsonOrThrow)
      .then((json) => {
        this._loadingPopup.close();
        this.closeDialog();

        // Open the bug page in new window.
        this._bugUrl = `https://issues.chromium.org/issues/${json.bug_id}`;
        window.open(this._bugUrl, '_blank');

        // Update anomalies to reflected new Bug Id.
        for (let i = 0; i < this.anomalies.length; i++) {
          this.anomalies[i].bug_id = json.bug_id;
        }

        // Let explore-simple-sk and chart-tooltip-sk that anomalies have changed and we need to re-render.
        this.dispatchEvent(
          new CustomEvent('anomaly-changed', {
            bubbles: true,
            composed: true,
            detail: {
              traceNames: this.traceNames,
              anomalies: this.anomalies,
              bugId: json.bug_id,
            },
          })
        );
      })
      .catch(() => {
        this._loadingPopup.close();
        this.closeDialog();
        errorMessage(
          'File new bug request failed due to an internal server error. Please try again.'
        );
      });
  }

  setAnomalies(anomalies: Anomaly[], traceNames: string[]): void {
    this.anomalies = anomalies;
    this.traceNames = traceNames;
  }

  updated(changedProperties: PropertyValues) {
    if (changedProperties.has('anomalies') || changedProperties.has('traceNames')) {
      if (this._form) {
        this._form.reset();
      }
    }
  }

  async open(): Promise<void> {
    await this.updateComplete;
    // If user is logged in, automatically add the e-mail to CC.
    LoggedIn().then((loginstatus: Status) => {
      this._user = loginstatus.email;
    });
    this._dialog!.showModal();
  }

  closeDialog = (): void => {
    this._dialog!.close();
  };

  get opened() {
    return this._dialog ? this._dialog.open : false;
  }

  private onMouseUp = (e: MouseEvent) => {
    e.preventDefault();
    this.isDragging = false;
  };

  private onMouseMove = (e: MouseEvent) => {
    if (!this.isDragging) {
      return;
    }
    e.preventDefault();
    const widthBoundary = window.innerWidth;
    const heightBoundary = window.innerHeight;
    // Calculate new left offset and right offset
    const newLeft = e.clientX - this.xOffset;
    const newTop = e.clientY - this.yOffset;
    if (
      widthBoundary > newLeft &&
      widthBoundary > newLeft + Number(this._dialog!.style.width) &&
      heightBoundary > newTop &&
      heightBoundary > newTop + Number(this._dialog?.style.height)
    ) {
      this._dialog!.style.left = `${newLeft}px`;
      this._dialog!.style.top = `${newTop}px`;
    }
  };

  private onMousedown = (e: MouseEvent) => {
    if (e.target !== this._dialog) {
      return;
    }
    document.addEventListener('mousemove', this.onMouseMove);
    document.addEventListener('mouseup', this.onMouseUp);
    this.isDragging = true;
    // Calculate the offset of the mouse click relative to the dialog's top-left
    this.xOffset = e.offsetX;
    this.yOffset = e.offsetY;
  };
}
