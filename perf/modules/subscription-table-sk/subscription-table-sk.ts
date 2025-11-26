/**
 * @module modules/subscription-table-sk
 * @description <h2><code>subscription-table-sk</code></h2>
 *
 * Displays details about a subscription and its associated alerts in a table.
 *
 * @example
 */
import '../../../infra-sk/modules/paramset-sk';
import { html } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { Alert, Subscription } from '../json';
import { toParamSet } from '../../../infra-sk/modules/query';

export class SubscriptionTableSk extends ElementSk {
  // The currently loaded subscription.
  private subscription: Subscription | null = null;

  // The alerts associated with the current subscription.
  private alerts: Alert[] | null = null;

  // Controls the visibility of the alerts table.
  private showAlerts: boolean = false;

  constructor() {
    super(SubscriptionTableSk.template);
  }

  private static template = (ele: SubscriptionTableSk) =>
    html`${ele.subscription
      ? html`
          <div class="subscription-details">
            <h2>${ele.subscription.name} (${ele.alerts?.length || 0} Alert(s) Configured)</h2>
            <p><strong>Contact Email:</strong> ${ele.subscription.contact_email || ''}</p>
            <p><strong>Revision:</strong> ${ele.formatRevision(ele.subscription.revision!)}</p>
            <p>
              <strong>Component:</strong> ${ele.formatBugComponent(
                ele.subscription.bug_component || ''
              )}
            </p>
            <p><strong>Hotlists:</strong> ${ele.subscription.hotlists?.join(', ') || ''}</p>
            <p>
              <strong>Priority:</strong> ${ele.subscription.bug_priority},
              <strong>Severity:</strong> ${ele.subscription.bug_severity}
            </p>
            <p><strong>CC's:</strong> ${ele.subscription.bug_cc_emails?.join(', ') || ''}</p>
          </div>
          <button id="btn-toggle-alerts" @click=${() => ele.toggleAlerts()}>
            ${ele.showAlerts
              ? 'Hide Alert Configuration(s) '
              : `Show ${ele.alerts?.length || 0} Alert Configuration(s)`}
          </button>
        `
      : html``}
    ${ele.showAlerts
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
              ${ele.alerts && ele.alerts.length > 0
                ? ele.alerts.map(
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

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
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
    this._render();
  }

  /** Toggles the visibility of the alerts table. */
  toggleAlerts() {
    this.showAlerts = !this.showAlerts;
    this._render();
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

define('subscription-table-sk', SubscriptionTableSk);
