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

export interface AnomalyConfig {
  threshold: number;
  step: number;
  radius: number;
  sparse: boolean;
  rules?: { match?: string[]; exclude?: string[] };
  action: number;
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

  @state() private threshold: number = 2.5;

  @state() private step: string = 'COHEN_STEP';

  @state() private radius: number = 8;

  @state() private sparse: boolean = true;

  @state() private rulesMatch: string =
    'master=internal.client.v8&benchmark=v8&test=JetStream2&subtest_1=default' +
    '&subtest_4=pgo&stat=value';

  @state() private rulesExclude: string = 'test=~.*(_avg|_min|_max|_sum|_count|_std)$';

  @state() private action: string = '';

  @state() private viewMode: 'builder' | 'proto' = 'builder';

  @state() private protoText: string = '';

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

                  <div class="slider-container">
                    <label for="threshold" class="slider-label">Threshold: ${this.threshold}</label>
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
                    <md-select-option value="MANNWHITNEYU_STEP"
                      ><div slot="headline">MANNWHITNEYU_STEP</div></md-select-option
                    >
                  </md-outlined-select>

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
    } else if (newMode === 'builder') {
      const tf = this.querySelector('#proto-text') as HTMLInputElement | null;
      if (tf) {
        this.protoText = tf.value;
      }
      this.importProto(this.protoText);
    }
    this.viewMode = newMode;
  }

  public getConfig(): SheriffConfig {
    const matches = this.rulesMatch
      .split('\n')
      .map((s: string) => s.trim())
      .filter((s: string) => s !== '');
    const excludes = this.rulesExclude
      .split('\n')
      .map((s: string) => s.trim())
      .filter((s: string) => s !== '');

    const rules: any = {};
    if (matches.length > 0) rules.match = matches;
    if (excludes.length > 0) rules.exclude = excludes;

    const stepMap: Record<string, number> = {
      ORIGINAL_STEP: 0,
      ABSOLUTE_STEP: 1,
      CONST_STEP: 2,
      PERCENT_STEP: 3,
      COHEN_STEP: 4,
      MANN_WHITNEY_U: 5,
    };

    const actionMap: Record<string, number> = {
      NOACTION: 0,
      TRIAGE: 1,
      BISECT: 2,
    };

    return {
      subscriptions: [
        {
          name: this.configName,
          contact_email: this.contactEmail,
          bug_component: this.bugComponent,
          anomaly_configs: [
            {
              threshold: this.threshold,
              step: stepMap[this.step] !== undefined ? stepMap[this.step] : 0,
              radius: this.radius,
              sparse: this.sparse,
              rules: matches.length > 0 || excludes.length > 0 ? rules : undefined,
              action: actionMap[this.action] !== undefined ? actionMap[this.action] : 0,
            },
          ],
        },
      ],
    };
  }

  public getProto(): string {
    if (this.viewMode === 'proto') {
      return this.protoText;
    }
    const matches = this.rulesMatch
      .split('\n')
      .map((s: string) => s.trim())
      .filter((s: string) => s !== '');
    const excludes = this.rulesExclude
      .split('\n')
      .map((s: string) => s.trim())
      .filter((s: string) => s !== '');

    const formatArray = (name: string, arr: string[]) =>
      arr.length > 0
        ? `      ${name}: [\n${arr.map((m) => `        "${m}"`).join(',\n')}\n      ]`
        : '';

    const rulesBlock =
      matches.length > 0 || excludes.length > 0
        ? `    rules: {\n${[formatArray('match', matches), formatArray('exclude', excludes)]
            .filter((s) => s !== '')
            .join('\n')}\n    }`
        : '';

    return `subscriptions {
${this.configName ? `  name: "${this.configName}"\n` : ''}${
      this.contactEmail ? `  contact_email: "${this.contactEmail}"\n` : ''
    }${this.bugComponent ? `  bug_component: "${this.bugComponent}"\n` : ''}  anomaly_configs {
    threshold: ${this.threshold}
    step: ${this.step}
    radius: ${this.radius}
    sparse: ${this.sparse ? 'True' : 'False'}
${rulesBlock ? `${rulesBlock}\n` : ''}${this.action ? `    action: ${this.action}\n` : ''}  }
}`.trim();
  }

  public importProto(proto: string) {
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
      const regex = new RegExp(`\\b${key}:\\s*([A-Z_]+)`);
      const match = proto.match(regex);
      return match ? match[1] : def;
    };

    const extractBoolean = (key: string, def: boolean) => {
      const regex = new RegExp(`\\b${key}:\\s*(True|False)`);
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
    this.threshold = extractNumber('threshold', 2.5);
    this.step = extractEnum('step', 'COHEN_STEP');
    this.radius = extractNumber('radius', 8);
    this.sparse = extractBoolean('sparse', true);
    this.rulesMatch = extractArray('match');
    this.rulesExclude = extractArray('exclude');
    this.action = extractEnum('action', '');
  }
}
