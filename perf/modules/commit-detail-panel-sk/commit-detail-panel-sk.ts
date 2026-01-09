/**
 * @module modules/commit-detail-panel-sk
 * @description <h2><code>commit-detail-panel-sk</code></h2>
 *
 * @evt commit-selected - Event produced when a commit is selected. The
 *     the event detail contains the serialized cid.CommitDetail and
 *     a simplified description of the commit:
 *
 *     <pre>
 *     {
 *       selected: 2,
 *       description: "foo (foo@example.org) 62W Commit from foo.",
 *       commit: {
 *         author: "foo (foo@example.org)",
 *         url: "skia.googlesource.com/bar",
 *         message: "Commit from foo.",
 *         ts: 1439649751,
 *       },
 *     }
 *     </pre>
 *
 * @attr {Boolean} selectable - A boolean attribute that if true means
 *     that the commits are selectable, and when selected
 *     the 'commit-selected' event is generated.
 *
 * @attr {Number} selected - The index of the selected commit.
 */
import { html, TemplateResult } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { findParent } from '../../../infra-sk/modules/dom';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import '../commit-detail-sk';
import { Commit } from '../json';

export interface CommitDetailPanelSkCommitSelectedDetails {
  selected: number;
  description: string;
  commit: Commit;
}

export class CommitDetailPanelSk extends ElementSk {
  private _details: Commit[] = [];

  private _hide: boolean = false;

  private _trace_id: string = '';

  constructor() {
    super(CommitDetailPanelSk.template);
  }

  private static rows = (ele: CommitDetailPanelSk): TemplateResult[] => {
    if (ele.hide) {
      return [html``];
    }
    return ele._details.map(
      (item, index) => html`
        <tr data-id="${index}" ?selected="${ele._isSelected(index)}">
          <td>
            <commit-detail-sk .cid=${item} .trace_id=${ele.trace_id}></commit-detail-sk>
          </td>
        </tr>
      `
    );
  };

  private static template = (ele: CommitDetailPanelSk) => html`
    <table @click=${ele._click}>
      ${CommitDetailPanelSk.rows(ele)}
    </table>
  `;

  connectedCallback(): void {
    super.connectedCallback();
    this._upgradeProperty('details');
    this._upgradeProperty('selected');
    this._upgradeProperty('selectable');
    this._upgradeProperty('hide');
    this._upgradeProperty('trace_id');
    this._render();
  }

  attributeChangedCallback(_: string, oldValue: string, newValue: string): void {
    if (oldValue !== newValue) {
      this._render();
    }
  }

  /** An array of serialized cid.CommitDetail. */
  get details(): Commit[] {
    return this._details;
  }

  set details(val: Commit[]) {
    this._details = val;
    this._render();
  }

  get trace_id(): string {
    return this._trace_id;
  }

  set trace_id(val: string) {
    this._trace_id = val;
    this._render();
  }

  private _isSelected(index: number) {
    return this.selectable && index === this.selected;
  }

  private _click(e: MouseEvent) {
    if (!this.selectable) {
      return;
    }
    const ele = findParent(e.target as HTMLElement, 'TR');
    if (!ele) {
      return;
    }
    this.selected = +(ele.dataset.id || '0');
    if (this.selected > this._details.length - 1) {
      return;
    }
    const commit = this._details[this.selected];
    const detail = {
      selected: this.selected,
      description: `${commit.author} -  ${commit.message}`,
      commit,
    };
    this.dispatchEvent(
      new CustomEvent<CommitDetailPanelSkCommitSelectedDetails>('commit-selected', {
        detail,
        bubbles: true,
      })
    );
  }

  static get observedAttributes(): string[] {
    return ['selectable', 'selected'];
  }

  /** Mirrors the selectable attribute. */
  get selectable(): boolean {
    return this.hasAttribute('selectable');
  }

  set selectable(val: boolean) {
    if (val) {
      this.setAttribute('selectable', '');
    } else {
      this.removeAttribute('selectable');
    }
  }

  /** Mirrors the selected attribute. */
  get selected(): number {
    if (this.hasAttribute('selected')) {
      return +this.getAttribute('selected')!;
    }
    return -1;
  }

  set selected(val: number) {
    this.setAttribute('selected', `${val}`);
  }

  /** @prop hide - Do not display the commit list if true.  */
  get hide(): boolean {
    return this._hide;
  }

  set hide(val: boolean) {
    this._hide = val;
    this._render();
  }
}

define('commit-detail-panel-sk', CommitDetailPanelSk);
