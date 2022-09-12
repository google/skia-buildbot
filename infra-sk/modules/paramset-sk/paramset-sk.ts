/**
 * @module module/paramset-sk
 * @description <h2><code>paramset-sk</code></h2>
 *
 * The paramset-sk element displays a paramset and generates events as the
 * params and labels are clicked.
 *
 * @evt paramset-key-click - Generated when the key for a paramset is clicked.
 *     The name of the key will be sent in e.detail.key. The value of
 *     e.detail.ctrl is true if the control key was pressed when clicking.
 *
 *      {
 *        key: "arch",
 *        ctrl: false,
 *      }
 *
 * @evt paramset-key-value-click - Generated when one value for a paramset is
 *     clicked. The name of the key will be sent in e.detail.key, the value in
 *     e.detail.value. The value of e.detail.ctrl is true if the control key was
 *     pressed when clicking.
 *
 *      {
 *        key: "arch",
 *        value: "x86",
 *        ctrl: false,
 *      }
 *
 * @evt plus-click - Generated when the plus sign is clicked. The element must
 *     have the 'clickable_plus' attribute set. The details of the event
 *     contains both the key and the values for the row, for example:
 *
 *      {
 *        key: "arch",
 *        values" ["x86", "risc-v"],
 *      }
 *
 * @attr {string} clickable - If true then keys and values look like they are
 *     clickable i.e. via color, text-decoration, and cursor. If clickable is
 *     false then this element won't generate the events listed below, and the
 *     keys and values are not styled to look clickable. Setting both clickable
 *     and clickable_values is unsupported.
 *
 * @attr {string} clickable_values - If true then only the values are clickable.
 *     Setting both clickable and clickable_values is unsupported.
 *
 * @attr {string} clickable_plus - If true then a plus sign is added to every
 * row in the right hand column, that when pressed emits the plus-click event
 * that contains the key and values for that row.
 *
 */
import { define } from 'elements-sk/define';
import { html, TemplateResult } from 'lit-html';
import { ParamSet } from 'common-sk/modules/query';
import { ElementSk } from '../ElementSk';
import 'elements-sk/icon/add-icon-sk';

export interface ParamSetSkClickEventDetail {
  readonly key: string;
  readonly value?: string;
  readonly ctrl: boolean;
}

export interface ParamSetSkPlusClickEventDetail {
  readonly key: string;
  readonly values: string[];
}

export class ParamSetSk extends ElementSk {
  private static template = (ele: ParamSetSk) => html`
  <table @click=${ele._click} class=${ele._computeClass()}>
    <tbody>
      <tr>
        <th></th>
        ${ParamSetSk.titlesTemplate(ele)}
      </tr>
      ${ParamSetSk.rowsTemplate(ele)}
    </tbody>
  </table>
`;

  private static titlesTemplate =
    (ele: ParamSetSk) => ele._normalizedTitles().map((t) => html`<th>${t}</th>`);

  private static rowsTemplate =
    (ele: ParamSetSk) => ele._sortedKeys.map((key) => ParamSetSk.rowTemplate(ele, key));

  private static rowTemplate = (ele: ParamSetSk, key: string) => html`
    <tr>
      <th data-key=${key}>${key}</th>
      ${ParamSetSk.paramsetValuesTemplate(ele, key)}
    </tr>`;

  private static paramsetValuesTemplate =
    (ele: ParamSetSk, key: string) => {
      const ret: TemplateResult[] = [];
      ele._paramsets.forEach(
        (p) => ret.push(
          html`<td>${ParamSetSk.paramsetValueTemplate(ele, key, p[key] || [])}</td>`,
          ParamSetSk.optionalPlusSign(ele, key, p),
        ),
      );
      return ret;
    };

  private static optionalPlusSign = (ele: ParamSetSk, key: string, p: ParamSet): TemplateResult => {
    if (!ele.clickable_plus) {
      return html``;
    }
    return html`
    <td><add-icon-sk
      data-key=${key}
      data-values=${JSON.stringify(p[key])}
    ></add-icon-sk></td>`;
  }

  private static paramsetValueTemplate =
    (ele: ParamSetSk, key: string, params: string[]) => params.map((value) => html`<div class=${ele._highlighted(key, value)}
                                      data-key=${key}
                                      data-value=${value}>${value}</div>`);

  private _titles: string[] = [];

  private _paramsets: ParamSet[] = [];

  private _sortedKeys: string[] = [];

  private _highlight: { [key: string]: string } = {};

  constructor() {
    super(ParamSetSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._upgradeProperty('paramsets');
    this._upgradeProperty('highlight');
    this._upgradeProperty('clickable');
    this._upgradeProperty('clickable_values');
    this._render();
  }

  private _computeClass() {
    if (this.clickable_values) {
      return 'clickable_values';
    } if (this.clickable) {
      return 'clickable';
    }
    return '';
  }

  private _highlighted(key: string, value: string) {
    return this._highlight[key] === value ? 'highlight' : '';
  }

  private _click(e: MouseEvent) {
    if (!this.clickable && !this.clickable_values && !this.clickable_plus) {
      return;
    }
    const t = e.target as HTMLElement;
    if (!t.dataset.key) {
      return;
    }
    if (t.nodeName === 'TH') {
      if (!this.clickable) {
        return;
      }
      const detail: ParamSetSkClickEventDetail = {
        key: t.dataset.key,
        ctrl: e.ctrlKey,
      };
      this.dispatchEvent(new CustomEvent<ParamSetSkClickEventDetail>('paramset-key-click', {
        detail,
        bubbles: true,
      }));
    } else if (t.nodeName === 'DIV') {
      const detail: ParamSetSkClickEventDetail = {
        key: t.dataset.key,
        value: t.dataset.value,
        ctrl: e.ctrlKey,
      };
      this.dispatchEvent(new CustomEvent<ParamSetSkClickEventDetail>('paramset-key-value-click', {
        detail,
        bubbles: true,
      }));
    } else if (t.nodeName === 'ADD-ICON-SK') {
      const detail: ParamSetSkPlusClickEventDetail = {
        key: t.dataset.key,
        values: JSON.parse(t.dataset.values!) as string[],
      };
      this.dispatchEvent(new CustomEvent<ParamSetSkPlusClickEventDetail>('plus-click', {
        detail,
        bubbles: true,
      }));
    }
  }

  static get observedAttributes() {
    return ['clickable', 'clickable_values', 'clickable_plus'];
  }

  /** Mirrors the clickable attribute.  */
  get clickable() { return this.hasAttribute('clickable'); }

  set clickable(val) {
    if (val) {
      this.setAttribute('clickable', '');
    } else {
      this.removeAttribute('clickable');
    }
  }

  /** Mirrors the clickable_values attribute.  */
  get clickable_values() { return this.hasAttribute('clickable_values'); }

  set clickable_values(val) {
    if (val) {
      this.setAttribute('clickable_values', '');
    } else {
      this.removeAttribute('clickable_values');
    }
  }

  /** Mirrors the clickable_plus attribute.  */
  get clickable_plus() { return this.hasAttribute('clickable_plus'); }

  set clickable_plus(val) {
    if (val) {
      this.setAttribute('clickable_plus', '');
    } else {
      this.removeAttribute('clickable_plus');
    }
  }

  attributeChangedCallback() {
    this._render();
  }

  /**
   * Titles for the ParamSets to display. The number of titles must match the
   * number of ParamSets, otherwise no titles will be shown.
   */
  get titles() { return this._titles; }

  set titles(val) {
    this._titles = val;
    this._render();
  }

  // Returns the titles specified by the user, or an empty title for each paramset
  // if the number of specified titles and the number of paramsets don't match.
  private _normalizedTitles() {
    if (this._titles.length === this._paramsets.length) {
      return this._titles;
    }
    return new Array<string>(this._paramsets.length).fill('');
  }

  /** ParamSets to display. */
  get paramsets() { return this._paramsets; }

  set paramsets(val) {
    this._paramsets = val;

    // Compute a rolled up set of all parameter keys across all paramsets.
    const allKeys = new Set<string>();
    this._paramsets.forEach((p) => {
      Object.keys(p).forEach((key) => {
        allKeys.add(key);
      });
    });
    this._sortedKeys = Array.from(allKeys).sort();
    this._render();
  }

  /** A serialized paramtools.Params indicating the entries to highlight. */
  get highlight() { return this._highlight; }

  set highlight(val) {
    this._highlight = val;
    this._render();
  }
}

define('paramset-sk', ParamSetSk);
