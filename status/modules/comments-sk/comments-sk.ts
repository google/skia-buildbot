/**
 * @module modules/comments-sk
 * @description <h2><code>comments-sk</code></h2>
 *
 * @evt
 *
 * @property addCommentUrl: string - Endpoint for posting comments.
 * @property allowAdd: boolean - Support adding comments, if permissions allow.
 * @property allowDelete: boolean - Support deleting comments, if permissions allow.
 * @property allowEmpty: boolean - Support empty comments.
 * @property showFlaky': boolean - Display flaky field of comments.
 * @property showIgnoreFailure: boolean - Display ignoreFailure field of comments.
 *
 */
import { define } from 'elements-sk/define';
import { html, TemplateResult } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { Comment } from '../rpc';
import { escapeAndLinkify } from '../../../infra-sk/modules/linkify';

import '../../../ct/modules/input-sk';
import '../../../infra-sk/modules/human-date-sk';
import 'elements-sk/icon/check-box-icon-sk';
import 'elements-sk/icon/check-box-outline-blank-icon-sk';
import 'elements-sk/icon/delete-icon-sk';
import 'elements-sk/checkbox-sk';
import 'elements-sk/styles/buttons';

export class CommentsSk extends ElementSk {
  private static template = (el: CommentsSk) =>
    html`
      <table class="comments">
        ${el.headings()}
        ${el.comments.map(
          (c) => html`
            <tr class="comment">
              <td><human-date-sk .date=${c.timestamp} .diff=${true}></human-date-sk> ago</td>
              <td>${c.user}</td>
              <td class="commentMessage">${escapeAndLinkify(c.message)}</td>
              ${el.optionalCommentFields(c)}
            </tr>
          `
        )}
        ${el.addCommentTemplate()}
      </table>
    `;

  private headings(): TemplateResult {
    return this.comments.length === 0
      ? html``
      : html`
          <tr>
            <th>Time</th>
            <th>User</th>
            <th>Message</th>
            ${this.showFlaky ? html`<th>Flaky</th> ` : html``}
            ${this.showIgnoreFailure ? html`<th>Ignore Failure</th> ` : html``}
          </tr>
        `;
  }
  private optionalCommentFields(comment: Comment): Array<TemplateResult> {
    const ret: Array<TemplateResult> = [];
    if (this.showFlaky) {
      ret.push(
        comment.flaky
          ? html`<td><check-box-icon-sk></check-box-icon-sk></td> `
          : html`<td><check-box-outline-blank-icon-sk></check-box-outline-blank-icon-sk></td>`
      );
    }
    if (this.showIgnoreFailure) {
      ret.push(
        comment.ignoreFailure
          ? html`<td><check-box-icon-sk></check-box-icon-sk></td> `
          : html`<td><check-box-outline-blank-icon-sk></check-box-outline-blank-icon-sk></td>`
      );
    }
    if (this.allowDelete && this.editRights) {
      ret.push(
        html`<td @click=${() => this.deleteComment(comment.id)}>
          <delete-icon-sk></delete-icon-sk>
        </td>`
      );
    }
    return ret;
  }

  private addCommentTemplate(): TemplateResult {
    return this.allowAdd && this.editRights
      ? html`
          <tr>
            <td colspan="3">
              <input-sk value="" class="commentField" label="Comment"></input-sk>
            </td>
            ${this.showFlaky
              ? html`<td><checkbox-sk class="checkFlaky"></checkbox-sk>Flaky</td>`
              : html``}
            ${this.showIgnoreFailure
              ? html`<td><checkbox-sk class="checkIgnoreFailure"></checkbox-sk>IgnoreFailure</td>`
              : html``}
            <td>
              <button>Submit</button>
            </td>
          </tr>
        `
      : html``;
  }

  private _comments: Array<Comment> = [];
  private _addCommentUrl: string = '';
  private _allowAdd: boolean = false;
  private _allowEmpty: boolean = false;
  private _allowDelete: boolean = false;
  private _showFlaky: boolean = false;
  private _showIgnoreFailure: boolean = false;
  // TODO(westont): Temporary, default to false once we can mock out login-sk for testing.
  private editRights: boolean = true;

  constructor() {
    super(CommentsSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._upgradeProperty('add_comment_url');
    this._render();
  }

  get add_comment_url(): string {
    return this._addCommentUrl;
  }

  set add_comment_url(v: string) {
    this._addCommentUrl = v;
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

  set comments(c: Array<Comment>) {
    this._comments = c;
    this._render();
  }

  get comments(): Array<Comment> {
    return this._comments;
  }

  private deleteComment(id: string) {
    // TODO(westont): implement this.
  }
  private addComment() {
    // TODO(westont): implement this.
  }
}

define('comments-sk', CommentsSk);
