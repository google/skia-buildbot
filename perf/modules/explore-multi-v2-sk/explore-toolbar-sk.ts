import { LitElement, css, html } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import '../window/window';

@customElement('explore-toolbar-sk')
export class ExploreToolbarSk extends LitElement {
  @property({ type: Number }) tracePage = 0;

  @property({ type: Number }) totalMatchedPages = 1;

  @property({ type: Boolean }) showAllTraces = false;

  @property({ type: String }) selectedSubrepo = 'none';

  @property({ type: Array }) availableSubrepos: string[] = [];

  @property({ type: String }) normalizeCentre = 'none';

  @property({ type: Boolean }) smooth = false;

  @property({ type: Boolean }) showDots = true;

  @property({ type: Boolean }) showSparklines = false;

  @property({ type: Boolean }) onlyRegressions = false;

  @property({ type: Boolean }) splitAll = false;

  @property({ type: Array }) availableSplitKeys: string[] = [];

  @property({ type: Array }) activeSplitKeys: string[] = [];

  @property({ type: Number }) pageSize = 10;

  @property({ type: Boolean }) showRegressions = true;

  @property({ type: Boolean }) tooltipDiffs = false;

  @property({ type: Boolean }) dateMode = false;

  @property({ type: Boolean }) evenXAxisSpacing = false;

  @property({ type: Boolean }) showZero = false;

  @property({ type: String }) transformPreset = 'none';

  @property({ type: String }) hoverMode = 'original';

  @property({ type: Number }) smoothingRadius = 20;

  @property({ type: Number }) edgeDetectionFactor = 1.0;

  @property({ type: Number }) edgeLookahead = 3;

  @property({ type: Boolean }) openAdvanced = true;

  static styles = css`
    :host {
      display: block;
      margin: 12px 0;
    }

    .toolbar {
      display: flex;
      flex-direction: column;
      gap: 8px;
      font-size: 12px;
    }

    .toolbar-section {
      display: flex;
      flex-wrap: wrap;
      gap: 12px;
      align-items: center;
      padding: 4px 0;
    }

    .toolbar-group {
      display: flex;
      align-items: center;
      gap: 6px;
    }

    .toolbar label {
      color: var(--on-surface);
      display: flex;
      align-items: center;
      gap: 4px;
      cursor: pointer;
      font-weight: 500;
    }

    .custom-btn {
      background: var(--surface);
      color: var(--on-surface);
      border: 1px solid var(--outline);
      padding: 4px 8px;
      border-radius: 4px;
      font-weight: 500;
      font-size: 12px;
      cursor: pointer;
      transition: all 0.2s ease;
    }

    .custom-btn:hover {
      background: var(--outline);
      border-color: var(--primary);
    }

    .custom-btn:disabled {
      opacity: 0.5;
      cursor: not-allowed;
    }

    .custom-select {
      background: var(--surface);
      color: var(--on-surface);
      border: 1px solid var(--outline);
      border-radius: 4px;
      padding: 3px 8px;
      font-size: 12px;
      cursor: pointer;
      outline: none;
      transition: all 0.2s;
    }

    .custom-select:focus {
      border-color: var(--primary);
    }

    .custom-checkbox {
      position: relative;
      display: flex;
      align-items: center;
      cursor: pointer;
      user-select: none;
    }

    .custom-checkbox input {
      position: absolute;
      opacity: 0;
      cursor: pointer;
      height: 0;
      width: 0;
    }

    .checkmark {
      height: 14px;
      width: 14px;
      background-color: var(--surface);
      border: 1px solid var(--outline);
      border-radius: 3px;
      margin-right: 6px;
      transition: all 0.2s;
      display: flex;
      align-items: center;
      justify-content: center;
    }

    .custom-checkbox:hover input ~ .checkmark {
      background-color: var(--outline);
    }

    .custom-checkbox input:checked ~ .checkmark {
      background-color: var(--primary);
      border-color: var(--primary);
    }

    .checkmark::after {
      content: '';
      display: none;
      width: 3px;
      height: 6px;
      border: solid var(--on-primary);
      border-width: 0 2px 2px 0;
      transform: rotate(45deg);
      margin-bottom: 1px;
    }

    .custom-checkbox input:checked ~ .checkmark::after {
      display: block;
    }

    .custom-slider {
      appearance: none;
      width: 60px;
      height: 2px;
      border-radius: 1px;
      background: var(--outline);
      outline: none;
    }

    .custom-slider::-webkit-slider-thumb {
      appearance: none;
      width: 10px;
      height: 10px;
      border-radius: 50%;
      background: var(--primary);
      cursor: pointer;
    }

    .slider-value {
      font-family: monospace;
      font-size: 10px;
      color: var(--on-background);
      min-width: 15px;
      text-align: right;
    }

    .label {
      color: var(--on-surface);
      font-weight: 500;
    }

    details.advanced-options-details {
      border: 1px solid var(--outline);
      border-radius: 6px;
      padding: 6px 12px;
      background: var(--surface);
    }

    summary.advanced-options-summary {
      cursor: pointer;
      font-size: 12px;
      font-weight: 600;
      color: var(--on-surface);
      user-select: none;
      padding: 2px 0;
    }

    details[open] summary.advanced-options-summary {
      margin-bottom: 8px;
    }

    .section-divider {
      border-top: 1px solid var(--outline);
      margin: 4px 0;
    }
  `;

  render() {
    const totalMatchedPages = Math.max(1, this.totalMatchedPages);

    return html`
      <details class="advanced-options-details" ?open=${this.openAdvanced}>
        <summary class="advanced-options-summary">Advanced Options</summary>
        <div class="toolbar">
          <!-- Section 1: Data & Navigation -->
          <div class="toolbar-section">
            <button
              class="custom-btn"
              @click=${() => this._emitChange('showAllTraces', !this.showAllTraces)}>
              ${this.showAllTraces ? 'Show Paged' : 'Show All'}
            </button>

            <button
              class="custom-btn"
              @click=${() => this.dispatchEvent(new CustomEvent('reset-zoom'))}>
              Reset Zoom
            </button>

            <div class="toolbar-group">
              <span class="label">Rollouts:</span>
              <select
                class="custom-select subrepo-select"
                .value=${this.selectedSubrepo}
                @change=${(e: any) => this._emitChange('selectedSubrepo', e.target.value)}>
                <option value="none">None</option>
                ${this.availableSubrepos.map((r) => html`<option value="${r}">${r}</option>`)}
              </select>
            </div>

            <div class="toolbar-group">
              <span class="label">Center:</span>
              <select
                class="custom-select"
                .value=${this.normalizeCentre}
                @change=${(e: any) => this._emitChange('normalizeCentre', e.target.value)}>
                <option value="none">None</option>
                <option value="first">First</option>
                <option value="average">Average</option>
                <option value="median">Median</option>
              </select>
            </div>

            <label class="custom-checkbox date-mode-checkbox">
              <input
                type="checkbox"
                .checked=${this.dateMode}
                @change=${(e: any) => this._emitChange('dateMode', e.target.checked)} />
              <span class="checkmark"></span>
              Date Mode
            </label>

            <label class="custom-checkbox">
              <input
                type="checkbox"
                .checked=${this.evenXAxisSpacing}
                @change=${(e: any) => this._emitChange('evenXAxisSpacing', e.target.checked)} />
              <span class="checkmark"></span>
              Even X-Axis Spacing
            </label>

            <label class="custom-checkbox">
              <input
                type="checkbox"
                .checked=${this.showZero}
                @change=${(e: any) => this._emitChange('showZero', e.target.checked)} />
              <span class="checkmark"></span>
              Show Zero
            </label>

            ${window.perf?.trace_transform
              ? html`
                  <div class="toolbar-group">
                    <span class="label">Transform:</span>
                    <select
                      class="custom-select"
                      .value=${this.transformPreset}
                      @change=${this._handleTransformPresetChange}>
                      <option value="none">None</option>
                      <option value="delta">Delta (X[i] - X[i-1])</option>
                      <option value="rel_delta">Relative Delta ((X[i] - X[i-1]) / X[i-1])</option>
                      <option value="velocity">Velocity (Delta / Commit Delta)</option>
                      <option value="avg3">Moving Average (3pt)</option>
                      <option value="median3">Moving Median (3pt)</option>
                      <option value="stddev10">Moving StdDev (10pt)</option>
                    </select>
                  </div>
                `
              : ''}

            <details
              id="yaxis-splitter"
              class="custom-select yaxis-splitter-details"
              style="position: relative;">
              <summary style="cursor: pointer;">Y-axis Splitter</summary>
              <div
                style="position: absolute; background: var(--surface, #1e293b); border: 1px solid var(--outline, rgba(255, 255, 255, 0.1)); padding: 8px; border-radius: 4px; z-index: 10; display: flex; flex-direction: column; gap: 4px; min-width: 150px;">
                ${this.availableSplitKeys.length === 0
                  ? html`<span style="color: var(--on-surface, #64748b);">No options</span>`
                  : this.availableSplitKeys.map(
                      (key) => html`
                        <label class="custom-checkbox">
                          <input
                            type="checkbox"
                            .checked=${this.activeSplitKeys.includes(key)}
                            @change=${() => {
                              this.dispatchEvent(
                                new CustomEvent('split', {
                                  detail: { key },
                                  bubbles: true,
                                  composed: true,
                                })
                              );
                            }} />
                          <span class="checkmark"></span>
                          ${key}
                        </label>
                      `
                    )}
              </div>
            </details>

            <div class="toolbar-group">
              <span class="label">Traces/Page:</span>
              <input
                class="custom-select"
                type="number"
                min="1"
                max="500"
                .value=${this.pageSize.toString()}
                @change=${(e: any) => this._emitChange('pageSize', parseInt(e.target.value, 10))}
                style="width: 50px;" />
            </div>

            <!-- Pagination -->
            <div class="toolbar-group" style="margin-left: auto;">
              <button
                class="custom-btn"
                style="padding: 2px 6px;"
                @click=${() => this._emitChange('tracePage', this.tracePage - 1)}
                ?disabled=${this.tracePage === 0}>
                &lt;
              </button>
              <span style="font-size: 11px; min-width: 50px; text-align: center;">
                ${this.tracePage + 1} of ${totalMatchedPages}
              </span>
              <button
                class="custom-btn"
                style="padding: 2px 6px;"
                @click=${() => this._emitChange('tracePage', this.tracePage + 1)}
                ?disabled=${this.tracePage >= totalMatchedPages - 1}>
                &gt;
              </button>
            </div>
          </div>

          <div class="section-divider"></div>

          <!-- Section 2: Visualization Options -->
          <div class="toolbar-section">
            <label class="custom-checkbox smooth-checkbox">
              <input
                type="checkbox"
                .checked=${this.smooth}
                @change=${(e: any) => {
                  this._emitChange('smooth', e.target.checked);
                  this._emitChange('hoverMode', e.target.checked ? 'both' : 'original');
                }} />
              <span class="checkmark"></span>
              Smooth
            </label>

            <label class="custom-checkbox">
              <input
                type="checkbox"
                .checked=${this.showDots}
                @change=${(e: any) => this._emitChange('showDots', e.target.checked)} />
              <span class="checkmark"></span>
              Dots
            </label>

            <label class="custom-checkbox">
              <input
                type="checkbox"
                .checked=${this.showSparklines}
                @change=${(e: any) => this._emitChange('showSparklines', e.target.checked)} />
              <span class="checkmark"></span>
              Sparklines
            </label>

            <label class="custom-checkbox">
              <input
                type="checkbox"
                .checked=${this.showRegressions}
                @change=${(e: any) => this._emitChange('showRegressions', e.target.checked)} />
              <span class="checkmark"></span>
              Show Regressions
            </label>

            <label class="custom-checkbox">
              <input
                type="checkbox"
                .checked=${this.tooltipDiffs}
                @change=${(e: any) => this._emitChange('tooltipDiffs', e.target.checked)} />
              <span class="checkmark"></span>
              Tooltip Diffs
            </label>

            ${window.perf?.enable_only_regressions_option
              ? html`
                  <label class="custom-checkbox only-regressions-checkbox">
                    <input
                      type="checkbox"
                      .checked=${this.onlyRegressions}
                      @change=${(e: any) =>
                        this._emitChange('onlyRegressions', e.target.checked)} />
                    <span class="checkmark"></span>
                    Only Regressions
                  </label>
                `
              : ''}
            ${window.perf?.enable_split_all_option
              ? html`
                  <label class="custom-checkbox split-all-checkbox">
                    <input
                      type="checkbox"
                      .checked=${this.splitAll}
                      @change=${(e: any) => this._emitChange('splitAll', e.target.checked)} />
                    <span class="checkmark"></span>
                    Split by All Keys
                  </label>
                `
              : ''}

            <div class="toolbar-group">
              <span class="label">Hover:</span>
              <select
                class="custom-select"
                .value=${this.hoverMode}
                @change=${(e: any) => this._emitChange('hoverMode', e.target.value)}>
                <option value="original">Original</option>
                <option value="smoothed">Smoothed</option>
                <option value="both">Both</option>
              </select>
            </div>

            <!-- Sliders (Inline if visible) -->
            ${this.hoverMode !== 'original'
              ? html`
                  <div class="toolbar-group">
                    <span class="label">Rad:</span>
                    <input
                      class="custom-slider rad-slider"
                      type="range"
                      min="1"
                      max="100"
                      .value=${this.smoothingRadius.toString()}
                      @input=${(e: any) =>
                        this._emitChange('smoothingRadius', parseInt(e.target.value, 10))} />
                    <span class="slider-value">${this.smoothingRadius}</span>
                  </div>

                  <div class="toolbar-group">
                    <span class="label">Edge:</span>
                    <input
                      class="custom-slider"
                      type="range"
                      min="0.1"
                      max="1"
                      step="0.05"
                      .value=${this.edgeDetectionFactor.toString()}
                      @input=${(e: any) =>
                        this._emitChange('edgeDetectionFactor', parseFloat(e.target.value))} />
                    <span class="slider-value">${this.edgeDetectionFactor.toFixed(2)}</span>
                  </div>

                  <div class="toolbar-group">
                    <span class="label">Outlier:</span>
                    <input
                      class="custom-slider"
                      type="range"
                      min="0"
                      max="5"
                      step="1"
                      .value=${this.edgeLookahead.toString()}
                      @input=${(e: any) =>
                        this._emitChange('edgeLookahead', parseInt(e.target.value, 10))} />
                    <span class="slider-value">${this.edgeLookahead}</span>
                  </div>
                `
              : ''}
          </div>
        </div>
      </details>
    `;
  }

  private _emitChange(name: string, value: any) {
    this.dispatchEvent(
      new CustomEvent('control-change', {
        detail: { name, value },
        bubbles: true,
        composed: true,
      })
    );
  }

  private _handleTransformPresetChange(e: any) {
    const val = e.target.value;
    this._emitChange('transformPreset', val);
  }
}
