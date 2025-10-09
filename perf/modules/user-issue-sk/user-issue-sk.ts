import { LitElement, TemplateResult, css, html } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import { LoggedIn } from '../../../infra-sk/modules/alogin-sk/alogin-sk';
import { AnomalySk } from '../anomaly-sk/anomaly-sk';
import { errorMessage } from '../errorMessage';
import { Status as LoginStatus } from '../../../infra-sk/modules/json';
import '../../../elements-sk/modules/icons/close-icon-sk';
import '../../../elements-sk/modules/icons/check-icon-sk';
import { UserIssue } from '../json';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';

@customElement('user-issue-sk')
export class UserIssueSk extends LitElement {
  static styles = css`
    .showLinkContainer {
      display: flex;
      align-items: center;

      .label {
        text-decoration: underline;
      }

      .linkContainer {
        color: var(--primary);
        font-size: 14px;
        display: flex;
        align-items: center;

        a {
          color: var(--primary);
          margin: 0 8px;
        }

        close-icon-sk {
          fill: var(--negative);
          cursor: pointer;
          height: 24px;
          width: 24px;
        }
      }
    }

    .new-issue-label {
      background: none;
      background: none !important;
      border: none;
      padding: 0 !important;
      color: var(--primary);
      text-decoration: underline;
      cursor: pointer;
      font-size: 14px;
    }

    .new-bug {
      align-items: center;
      background: transparent;
      border-radius: 4px;
      border: solid 1px var(--outline);
      box-shadow: none;
      color: var(--primary);
      display: inline-flex;
      fill: var(--primary);
      font-size: 16px;
      height: 24px;
      justify-content: center;
      margin: 4px 2px;
      min-width: auto;
      padding: 0 4px;
      text-align: center;
      text-transform: none;

      .icon-sk {
        font-size: 20px;
      }

      svg {
        width: 20px;
        height: 20px;
      }
    }
  `;

  // Email of the logged in user. Empty string otherwise
  @property({ attribute: true })
  user_id: string = '';

  // bug_id = 0 signifies no buganizer issue available in the database for the
  // data point. bug_id > 0 means we have an existing buganizer issue.
  @property({ attribute: true })
  bug_id: number = 0;

  // The trace_key of the selected data point. Format: ,a=1,b=2,c=3,
  @property({ attribute: true })
  trace_key: string = '';

  // Commit position of the data point
  @property({ attribute: true })
  commit_position: number = 0;

  @property({ state: true })
  issueExists = false;

  private bugComponent = 'Blink>javascript';

  connectedCallback() {
    super.connectedCallback();
    if (this.user_id !== '') {
      LoggedIn().then((status: LoginStatus) => {
        this.user_id = status.email;
      });
    }
  }

  render() {
    if (this.bug_id === -1) {
      return html``;
    }
    if (this.issueExists && this.user_id !== '') {
      return html`${this.showLinkTemplate()}`;
    }
    if (this.user_id !== '') {
      return html`<button id="add-issue-button" @click=${this.addOrFindIssue}>Add Issue</button>`;
    }
    return html``;
  }

  // If a bug is already associated with the data point show them the link.
  // The delete action for this bug will only be shown if the user is logged in.
  showLinkTemplate(): TemplateResult {
    if (this.user_id === '') {
      return html`
        <div class="showLinkContainer">
          <span class="label">Bug:</span>
          <span class="linkContainer">
            ${AnomalySk.formatBug(window.perf.bug_host_url, this.bug_id)}
          </span>
        </div>
      `;
    }

    return html`
      <div class="showLinkContainer">
        <span class="label">Bug:</span>
        <span class="linkContainer">
          ${AnomalySk.formatBug(window.perf.bug_host_url, this.bug_id)}
          <close-icon-sk @click=${this.removeIssue} ?hidden=${this.bug_id === 0}> </close-icon-sk>
        </span>
      </div>
    `;
  }

  // Makes an api call to save a buganizer issue
  // Also emits an event to refresh the existing list of user issues
  // with the newly added object.
  private async saveIssue() {
    const traceKey = this.trace_key;
    const commitPosition = this.commit_position;
    const saveIssueRequest = {
      trace_key: traceKey,
      commit_position: commitPosition,
      issue_id: this.bug_id,
    };
    const saveUserIssueResp = await fetch('/_/user_issue/save', {
      method: 'POST',
      body: JSON.stringify(saveIssueRequest),
      headers: {
        'Content-Type': 'application/json',
      },
    });

    if (!saveUserIssueResp.ok) {
      const msg = await saveUserIssueResp.text();
      errorMessage(`${saveUserIssueResp.statusText}: ${msg}`);
      return;
    }

    this.dispatchEvent(
      new CustomEvent('user-issue-changed', {
        detail: {
          trace_key: this.trace_key,
          commit_position: this.commit_position,
          bug_id: this.bug_id,
        },
        bubbles: true,
      })
    );
  }

  // Makes an api call to delete a buganizer issue
  // Also emits an event to refresh the existing list of user issues
  // with the newly deleted object.
  private async removeIssue() {
    const traceKey = this.trace_key;
    const commitPosition = this.commit_position;

    const req = {
      trace_key: traceKey,
      commit_position: commitPosition,
    };
    const resp = await fetch('/_/user_issue/delete', {
      method: 'POST',
      body: JSON.stringify(req),
      headers: {
        'Content-Type': 'application/json',
      },
    });

    if (!resp.ok) {
      const msg = await resp.text();
      errorMessage(`${resp.statusText}: ${msg}`);
      return;
    }

    // Since the bug is deleted now, we set the bug_id to 0 for showing the
    // Add buganizer issue template
    this.bug_id = 0;
    this.issueExists = false;

    this.dispatchEvent(
      new CustomEvent('user-issue-changed', {
        detail: {
          trace_key: this.trace_key,
          commit_position: this.commit_position,
          bug_id: this.bug_id,
        },
        bubbles: true,
      })
    );
  }

  /* API CALLS */
  // Makes an api call to create a new buganizer issue if the bug not exists for the trace key
  private async fileNewBug() {
    const bugTitle = this.trace_key + 'at commit position: ' + this.commit_position;

    const body = {
      trace_names: [this.trace_key],
      title: bugTitle,
      ccs: this.user_id,
      bug_component: this.bugComponent,
      assignee: this.user_id,
    };

    await fetch('/_/triage/file_bug', {
      method: 'POST',
      body: JSON.stringify(body),
      headers: {
        'Content-Type': 'application/json',
      },
    })
      .then(jsonOrThrow)
      .then((json) => {
        this.bug_id = json.bug_id;
      })
      .catch(() => {
        errorMessage(
          'File new bug request failed due to an internal server error. Please try again.'
        );
      });
  }

  private async addOrFindIssue() {
    const traceKey = this.trace_key;
    const commitPosition = this.commit_position;

    //check if the issue already exists in the database. If it does not exist, create a
    // new bug issue first, then add it to the bug issue list for the trace key
    const getIssueRequest = {
      trace_keys: traceKey,
      begin_commit_position: commitPosition,
      end_commit_position: commitPosition,
    };
    try {
      const json = await fetch('/_/user_issues', {
        method: 'POST',
        body: JSON.stringify(getIssueRequest),
        headers: {
          'Content-Type': 'application/json',
        },
      }).then(jsonOrThrow);

      const userIssues: UserIssue[] = json.UserIssues;
      let issueFound = false;
      userIssues.forEach((userIssue) => {
        if (userIssue.IssueId === this.bug_id) {
          issueFound = true;
        }
      });

      if (issueFound) {
        this.issueExists = true;
      } else {
        await this.fileNewBug();
        this.issueExists = true;
      }
      if (this.issueExists) {
        await this.saveIssue();
      }
    } catch (json) {
      const msg = await (json as Response).text();
      errorMessage(`${(json as Response).statusText}: ${msg}`);
      return;
    }
  }
}
