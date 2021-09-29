/**
 * @module modules/comments-sk
 * @description <h2><code>comments-sk</code></h2>
 * A Custom element for displaying, adding, and deleteing comments for commits, taskspecs,
 * and tasks.
 *
 * @evt data-update: void - Fires whenever this element has added or deleted a comment.
 *
 * @property allowAdd: boolean - Support adding comments, if permissions allow.
 * @property allowDelete: boolean - Support deleting comments, if permissions allow.
 * @property allowEmpty: boolean - Support empty comments.
 * @property commentData: CommentData - Comments and metadata (repo, taskspec, commit, etc).
 * @property editRights: boolean - If the logged in user has edit rights to add/delete comments.
 * @property showFlaky': boolean - Display flaky field of comments.
 * @property showIgnoreFailure: boolean - Display ignoreFailure field of comments.
 *
 */
import { define } from 'elements-sk/define';
import { html, TemplateResult } from 'lit-html';
import { errorMessage } from 'elements-sk/errorMessage';
import { $$ } from 'common-sk/modules/dom';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import {
  AddCommentRequest, Comment, GetStatusService, StatusService,
} from '../rpc';
import { escapeAndLinkify } from '../../../infra-sk/modules/linkify';

import '../../../ct/modules/input-sk';
import '../../../infra-sk/modules/human-date-sk';
import 'elements-sk/icon/check-box-icon-sk';
import 'elements-sk/icon/check-box-outline-blank-icon-sk';
import 'elements-sk/icon/delete-icon-sk';
import 'elements-sk/checkbox-sk';
import 'elements-sk/styles/buttons';

// Helper class for parent to set comments to display, as well as metadata
// about them and any comments added by the user.
export class CommentData {
  repo: string = '';

  taskId: string = '';

  taskSpec: string = '';

  commit: string = '';

  comments: Array<Comment> = [];
}

export class CommentsSk extends ElementSk {
  private static template = (el: CommentsSk) => html`
      <table class="comments">
        ${el.comments.length > 0
    ? html`
              <tr>
                <th>Time</th>
                <th>User</th>
                <th>Message</th>
                ${el.showFlaky ? html`<th>Flaky</th> ` : html``}
                ${el.showIgnoreFailure ? html`<th>Ignore Failure</th> ` : html``}
                ${el.allowDelete && el.editRights ? html`<th>Delete</th>` : html``}
              </tr>
            `
    : html``}
        ${el.comments.map(
      (c) => html`
            <tr class="comment">
              <td><human-date-sk .date=${c.timestamp} .diff=${true}></human-date-sk> ago</td>
              <td>${c.user}</td>
              <td class="commentMessage">${escapeAndLinkify(c.message)}</td>
              ${el.optionalCommentFields(c)}
            </tr>
          `,
    )}
        ${el.allowAdd && el.editRights
      ? html`
              <tr>
                <td colspan="3">
                  <input-sk value="" class="commentField" label="Comment"></input-sk>
                </td>
                ${el.showFlaky
        ? html`<td><checkbox-sk class="commentFlaky" label="Flaky"></checkbox-sk></td>`
        : html``}
                ${el.showIgnoreFailure
          ? html`<td>
                      <checkbox-sk class="commentIgnoreFailure" label="IgnoreFailure"></checkbox-sk>
                    </td>`
          : html``}
                <td>
                  <button @click=${() => el.addComment()}>Submit</button>
                </td>
              </tr>
            `
      : html``}
      </table>
    `;

  private optionalCommentFields(comment: Comment): Array<TemplateResult> {
    const ret: Array<TemplateResult> = [];
    if (this.showFlaky) {
      ret.push(
        comment.flaky
          ? html`<td><check-box-icon-sk></check-box-icon-sk></td> `
          : html`<td><check-box-outline-blank-icon-sk></check-box-outline-blank-icon-sk></td>`,
      );
    }
    if (this.showIgnoreFailure) {
      ret.push(
        comment.ignoreFailure
          ? html`<td><check-box-icon-sk></check-box-icon-sk></td> `
          : html`<td><check-box-outline-blank-icon-sk></check-box-outline-blank-icon-sk></td>`,
      );
    }
    if (this.allowDelete && this.editRights) {
      ret.push(
        html`<td @click=${() => this.deleteComment(comment)}>
          <delete-icon-sk></delete-icon-sk>
        </td>`,
      );
    }
    return ret;
  }

  private _commentData: CommentData = new CommentData();

  private _allowAdd: boolean = false;

  private _allowEmpty: boolean = false;

  private _allowDelete: boolean = false;

  private _showFlaky: boolean = false;

  private _showIgnoreFailure: boolean = false;

  private _editRights: boolean = false;

  private client: StatusService = GetStatusService();

  constructor() {
    super(CommentsSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._upgradeProperty('allowAdd');
    this._upgradeProperty('allowEmpty');
    this._upgradeProperty('allowDelete');
    this._upgradeProperty('editRights');
    this._upgradeProperty('showIngoreFailure');
    this._upgradeProperty('showFlaky');
    this._upgradeProperty('commentData');
    this._render();
  }

  _render() {
    // Don't render of we don't have any information.
    if (this.commentData) {
      super._render();
    }
  }

  get comments() {
    return this.commentData.comments;
  }

  get editRights(): boolean {
    return this._editRights;
  }

  set editRights(v: boolean) {
    this._editRights = v;
    this._render();
  }

  get allowAdd(): boolean {
    return this._allowAdd;
  }

  set allowAdd(v: boolean) {
    this._allowAdd = v;
    this._render();
  }

  get allowEmpty(): boolean {
    return this._allowEmpty;
  }

  set allowEmpty(v: boolean) {
    this._allowEmpty = v;
    this._render();
  }

  get allowDelete(): boolean {
    return this._allowDelete;
  }

  set allowDelete(v: boolean) {
    this._allowDelete = v;
    this._render();
  }

  get showIgnoreFailure(): boolean {
    return this._showIgnoreFailure;
  }

  set showIgnoreFailure(v: boolean) {
    this._showIgnoreFailure = v;
    this._render();
  }

  get showFlaky(): boolean {
    return this._showFlaky;
  }

  set showFlaky(v: boolean) {
    this._showFlaky = v;
    this._render();
  }

  set commentData(c: CommentData) {
    this._commentData = c;
    this._render();
  }

  get commentData(): CommentData {
    return this._commentData;
  }

  private deleteComment(comment: Comment) {
    this.client
      .deleteComment({
        timestamp: comment.timestamp,
        repo: this.commentData.repo,
        taskId: comment.taskId,
        taskSpec: comment.taskSpecName,
        commit: comment.commit,
      })
      .then(() => {
        // Visually get rid of the comment that was removed, and fire an event for the parent
        // element to refresh so it doesn't reappear.
        this.commentData.comments = this.comments.filter((c) => c != comment);
        this.dispatchEvent(new CustomEvent('data-update', { bubbles: true, detail: { a: 1 } }));
        this._render();
      })
      .catch(errorMessage);
  }

  private addComment() {
    const req: AddCommentRequest = {
      repo: this.commentData.repo,
      taskId: this.commentData.taskId,
      taskSpec: this.commentData.taskSpec,
      commit: this.commentData.commit,
      message: ($$('.commentField') as any).value,
      flaky: ($$('.commentFlaky', this) as any)?.checked,
      ignoreFailure: ($$('.commentIgnoreFailure', this) as any)?.checked,
    };
    const comments = this.comments;
    this.client
      .addComment(req)
      .then((resp) => {
        (<HTMLInputElement>$$('input-sk', this)).value = '';
        // If we haven't altered what's being displayed since we sent the request, add the comment
        // to our array so the UI feels snappy, even though the comment hasn't been picked up by
        // our parent yet.
        if (this.comments == comments) {
          this.comments.push({
            id: resp.timestamp!,
            timestamp: resp.timestamp,
            message: req.message,
            repo: req.repo,
            deleted: false,
            flaky: this._showFlaky,
            ignoreFailure: req.ignoreFailure,
            taskSpecName: req.taskSpec,
            taskId: req.taskId,
            commit: req.commit,
            user: 'You',
          });
        }
        this._render();
        this.dispatchEvent(new Event('data-update', { bubbles: true }));
      })
      .catch(errorMessage);
  }
}

define('comments-sk', CommentsSk);
