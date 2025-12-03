/**
 * @module modules/anomaly-playground-sk
 * @description <h2><code>anomaly-playground-sk</code></h2>
 *
 * A page for experimenting with anomaly detection algorithms.
 */
import { html } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { ExploreSimpleSk } from '../explore-simple-sk/explore-simple-sk';
import {
  Anomaly,
  FrameResponse,
  TraceSet,
  Trace,
  DataFrame,
  CommitNumber,
  TimestampSeconds,
  ReadOnlyParamSet,
  StepDetection,
  Direction,
  QueryConfig,
  FrameRequest,
} from '../json';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { stateReflector } from '../../../infra-sk/modules/stateReflector';
import { HintableObject } from '../../../infra-sk/modules/hintable';
import { errorMessage } from '../errorMessage';

import '@material/web/button/filled-button';
import '@material/web/textfield/outlined-text-field';
import '@material/web/select/outlined-select';
import '@material/web/select/select-option';
import '@material/web/checkbox/checkbox';

import '../explore-simple-sk';
import '../dataframe/dataframe_context';

const StepDetectionValues = ['', 'absolute', 'const', 'percent', 'cohen', 'mannwhitneyu'] as const;

export class AnomalyPlaygroundSk extends ElementSk {
  private exploreSimpleSkFactory = () => new ExploreSimpleSk(false);

  private static template = (ele: AnomalyPlaygroundSk) => html`
    <dataframe-repository-sk>
      <div class="playground-controls">
        <md-outlined-text-field
          label="Comma-separated values"
          id="trace-input"
          textarea
          rows="10"
          value="1,1,1,1,1,1,1,1,1,1,5,5,5,5,5,5,5,5,5,5,1,1,1,1,1,1,1,1,1,1">
        </md-outlined-text-field>
        <div class="param-controls">
          <md-outlined-select id="algorithm-selector" label="Algorithm">
            ${StepDetectionValues.map(
              (algo) => html`<md-select-option value=${algo}>${algo}</md-select-option>`
            )}
          </md-outlined-select>
          <md-outlined-select id="direction-selector" label="Regression Direction" value="UP">
            <md-select-option value="UP">UP</md-select-option>
            <md-select-option value="DOWN">DOWN</md-select-option>
          </md-outlined-select>
          <md-outlined-text-field
            id="radius-input"
            label="Radius"
            type="number"
            value="5"></md-outlined-text-field>
          <md-outlined-text-field
            id="threshold-input"
            label="Threshold"
            type="number"
            value="3.0"></md-outlined-text-field>
          <label style="display: flex; align-items: center;">
            <md-checkbox id="group-anomalies" checked></md-checkbox>
            Group Anomalies
          </label>
        </div>
        <div class="buttons">
          <md-filled-button @click=${ele.detect} ?disabled=${ele.detectButtonDisabled}
            >Detect</md-filled-button
          >
        </div>
      </div>
      <div id="graph-container"></div>
    </dataframe-repository-sk>
  `;

  private traceInput: HTMLTextAreaElement | null = null;

  private algorithmSelector: HTMLSelectElement | null = null;

  private directionSelector: HTMLSelectElement | null = null;

  private radiusInput: HTMLInputElement | null = null;

  private thresholdInput: HTMLInputElement | null = null;

  private groupAnomaliesCheckbox: HTMLInputElement | null = null;

  private exploreSimpleSk: ExploreSimpleSk | null = null;

  private graphContainer: HTMLDivElement | null = null;

  private dataframe: DataFrame | null = null;

  private detectButtonDisabled: boolean = true;

  private stateHasChanged: () => void = () => {};

  constructor() {
    super(AnomalyPlaygroundSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
    this.traceInput = this.querySelector('#trace-input');
    this.algorithmSelector = this.querySelector('#algorithm-selector');
    this.directionSelector = this.querySelector('#direction-selector');
    this.radiusInput = this.querySelector('#radius-input');
    this.thresholdInput = this.querySelector('#threshold-input');
    this.groupAnomaliesCheckbox = this.querySelector('#group-anomalies');
    this.graphContainer = this.querySelector('#graph-container');

    // Clear any existing graph.
    this.graphContainer!.innerHTML = '';

    // Create, configure, and append the graph element.
    const defaults: QueryConfig = {
      default_param_selections: null,
      default_url_values: null,
      include_params: null,
    };
    this.exploreSimpleSk = this.exploreSimpleSkFactory();
    this.exploreSimpleSk.id = 'explore';
    this.exploreSimpleSk.defaults = defaults;
    this.exploreSimpleSk.openQueryByDefault = false;
    this.exploreSimpleSk.navOpen = false;
    this.exploreSimpleSk.disablePointLinks = true;
    this.exploreSimpleSk.showHeader = false;
    this.exploreSimpleSk.state.hide_paramset = true;
    this.graphContainer!.appendChild(this.exploreSimpleSk);

    this.stateHasChanged = stateReflector(
      () => ({
        trace: this.traceInput?.value || '',
        algo: this.algorithmSelector?.value || '',
        direction: this.directionSelector?.value || 'UP',
        radius: this.radiusInput ? parseFloat(this.radiusInput.value) : 5,
        threshold: this.thresholdInput ? parseFloat(this.thresholdInput.value) : 3.0,
        group: this.groupAnomaliesCheckbox?.checked ?? true,
      }),
      (state: HintableObject) => {
        if (!this.traceInput) return;
        this.traceInput.value = state.trace as string;
        this.algorithmSelector!.value = state.algo as string;
        this.directionSelector!.value = state.direction as string;
        this.radiusInput!.value = String(state.radius);
        this.thresholdInput!.value = String(state.threshold);
        this.groupAnomaliesCheckbox!.checked = state.group as boolean;

        this.checkInputs();
        this.plot();
      }
    );

    this.traceInput!.addEventListener('input', () => {
      this.exploreSimpleSk!.clearAnomalyMap();
      this.plot();
      this.stateHasChanged();
    });

    this.addEventListener('input', () => {
      this.checkInputs();
      this.stateHasChanged();
    });
    this.addEventListener('change', () => {
      this.checkInputs();
      this.stateHasChanged();
    });
    this.checkInputs();
  }

  private checkInputs() {
    if (
      !this.algorithmSelector ||
      !this.directionSelector ||
      !this.radiusInput ||
      !this.thresholdInput
    ) {
      return;
    }
    const algo = this.algorithmSelector.value;
    const direction = this.directionSelector.value;
    const radius = parseFloat(this.radiusInput.value);
    const threshold = parseFloat(this.thresholdInput.value);

    const newState =
      !algo || !direction || isNaN(radius) || radius <= 0 || isNaN(threshold) || threshold <= 0;
    if (this.detectButtonDisabled !== newState) {
      this.detectButtonDisabled = newState;
      this._render();
    }
  }

  private getTraceData(): number[] {
    if (!this.traceInput) {
      return [];
    }
    return this.traceInput.value
      .split(',')
      .map((s) => s.trim())
      .filter((s) => s !== '')
      .map((s) => parseFloat(s));
  }

  private plot() {
    if (!this.exploreSimpleSk) {
      errorMessage('Graph element not ready.');
      return;
    }
    const trace = this.getTraceData();
    if (trace.some(isNaN)) {
      errorMessage('Invalid trace data. Please enter comma-separated numbers.');
      return;
    }

    const traceset = TraceSet({
      [',name=playground,']: trace as Trace,
    });
    this.dataframe = {
      traceset: traceset,
      header: [],
      paramset: ReadOnlyParamSet({}),
      skip: 0,
      traceMetadata: [],
    };
    for (let i = 0; i < trace.length; i++) {
      this.dataframe.header?.push({
        offset: CommitNumber(i),
        timestamp: TimestampSeconds(i),
        author: '',
        hash: '',
        message: '',
        url: '',
      });
    }

    const request: FrameRequest = {
      begin: 0,
      end: trace.length,
      num_commits: trace.length,
      request_type: 0,
      formulas: [],
      queries: ['name=playground'],
      keys: '',
      tz: '',
      pivot: null,
      disable_filter_parent_traces: false,
    };

    const frameResponse: FrameResponse = {
      dataframe: this.dataframe,
      skps: [],
      msg: '',
      display_mode: 'display_plot',
      anomalymap: {},
    };

    this.exploreSimpleSk.UpdateWithFrameResponse(frameResponse, request, false);
  }

  private async detect() {
    if (!this.exploreSimpleSk) {
      errorMessage('Graph element not ready.');
      return;
    }
    if (!this.dataframe) {
      errorMessage('Dataframe not initialized. Please Plot first.');
      return;
    }
    const trace = this.getTraceData();
    if (trace.some(isNaN)) {
      errorMessage('Invalid trace data. Please enter comma-separated numbers.');
      return;
    }

    const radius = parseInt(this.radiusInput!.value);
    const threshold = parseFloat(this.thresholdInput!.value);
    const algorithm = this.algorithmSelector!.value as StepDetection;
    const direction = this.directionSelector!.value as Direction;
    const groupAnomalies = this.groupAnomaliesCheckbox!.checked;

    const request: FrameRequest = {
      begin: 0,
      end: trace.length,
      num_commits: trace.length,
      request_type: 0,
      formulas: [],
      queries: ['name=playground'],
      keys: '',
      tz: '',
      pivot: null,
      disable_filter_parent_traces: false,
    };

    // Close any existing tooltips.
    this.exploreSimpleSk.closeTooltip();

    // Clear existing anomalies first.
    this.exploreSimpleSk.clearAnomalyMap();

    try {
      const resp = await fetch('/_/playground/anomaly/v1/detect', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          trace: trace,
          radius: radius,
          threshold: threshold,
          algorithm: algorithm,
          direction: direction,
          group_anomalies: groupAnomalies,
        }),
      });

      const json = await jsonOrThrow(resp);
      const anomalies = json.anomalies as Anomaly[];

      if (anomalies.length === 0) {
        return;
      }

      const anomalymap: { [key: string]: { [key: number]: Anomaly } } = {};
      anomalymap[',name=playground,'] = {};
      anomalies.forEach((anomaly) => {
        const delta = anomaly.median_after_anomaly - anomaly.median_before_anomaly;
        if (direction === 'UP') {
          anomaly.is_improvement = delta < 0;
        } else if (direction === 'DOWN') {
          anomaly.is_improvement = delta > 0;
        } else {
          anomaly.is_improvement = false;
        }
        anomalymap[',name=playground,'][anomaly.start_revision] = anomaly;
      });

      const frameResponse: FrameResponse = {
        dataframe: this.dataframe,
        skps: [],
        msg: '',
        display_mode: 'display_plot',
        anomalymap: anomalymap,
      };

      this.exploreSimpleSk.UpdateWithFrameResponse(frameResponse, request, false);
    } catch (err) {
      errorMessage('Failed to detect anomalies.');
      console.error(err);
    }
  }
}

define('anomaly-playground-sk', AnomalyPlaygroundSk);
