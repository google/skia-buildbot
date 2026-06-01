/**
 * @module modules/subscription-table-sk
 * @description <h2><code>subscription-table-sk</code></h2>
 *
 * Displays details about a subscription and its associated alerts in a table.
 *
 * @example
 */
import '../../../infra-sk/modules/paramset-sk';
import { html, LitElement } from 'lit';
import { customElement, state, property } from 'lit/decorators.js';
import { Alert, Subscription, AnomalyDetectionRule } from '../json';
import { toParamSet } from '../../../infra-sk/modules/query';
import '../window/window';

@customElement('subscription-table-sk')
export class SubscriptionTableSk extends LitElement {
  // The currently loaded subscription.
  @property({ attribute: false })
  subscription: Subscription | null = null;

  // The config url.
  @property({ type: String })
  configUrl: string = '';

  // The alerts associated with the current subscription.
  @property({ attribute: false })
  alerts: Alert[] | null = null;

  // Controls the visibility of the alerts table.
  @state()
  private showAlerts: boolean = false;

  createRenderRoot() {
    return this;
  }

  render() {
    return html`${this.subscription
      ? html`
          <div class="subscription-details">
            <h2>${this.subscription.name}</h2>
            <p>
              <strong>Config:</strong>
              ${this.formatConfigUrl(this.subscription.revision!)}
            </p>
            <p><strong>Revision:</strong> ${this.formatRevision(this.subscription.revision!)}</p>
            <p><strong>Contact email:</strong> ${this.subscription.contact_email || ''}</p>
            <div class="bug-config">
              <h3>Bugs configuration</h3>
              <p>
                <strong>Component:</strong> ${this.formatBugComponent(
                  this.subscription.bug_component || ''
                )}
              </p>
              <p>
                <strong>Priority:</strong> ${this.subscription.bug_priority},
                <strong>Severity:</strong> ${this.subscription.bug_severity}
              </p>
              <p>
                <strong>Hotlists:</strong> ${this.subscription.hotlists?.length
                  ? this.subscription.hotlists.join(', ')
                  : html`<em>(not set)</em>`}
              </p>
              <p>
                <strong>CC's:</strong> ${this.subscription.bug_cc_emails?.length
                  ? this.subscription.bug_cc_emails.join(', ')
                  : html`<em>(not set)</em>`}
              </p>
            </div>
            <button id="btn-toggle-alerts" @click=${() => this.toggleAlerts()}>
              ${this.showAlerts
                ? 'Hide alerts configuration'
                : `Show alerts configuration (${this.alerts?.length || 0})`}
            </button>
            ${this.showAlerts
              ? html`
                  <table id="alerts-table">
                    <thead>
                      <tr>
                        <th
                          id="matching-rules"
                          title="Rules how to select traces to apply anomaly detection algorithms. ! for negation, ~ for regex matching.">
                          Trace matching rules
                        </th>
                        <th id="algorithm" title="Anomaly detection algorithm">Algorithm</th>
                        <th id="other">Params</th>
                        <th
                          id="action"
                          title="What actions should be taken for detected anomalies: NOACTION (for manual triage), TRIAGE (files a bug), or BISECT (triggers culprit finding workflow).">
                          Action
                        </th>
                      </tr>
                    </thead>
                    <tbody>
                      ${this.alerts && this.alerts.length > 0
                        ? this.alerts.map(
                            (alert) => html`
                              <tr>
                                <td>
                                  <paramset-sk
                                    .paramsets=${[toParamSet(alert.query)]}></paramset-sk>
                                </td>
                                <td>${this.formatStepAlgorithm(alert)}</td>
                                <td class="other-config">
                                  <span
                                    title="How many commits to each side of a commit to consider when looking for a step."
                                    >Radius: ${alert.radius}</span
                                  ><br />
                                  <span
                                    title="If algo is set to K-means, this determines the K in K-means clustering, otherwise this value is ignored."
                                    >K: ${alert.k}</span
                                  ><br />
                                  <span
                                    title="The threshold value beyond which values become interesting (indicates a real regression)."
                                    >Interesting: ${alert.interesting}</span
                                  ><br />
                                  <span
                                    title="How many traces need to be found interesting before an alert is fired."
                                    >Minimum number: ${alert.minimum_num}</span
                                  ><br />
                                  <span title="If true, only include commits that have data."
                                    >Sparse: ${alert.sparse ? 'True' : 'False'}</span
                                  ><br />
                                  <span
                                    title="Which direction of change is detected as an anomaly ('UP', 'DOWN', or 'BOTH'). The trace's 'improvement_direction' parameter determines whether the anomaly is treated as a regression (e.g., to trigger bug filing or bisection) or an improvement. Using 'BOTH' is useful for getting 'FYI' detections for improvements without triggering regression workflows."
                                    >Direction: ${alert.direction}</span
                                  >
                                </td>
                                <td>${this.formatAction(alert.action)}</td>
                              </tr>
                            `
                          )
                        : html``}
                    </tbody>
                  </table>
                `
              : html``}
          </div>
        `
      : html``} `;
  }

  protected willUpdate(changedProperties: Map<string, any>): void {
    if (changedProperties.has('subscription')) {
      this.showAlerts = false;
    }
  }

  /**
   * Loads a subscription and its alerts into the table.
   * @param subscription The subscription to display.
   * @param alerts The alerts associated with the subscription.
   * Deprecated: Use .subscription and .alerts properties instead.
   */
  load(subscription: Subscription, alerts: Alert[]) {
    this.subscription = subscription;
    this.alerts = alerts;
    this.showAlerts = false;
  }

  /** Toggles the visibility of the alerts table. */
  toggleAlerts() {
    this.showAlerts = !this.showAlerts;
  }

  /** Formats the step algorithm display. */
  private formatStepAlgorithm(alert: Alert): string {
    if (alert.detection_rule) {
      return this.formatDetectionRule(alert.detection_rule);
    }
    return alert.step;
  }

  private formatDetectionRule(rule: AnomalyDetectionRule | null): string {
    if (!rule) return '';
    if (rule.simple_rule) {
      return `${rule.simple_rule.step} (th: ${rule.simple_rule.threshold})`;
    }
    if (rule.complex_rule) {
      const op = rule.complex_rule.op || '';
      const rules = rule.complex_rule.rules || [];
      const formattedRules = rules.map((r) => this.formatDetectionRule(r)).join(` ${op} `);
      return `(${formattedRules})`;
    }
    return '';
  }

  private formatConfigUrl(revision: string) {
    const url = this.getConfigUrl(revision);
    if (!url) {
      return html`<em>(not set)</em>`;
    }
    const urlText = url.split(':').pop() || '';
    return html`<a href="${url}" target="_blank" rel="noopener noreferrer">${urlText}</a>`;
  }

  private getConfigUrl(revision: string) {
    let url = this.configUrl;
    if (!url) {
      return '';
    }
    if (url.startsWith('https://chrome-internal.googlesource.com/')) {
      // Open internal codesearch for faster editing.
      url = url.replace(
        'https://chrome-internal.googlesource.com/',
        'https://source.corp.google.com/h/chrome-internal/'
      );
      url = url.replace('/+/{revision}/', '/+/{revision}:');
    }
    return url.replace('{revision}', revision);
  }

  /** Formats the revision string as a link to the config file.
   * @param revision The revision string.
   * @returns A lit/html TemplateResult representing the link.
   */
  private formatRevision(revision: string) {
    const url = this.getConfigUrl(revision);
    if (!url) {
      return html`${revision}`;
    }
    return html`<a href="${url}" target="_blank" rel="noopener noreferrer">${revision}</a>`;
  }

  /** Formats the alert action with a tooltip explaining its meaning. */
  private formatAction(action?: string) {
    let title = '';
    switch (action?.toUpperCase()) {
      case 'NOACTION':
        title = 'For manual triage';
        break;
      case 'TRIAGE':
        title = 'Files a bug';
        break;
      case 'BISECT':
        title = 'Triggers culprit finding workflow';
        break;
      default:
        return html`${action}`;
    }
    return html`<span title="${title}">${action}</span>`;
  }

  /** * Formats the bug component string as a link to the issue tracker.
   * Assumes the component string is an ID (e.g. "1547614").
   */
  private formatBugComponent(component: string) {
    if (!component) return html``;

    // Construct the URL with the component ID
    const queryValue = `status:open componentid:${component}`;
    const encodedQuery = encodeURIComponent(queryValue);
    const url = `https://g-issues.chromium.org/issues?q=${encodedQuery}&s=created_time:desc`;

    return html`<a href="${url}" target="_blank" rel="noopener noreferrer">${component}</a>`;
  }
}
