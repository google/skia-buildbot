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
import { customElement, state } from 'lit/decorators.js';
import { Alert, Subscription } from '../json';
import { toParamSet } from '../../../infra-sk/modules/query';

@customElement('subscription-table-sk')
export class SubscriptionTableSk extends LitElement {
  // The currently loaded subscription.
  @state()
  private subscription: Subscription | null = null;

  // The alerts associated with the current subscription.
  @state()
  private alerts: Alert[] | null = null;

  // Controls the visibility of the alerts table.
  @state()
  private showAlerts: boolean = false;

  constructor() {
    super();
  }

  createRenderRoot() {
    return this;
  }

  render() {
    return html`${this.subscription
      ? html`
          <div class="subscription-details">
            <h2>${this.subscription.name} (${this.alerts?.length || 0} Alert(s) Configured)</h2>
            <p><strong>Contact Email:</strong> ${this.subscription.contact_email || ''}</p>
            <p><strong>Revision:</strong> ${this.formatRevision(this.subscription.revision!)}</p>
            <p>
              <strong>Component:</strong> ${this.formatBugComponent(
                this.subscription.bug_component || ''
              )}
            </p>
            <p><strong>Hotlists:</strong> ${this.subscription.hotlists?.join(', ') || ''}</p>
            <p>
              <strong>Priority:</strong> ${this.subscription.bug_priority},
              <strong>Severity:</strong> ${this.subscription.bug_severity}
            </p>
            <p><strong>CC's:</strong> ${this.subscription.bug_cc_emails?.join(', ') || ''}</p>
          </div>
          <button id="btn-toggle-alerts" @click=${() => this.toggleAlerts()}>
            ${this.showAlerts
              ? 'Hide Alert Configuration(s) '
              : `Show ${this.alerts?.length || 0} Alert Configuration(s)`}
          </button>
        `
      : html``}
    ${this.showAlerts
      ? html`
          <table id="alerts-table">
            <thead>
              <tr>
                <th id="configuration">Configuration(s)</th>
                <th id="algorithm">Step Algorithm</th>
                <th id="radius">Radius</th>
                <th id="k">K</th>
                <th id="interesting">Interesting</th>
                <th id="minimum-num">Minimum Number</th>
                <th id="sparse">Sparse</th>
                <th id="direction">Direction</th>
                <th id="action">Action</th>
              </tr>
            </thead>
            <tbody>
              ${this.alerts && this.alerts.length > 0
                ? this.alerts.map(
                    (alert) => html`
                      <tr>
                        <td>
                          <paramset-sk .paramsets=${[toParamSet(alert.query)]}></paramset-sk>
                        </td>
                        <td>${alert.step}</td>
                        <td>${alert.radius}</td>
                        <td>${alert.k}</td>
                        <td>${alert.interesting}</td>
                        <td>${alert.minimum_num}</td>
                        <td>${alert.sparse ? 'Sparse' : 'Dense'}</td>
                        <td>${alert.direction}</td>
                        <td>${alert.action}</td>
                      </tr>
                    `
                  )
                : html``}
            </tbody>
          </table>
        `
      : html``} `;
  }

  /**
   * Loads a subscription and its alerts into the table.
   * @param subscription The subscription to display.
   * @param alerts The alerts associated with the subscription.
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

  /** Formats the revision string as a link to the config file.
   * @param revision The revision string.
   * @returns A lit/html TemplateResult representing the link.
   */
  private formatRevision(revision: string) {
    return html`<a
      href="https://chrome-internal.googlesource.com/infra/infra_internal/+/${revision}/infra/config/generated/skia-sheriff-configs.cfg"
      >${revision}</a
    >`;
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
