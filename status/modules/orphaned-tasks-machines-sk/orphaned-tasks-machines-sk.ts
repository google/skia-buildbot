import { html } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import '../../../infra-sk/modules/theme-chooser-sk';
import '../../../infra-sk/modules/app-sk';
import '../../../infra-sk/modules/alogin-sk';
import '../../../elements-sk/modules/error-toast-sk';
import '../../../elements-sk/modules/icons/battery-charging-80-icon-sk';
import '../../../elements-sk/modules/icons/dashboard-icon-sk';
import '../../../elements-sk/modules/icons/devices-icon-sk';
import '../navigation-sk';
import { errorMessage } from '../../../elements-sk/modules/errorMessage';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { swarmingUrl } from '../settings';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';

export interface Group {
  tasks: string[];
  machines: string[];
  dimensions: string[];
  last_task_id: string;
}

export interface Report {
  no_matching_machines: Group[];
  no_matching_tasks: Group[];
  timestamp: Date;
}

export class OrphanedTasksMachinesSk extends ElementSk {
  private report: Report | null = null;

  private static template = (el: OrphanedTasksMachinesSk) => {
    const swarmUrl = swarmingUrl('');
    return html`
      <app-sk>
        <header>
          <h1>Orphaned Tasks/Machines</h1>
          <div class="spacer"></div>
          <alogin-sk></alogin-sk>
          <theme-chooser-sk></theme-chooser-sk>
        </header>
        <aside>
          <div>
            <navigation-sk></navigation-sk>
          </div>
        </aside>
        <main>
          ${el.report
            ? html`
                <p>
                  <em>Report generated at: ${new Date(el.report.timestamp).toLocaleString()}</em>
                </p>

                ${el.report.no_matching_machines?.length > 0
                  ? html`
                      <h2>
                        Tasks with No Matching Machines
                        (${el.report.no_matching_machines?.length || 0})
                      </h2>
                      <table>
                        <thead>
                          <tr>
                            <th>Dimensions</th>
                            <th>Affected Tasks</th>
                            <th>Last Successful Task ID</th>
                            <th>Action</th>
                          </tr>
                        </thead>
                        <tbody>
                          ${el.report.no_matching_machines.map(
                            (g) => html`
                              <tr>
                                <td>
                                  ${g.dimensions.map(
                                    (d) => html`<span class="dimension-tag">${d}</span>`
                                  )}
                                </td>
                                <td>
                                  <ul>
                                    ${g.tasks.map((t) => html`<li>${t}</li>`)}
                                  </ul>
                                </td>
                                <td>
                                  ${g.last_task_id
                                    ? html`
                                        <a
                                          href="${swarmUrl}/task?id=${g.last_task_id}"
                                          target="_blank"
                                          rel="noopener noreferrer"
                                          >${g.last_task_id}</a
                                        >
                                      `
                                    : html``}
                                </td>
                                <td>
                                  <a
                                    href="${swarmUrl}/botlist?f=${g.dimensions.join('&f=')}"
                                    target="_blank"
                                    rel="noopener noreferrer"
                                    class="link-btn"
                                    >Search Swarming Bots</a
                                  >
                                </td>
                              </tr>
                            `
                          )}
                        </tbody>
                      </table>
                    `
                  : html``}
                ${el.report.no_matching_tasks?.length > 0
                  ? html`
                      <h2>
                        Machines with No Matching Tasks
                        (${el.report.no_matching_tasks?.length || 0})
                      </h2>
                      <table>
                        <thead>
                          <tr>
                            <th>Dimensions</th>
                            <th>Unused Machines</th>
                            <th>Action</th>
                          </tr>
                        </thead>
                        <tbody>
                          ${el.report.no_matching_tasks.map(
                            (g) => html`
                              <tr>
                                <td>
                                  ${g.dimensions.map(
                                    (d) => html`<span class="dimension-tag">${d}</span>`
                                  )}
                                </td>
                                <td>
                                  <ul>
                                    ${g.machines.map((m) => html`<li>${m}</li>`)}
                                  </ul>
                                </td>
                                <td>
                                  <a
                                    href="${swarmUrl}/botlist?f=${g.dimensions.join('&f=')}"
                                    target="_blank"
                                    rel="noopener noreferrer"
                                    class="link-btn"
                                    >Search Swarming Bots</a
                                  >
                                </td>
                              </tr>
                            `
                          )}
                        </tbody>
                      </table>
                    `
                  : html``}
                ${!el.report.no_matching_tasks?.length && !el.report.no_matching_machines?.length
                  ? html` <p>No orphaned tasks or machines.</p> `
                  : html``}
              `
            : html`
                <div class="loading-msg">
                  The report is still being generated by the backend. This page will automatically
                  update.
                </div>
              `}
        </main>
        <footer><error-toast-sk></error-toast-sk></footer>
      </app-sk>
    `;
  };

  constructor() {
    super(OrphanedTasksMachinesSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
    this.refresh();
  }

  private refresh() {
    fetch('/json/orphaned-tasks-machines')
      .then((resp) => jsonOrThrow(resp))
      .then((data) => {
        if (data.status === 'loading') {
          this._render();
          window.setTimeout(() => this.refresh(), 10 * 1000);
          return;
        }
        this.report = data as Report;
        this._render();
      })
      .catch((err) => {
        this._render();
        errorMessage(err);
        window.setTimeout(() => this.refresh(), 10 * 1000);
      });
  }
}

define('orphaned-tasks-machines-sk', OrphanedTasksMachinesSk);
