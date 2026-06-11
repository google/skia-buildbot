/**
 * @module modules/sheriff-configs-dry-run-sk
 * @description <h2><code>sheriff-configs-dry-run-sk</code></h2>
 *
 * Component for creating and exporting sheriff configs to protobuf text format
 * for dry-run testing.
 */
import { html, LitElement } from 'lit';
import { customElement, state } from 'lit/decorators.js';
import '@material/web/radio/radio.js';
import '@material/web/slider/slider.js';
import '@material/web/select/outlined-select.js';
import '@material/web/select/select-option.js';
import '@material/web/textfield/outlined-text-field.js';
import '@material/web/checkbox/checkbox.js';
import { errorMessage } from '../errorMessage';

export interface AnomalyConfig {
  threshold?: number;
  step?: number;
  radius: number;
  sparse: boolean;
  rules?: { match?: string[]; exclude?: string[] };
  action: number;
  detection_rule?: any;
}

export interface SheriffConfig {
  subscriptions: Array<{
    name?: string;
    contact_email?: string;
    bug_component?: string;
    anomaly_configs: AnomalyConfig[];
  }>;
}

@customElement('sheriff-configs-dry-run-sk')
export class SheriffConfigsDryRunSk extends LitElement {
  @state() private configName: string = '';

  @state() private contactEmail: string = '';

  @state() private bugComponent: string = '';

  @state() private ruleType: 'simple' | 'compound' = 'simple';

  @state() private threshold: number = 2.5;

  @state() private step: string = 'COHEN_STEP';

  @state() private compoundOp: 'AND' | 'OR' = 'OR';

  @state() private compoundRules: Array<{ step: string; threshold: number }> = [
    { step: 'COHEN_STEP', threshold: 2.5 },
    { step: 'ABSOLUTE_STEP', threshold: 5.0 },
  ];

  @state() private radius: number = 8;

  @state() private sparse: boolean = true;

  @state() private rulesMatch: string =
    'master=internal.client.v8&benchmark=v8&test=JetStream2&subtest_1=default' +
    '&subtest_4=pgo&stat=value';

  @state() private rulesExclude: string = 'test=~.*(_avg|_min|_max|_sum|_count|_std)$';

  @state() private action: string = '';

  @state() private viewMode: 'builder' | 'proto' = 'builder';

  @state() private protoText: string = '';

  private lastImportError: string = '';

  createRenderRoot() {
    return this;
  }

  render() {
    return html`
      <h3>Sheriff Config Dry Run</h3>

      <div class="header-row">
        <label class="view-mode-label">
          <md-radio
            name="viewMode"
            value="builder"
            ?checked=${this.viewMode === 'builder'}
            @change=${this.toggleViewMode}></md-radio>
          <span class="view-mode-text">Builder</span>
        </label>
        <label class="view-mode-label">
          <md-radio
            name="viewMode"
            value="proto"
            ?checked=${this.viewMode === 'proto'}
            @change=${this.toggleViewMode}></md-radio>
          <span class="view-mode-text">Protobuf Text</span>
        </label>
      </div>

      ${this.viewMode === 'builder'
        ? html`
            <div class="builder-container">
              <div class="cards-grid">
                <!-- Anomaly Configs Card -->
                <div class="card">
                  <h4 class="card-title">Anomaly Configs</h4>

                  <div class="rule-type-selector" style="margin-bottom: 16px;">
                    <label class="view-mode-label">
                      <md-radio
                        name="ruleType"
                        value="simple"
                        ?checked=${this.ruleType === 'simple'}
                        @change=${() => {
                          this.ruleType = 'simple';
                        }}></md-radio>
                      <span class="view-mode-text">Simple</span>
                    </label>
                    <label class="view-mode-label">
                      <md-radio
                        name="ruleType"
                        value="compound"
                        ?checked=${this.ruleType === 'compound'}
                        @change=${() => {
                          this.ruleType = 'compound';
                        }}></md-radio>
                      <span class="view-mode-text">Compound</span>
                    </label>
                  </div>

                  ${this.ruleType === 'simple'
                    ? html`
                        <div class="slider-container">
                          <label for="threshold" class="slider-label"
                            >Threshold: ${this.threshold}</label
                          >
                          <md-slider
                            id="threshold"
                            min="0"
                            max="10"
                            step="0.1"
                            .value=${this.threshold}
                            @input=${(e: Event) => {
                              this.threshold = Number((e.target as HTMLInputElement).value);
                            }}></md-slider>
                        </div>

                        <md-outlined-select
                          label="Step"
                          id="step"
                          .value=${this.step}
                          @change=${(e: Event) => {
                            this.step = (e.target as HTMLSelectElement).value;
                          }}>
                          <md-select-option value="COHEN_STEP"
                            ><div slot="headline">COHEN_STEP</div></md-select-option
                          >
                          <md-select-option value="PERCENT_STEP"
                            ><div slot="headline">PERCENT_STEP</div></md-select-option
                          >
                          <md-select-option value="CONST_STEP"
                            ><div slot="headline">CONST_STEP</div></md-select-option
                          >
                          <md-select-option value="ABSOLUTE_STEP"
                            ><div slot="headline">ABSOLUTE_STEP</div></md-select-option
                          >
                          <md-select-option value="MANN_WHITNEY_U"
                            ><div slot="headline">MANN_WHITNEY_U</div></md-select-option
                          >
                          <md-select-option value="STEPINESS"
                            ><div slot="headline">STEPINESS</div></md-select-option
                          >
                        </md-outlined-select>
                      `
                    : html`
                        <div class="compound-container">
                          <md-outlined-select
                            label="Operator"
                            .value=${this.compoundOp}
                            @change=${(e: Event) => {
                              this.compoundOp = (e.target as HTMLSelectElement).value as
                                | 'AND'
                                | 'OR';
                            }}>
                            <md-select-option value="OR"
                              ><div slot="headline">OR</div></md-select-option
                            >
                            <md-select-option value="AND"
                              ><div slot="headline">AND</div></md-select-option
                            >
                          </md-outlined-select>

                          <div class="subrules-list">
                            <label class="slider-label">Sub-rules:</label>
                            ${this.compoundRules.map(
                              (rule, index) => html`
                                <div class="subrule-item">
                                  <md-outlined-select
                                    label="Step"
                                    .value=${rule.step}
                                    @change=${(e: Event) => {
                                      this.compoundRules[index].step = (
                                        e.target as HTMLSelectElement
                                      ).value;
                                      this.compoundRules = [...this.compoundRules];
                                    }}>
                                    <md-select-option value="COHEN_STEP"
                                      ><div slot="headline">COHEN_STEP</div></md-select-option
                                    >
                                    <md-select-option value="PERCENT_STEP"
                                      ><div slot="headline">PERCENT_STEP</div></md-select-option
                                    >
                                    <md-select-option value="CONST_STEP"
                                      ><div slot="headline">CONST_STEP</div></md-select-option
                                    >
                                    <md-select-option value="ABSOLUTE_STEP"
                                      ><div slot="headline">ABSOLUTE_STEP</div></md-select-option
                                    >
                                    <md-select-option value="MANN_WHITNEY_U"
                                      ><div slot="headline">MANN_WHITNEY_U</div></md-select-option
                                    >
                                    <md-select-option value="STEPINESS"
                                      ><div slot="headline">STEPINESS</div></md-select-option
                                    >
                                  </md-outlined-select>

                                  <div class="slider-container">
                                    <label class="slider-label">Threshold: ${rule.threshold}</label>
                                    <md-slider
                                      min="0"
                                      max="10"
                                      step="0.1"
                                      .value=${rule.threshold}
                                      @input=${(e: Event) => {
                                        this.compoundRules[index].threshold = Number(
                                          (e.target as HTMLInputElement).value
                                        );
                                        this.compoundRules = [...this.compoundRules];
                                      }}></md-slider>
                                  </div>

                                  <button
                                    class="action-button"
                                    @click=${() => {
                                      this.compoundRules.splice(index, 1);
                                      this.compoundRules = [...this.compoundRules];
                                    }}>
                                    Remove
                                  </button>
                                </div>
                              `
                            )}
                          </div>
                          <button
                            class="action-button"
                            @click=${() => {
                              this.compoundRules.push({ step: 'COHEN_STEP', threshold: 2.5 });
                              this.compoundRules = [...this.compoundRules];
                            }}>
                            Add Rule
                          </button>
                        </div>
                      `}

                  <div class="slider-container">
                    <label for="radius" class="slider-label">Radius: ${this.radius}</label>
                    <md-slider
                      id="radius"
                      min="1"
                      max="20"
                      step="1"
                      .value=${this.radius}
                      @input=${(e: Event) => {
                        this.radius = Number((e.target as HTMLInputElement).value);
                      }}></md-slider>
                  </div>

                  <label class="checkbox-label">
                    <md-checkbox
                      ?checked=${this.sparse}
                      @change=${(e: Event) => {
                        this.sparse = (e.target as HTMLInputElement).checked;
                      }}></md-checkbox>
                    <span class="checkbox-text">Sparse</span>
                  </label>
                </div>

                <!-- Rules Card -->
                <div class="card">
                  <h4 class="card-title">Rules</h4>

                  <md-outlined-text-field
                    type="textarea"
                    label="Match (one per line)"
                    rows="4"
                    .value=${this.rulesMatch}
                    @input=${(e: Event) => {
                      this.rulesMatch = (e.target as HTMLInputElement).value;
                    }}>
                  </md-outlined-text-field>

                  <md-outlined-text-field
                    type="textarea"
                    label="Exclude (one per line)"
                    rows="4"
                    .value=${this.rulesExclude}
                    @input=${(e: Event) => {
                      this.rulesExclude = (e.target as HTMLInputElement).value;
                    }}>
                  </md-outlined-text-field>
                </div>
              </div>

              <!-- Production Fields Spoiler -->
              <details style="margin-top: 8px;">
                <summary class="spoiler-summary">Production Fields (Optional)</summary>

                <div class="production-fields">
                  <md-outlined-text-field
                    label="Name"
                    .value=${this.configName}
                    @input=${(e: Event) => {
                      this.configName = (e.target as HTMLInputElement).value;
                    }}></md-outlined-text-field>

                  <md-outlined-text-field
                    label="Contact Email"
                    .value=${this.contactEmail}
                    @input=${(e: Event) => {
                      this.contactEmail = (e.target as HTMLInputElement).value;
                    }}></md-outlined-text-field>

                  <md-outlined-text-field
                    label="Bug Component"
                    .value=${this.bugComponent}
                    @input=${(e: Event) => {
                      this.bugComponent = (e.target as HTMLInputElement).value;
                    }}></md-outlined-text-field>

                  <md-outlined-select
                    label="Action"
                    id="action"
                    .value=${this.action}
                    @change=${(e: Event) => {
                      this.action = (e.target as HTMLSelectElement).value;
                    }}>
                    <md-select-option value=""><div slot="headline">(None)</div></md-select-option>
                    <md-select-option value="NOACTION"
                      ><div slot="headline">NOACTION</div></md-select-option
                    >
                    <md-select-option value="BISECT"
                      ><div slot="headline">BISECT</div></md-select-option
                    >
                    <md-select-option value="REPORT"
                      ><div slot="headline">REPORT</div></md-select-option
                    >
                  </md-outlined-select>
                </div>
              </details>
            </div>
          `
        : html`
            <md-outlined-text-field
              id="proto-text"
              type="textarea"
              class="proto-textarea"
              .value=${this.protoText}>
            </md-outlined-text-field>
          `}
    `;
  }

  private toggleViewMode(e: Event) {
    const newMode = (e.target as HTMLInputElement).value as 'builder' | 'proto';
    if (newMode === 'proto') {
      this.protoText = this.getProto();
      this.viewMode = newMode;
    } else if (newMode === 'builder') {
      const tf = this.querySelector('#proto-text') as HTMLInputElement | null;
      if (tf) {
        this.protoText = tf.value;
      }
      const success = this.importProto(this.protoText);
      if (!success) {
        errorMessage(`Malformed protobuf text: ${this.lastImportError}`);
        const protoRadio = this.querySelector<HTMLInputElement>('md-radio[value="proto"]');
        if (protoRadio) protoRadio.checked = true;
        return;
      }
      this.viewMode = newMode;
    }
  }

  public getConfig(): string | null {
    if (this.viewMode === 'proto') {
      const tf = this.querySelector('#proto-text') as HTMLInputElement | null;
      if (tf) {
        this.protoText = tf.value;
      }
    }

    return this.getProto();
  }

  public getProto(): string {
    if (this.viewMode === 'proto') {
      return this.protoText;
    }

    const lines: string[] = ['subscriptions {'];
    if (this.configName) {
      lines.push(`  name: "${this.configName}"`);
    }
    if (this.contactEmail) {
      lines.push(`  contact_email: "${this.contactEmail}"`);
    }
    if (this.bugComponent) {
      lines.push(`  bug_component: "${this.bugComponent}"`);
    }
    lines.push('  anomaly_configs {');

    if (this.ruleType === 'simple') {
      lines.push(`    threshold: ${this.threshold}`);
      lines.push(`    step: ${this.step}`);
      lines.push(`    radius: ${this.radius}`);
      lines.push(`    sparse: ${this.sparse ? 'True' : 'False'}`);
    } else {
      lines.push(`    radius: ${this.radius}`);
      lines.push(`    sparse: ${this.sparse ? 'True' : 'False'}`);
      lines.push(`    detection_rule {`);
      lines.push(`      complex_rule {`);
      lines.push(`        op: ${this.compoundOp}`);
      for (const r of this.compoundRules) {
        lines.push(`        rules {`);
        lines.push(`          simple_rule {`);
        lines.push(`            step: ${r.step}`);
        lines.push(`            threshold: ${r.threshold}`);
        lines.push(`          }`);
        lines.push(`        }`);
      }
      lines.push(`      }`);
      lines.push(`    }`);
    }

    const matches = this.rulesMatch
      .split('\n')
      .map((s: string) => s.trim())
      .filter((s: string) => s !== '');
    const excludes = this.rulesExclude
      .split('\n')
      .map((s: string) => s.trim())
      .filter((s: string) => s !== '');

    if (matches.length > 0 || excludes.length > 0) {
      lines.push('    rules: {');
      if (matches.length > 0) {
        lines.push('      match: [');
        matches.forEach((m, i) => lines.push(`        "${m}"${i < matches.length - 1 ? ',' : ''}`));
        lines.push('      ]');
      }
      if (excludes.length > 0) {
        lines.push('      exclude: [');
        excludes.forEach((e, i) =>
          lines.push(`        "${e}"${i < excludes.length - 1 ? ',' : ''}`)
        );
        lines.push('      ]');
      }
      lines.push('    }');
    }

    if (this.action) {
      lines.push(`    action: ${this.action}`);
    }

    lines.push('  }');
    lines.push('}');

    return lines.join('\n');
  }

  public importProto(proto: string): boolean {
    this.lastImportError = '';
    if (!proto.includes('subscriptions') || !proto.includes('anomaly_configs')) {
      this.lastImportError = 'Missing required "subscriptions" or "anomaly_configs" blocks.';
      return false;
    }

    const ruleWords = proto.match(/\b[a-zA-Z0-9_]+_rul[a-z]*\b/g) || [];
    if (ruleWords.some((w) => !['detection_rule', 'complex_rule', 'simple_rule'].includes(w))) {
      const badWord = ruleWords.find(
        (w) => !['detection_rule', 'complex_rule', 'simple_rule'].includes(w)
      );
      this.lastImportError = `Invalid rule identifier "${badWord}". Expected detection_rule, complex_rule, or simple_rule.`;
      return false;
    }

    const extractString = (key: string) => {
      const regex = new RegExp(`\\b${key}:\\s*"([^"]+)"`);
      const match = proto.match(regex);
      return match ? match[1] : '';
    };

    const extractNumber = (key: string, def: number) => {
      const regex = new RegExp(`\\b${key}:\\s*([0-9.]+)`);
      const match = proto.match(regex);
      return match ? parseFloat(match[1]) : def;
    };

    const extractEnum = (key: string, def: string) => {
      const regex = new RegExp(`\\b${key}:\\s*([a-zA-Z0-9_]+)`);
      const match = proto.match(regex);
      return match ? match[1] : def;
    };

    const extractBoolean = (key: string, def: boolean) => {
      const regex = new RegExp(`\\b${key}:\\s*([a-zA-Z0-9_]+)`);
      const match = proto.match(regex);
      if (match) return match[1] === 'True';
      return def;
    };

    const extractArray = (key: string): string => {
      // Try bracket notation first: match: ["a", "b"]
      const blockRegex = new RegExp(`\\b${key}:\\s*\\[([^\\]]*)\\]`);
      const blockMatch = proto.match(blockRegex);

      if (blockMatch) {
        const items = blockMatch[1].match(/"([^"]+)"/g) || [];
        return items.map((i: string) => i.replace(/(^"|"$)/g, '')).join('\n');
      }

      // Fallback to repeated fields: match: "a" \n match: "b"
      const repeatedRegex = new RegExp(`\\b${key}:\\s*"([^"]+)"`, 'g');
      const items = [];
      let match;
      while ((match = repeatedRegex.exec(proto)) !== null) {
        items.push(match[1]);
      }
      return items.join('\n');
    };

    this.configName = extractString('name');
    this.contactEmail = extractString('contact_email');
    this.bugComponent = extractString('bug_component');
    this.radius = extractNumber('radius', 8);
    this.sparse = extractBoolean('sparse', true);
    this.rulesMatch = extractArray('match');
    this.rulesExclude = extractArray('exclude');

    const act = extractEnum('action', '');
    if (act !== '' && !['NOACTION', 'TRIAGE', 'BISECT', 'REPORT'].includes(act)) {
      this.lastImportError = `Invalid action "${act}". Expected NOACTION, TRIAGE, BISECT, or REPORT.`;
      return false;
    }
    this.action = act;

    const validSteps = [
      'ORIGINAL_STEP',
      'ABSOLUTE_STEP',
      'CONST_STEP',
      'PERCENT_STEP',
      'COHEN_STEP',
      'MANN_WHITNEY_U',
      'STEPINESS',
      'PERCENT_MEDIAN_STEP',
    ];

    if (proto.includes('detection_rule')) {
      this.ruleType = 'compound';
      const opMatch = proto.match(/\bop:\s*([a-zA-Z0-9_]+)/);
      if (opMatch) {
        if (!['AND', 'OR'].includes(opMatch[1])) {
          this.lastImportError = `Invalid operator "${opMatch[1]}". Expected AND or OR.`;
          return false;
        }
        this.compoundOp = opMatch[1] as 'AND' | 'OR';
      } else {
        this.compoundOp = 'OR';
      }

      const simpleRuleBlockRegex = /simple_rule\s*\{([^}]+)\}/g;
      const rules = [];
      let match;
      while ((match = simpleRuleBlockRegex.exec(proto)) !== null) {
        const block = match[1];
        const stepMatch = block.match(/\bstep:\s*([a-zA-Z0-9_]+)/);
        const threshMatch = block.match(/\bthreshold:\s*([0-9.]+)/);
        if (stepMatch && threshMatch) {
          const st = stepMatch[1];
          if (!validSteps.includes(st)) {
            this.lastImportError = `Invalid step "${st}". Expected one of: ${validSteps.join(', ')}.`;
            return false;
          }
          rules.push({ step: st, threshold: parseFloat(threshMatch[1]) });
        } else {
          this.lastImportError =
            'A simple_rule block is missing required "step" or "threshold" fields.';
          return false;
        }
      }
      if (rules.length === 0) {
        this.lastImportError = 'A complex_rule requires at least one simple_rule block.';
        return false;
      }
      this.compoundRules = rules;
    } else {
      this.ruleType = 'simple';
      this.threshold = extractNumber('threshold', 2.5);
      const st = extractEnum('step', 'COHEN_STEP');
      if (!validSteps.includes(st)) {
        this.lastImportError = `Invalid step "${st}". Expected one of: ${validSteps.join(', ')}.`;
        return false;
      }
      this.step = st;
    }
    return true;
  }
}
