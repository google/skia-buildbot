import { html, LitElement } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import { AnomalyGroupingConfig, RevisionGroupingMode, GroupingCriteria } from './grouping';

@customElement('anomalies-grouping-settings-sk')
export class AnomaliesGroupingSettingsSk extends LitElement {
  @property({ attribute: false })
  config!: AnomalyGroupingConfig;

  @property({ type: String })
  uniqueId: string = '';

  createRenderRoot() {
    return this;
  }

  render() {
    if (!this.config) {
      return html``;
    }
    const safeId = this.uniqueId || 'default';
    return html`
      <details class="grouping-settings">
        <summary>Grouping Settings</summary>
        <div class="grouping-settings-panel">
          <div class="grouping-setting-group">
            <label class="grouping-setting-label" for="revision-mode-select-${safeId}"
              >Commit Range Strategy</label
            >
            <select
              id="revision-mode-select-${safeId}"
              @change=${(e: Event) => this.onRevisionModeChange(e)}>
              <option value="OVERLAPPING" ?selected=${this.config.revisionMode === 'OVERLAPPING'}>
                Overlapping Ranges
              </option>
              <option value="EXACT" ?selected=${this.config.revisionMode === 'EXACT'}>
                Exact Range Only
              </option>
              <option value="ANY" ?selected=${this.config.revisionMode === 'ANY'}>
                Ignore Range (Group All)
              </option>
            </select>
          </div>

          <div class="grouping-setting-group">
            <label class="grouping-setting-label">Single Anomalies Strategy</label>
            <div class="checkbox-container">
              <label title="If unchecked, single anomalies will not be forced into groups">
                <input
                  type="checkbox"
                  ?checked=${this.config.groupSingles}
                  @change=${(e: Event) => this.onGroupSinglesChange(e)} />
                Group remaining single anomalies by selected criteria (may lead to grouping of
                unrelated anomalies!)
              </label>
            </div>
          </div>

          <div class="grouping-setting-group">
            <label class="grouping-setting-label">Split Groups By</label>
            <div class="checkbox-container">
              <label>
                <input
                  type="checkbox"
                  value="BENCHMARK"
                  ?checked=${this.config.groupBy.has('BENCHMARK')}
                  @change=${(e: Event) => this.onGroupByChange(e, 'BENCHMARK')} />
                Benchmark
              </label>
              <label>
                <input
                  type="checkbox"
                  value="BOT"
                  ?checked=${this.config.groupBy.has('BOT')}
                  @change=${(e: Event) => this.onGroupByChange(e, 'BOT')} />
                Bot
              </label>
              <label>
                <input
                  type="checkbox"
                  value="TEST"
                  ?checked=${this.config.groupBy.has('TEST')}
                  @change=${(e: Event) => this.onGroupByChange(e, 'TEST')} />
                Test (without subtests)
              </label>
            </div>
          </div>
        </div>
      </details>
    `;
  }

  private onRevisionModeChange(e: Event) {
    const select = e.target as HTMLSelectElement;
    this.dispatchEvent(
      new CustomEvent<RevisionGroupingMode>('revision-mode-change', {
        detail: select.value as RevisionGroupingMode,
        bubbles: true,
      })
    );
  }

  private onGroupSinglesChange(e: Event) {
    const checkbox = e.target as HTMLInputElement;
    this.dispatchEvent(
      new CustomEvent<boolean>('group-singles-change', {
        detail: checkbox.checked,
        bubbles: true,
      })
    );
  }

  private onGroupByChange(e: Event, criteria: GroupingCriteria) {
    const checkbox = e.target as HTMLInputElement;
    this.dispatchEvent(
      new CustomEvent<{ criteria: GroupingCriteria; enabled: boolean }>('group-by-change', {
        detail: {
          criteria,
          enabled: checkbox.checked,
        },
        bubbles: true,
      })
    );
  }
}
