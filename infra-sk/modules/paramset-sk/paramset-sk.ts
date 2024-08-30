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
 * @evt paramset-checkbox-click - Generated when the checkbox for a paramset value
 *     is clicked.
 *
 *      {
 *        key: "arch",
 *        value: "x86",
 *        selected: true
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
 * @evt paramset-value-remove-click - Generated when one value for a paramset is
 *     removed. The name of the key will be sent in e.detail.key, the value in
 *     e.detail.value.
 *
 *      {
 *        key: "arch",
 *        value: "x86",
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
 * @attr {string} removable_values - If true then the cancel icon is displayed
 * next to each value in the paramset to remove the values from the set
 *
 * @attr {string} checkbox_values - If true, then the values displayed will have
 * a checkbox to let the user select/unselect the specific value.
 *
 */
import { html, TemplateResult } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { ParamSet } from '../query';
import { ElementSk } from '../ElementSk';
import '../../../elements-sk/modules/icons/add-icon-sk';
import { ToastSk } from '../../../elements-sk/modules/toast-sk/toast-sk';
import '../../../elements-sk/modules/icons/cancel-icon-sk';
import '../../../elements-sk/modules/checkbox-sk';
import '../../../elements-sk/modules/toast-sk';
import { $$ } from '../dom';

export interface ParamSetSkClickEventDetail {
  readonly key: string;
  readonly value?: string;
  readonly ctrl: boolean;
}

export interface ParamSetSkPlusClickEventDetail {
  readonly key: string;
  readonly values: string[];
}

export interface ParamSetSkRemoveClickEventDetail {
  readonly key: string;
  readonly value: string;
}

export interface ParamSetSkCheckboxClickEventDetail {
  readonly key: string;
  readonly value: string;
  readonly selected: boolean;
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
    <toast-sk duration="2000">Copied</toast-sk>
  `;

  private static titlesTemplate = (ele: ParamSetSk) =>
    ele._normalizedTitles().map((t) => html`<th>${t}</th>`);

  private static rowsTemplate = (ele: ParamSetSk) =>
    ele._sortedKeys.map((key) => ParamSetSk.rowTemplate(ele, key));

  private static rowTemplate = (ele: ParamSetSk, key: string) =>
    html` <tr>
      <th data-key=${key}>${key}</th>
      ${ParamSetSk.paramsetValuesTemplate(ele, key)}
    </tr>`;

  private static paramsetValuesTemplate = (ele: ParamSetSk, key: string) => {
    const ret: TemplateResult[] = [];
    ele._paramsets.forEach((p) =>
      ret.push(
        html`<td>
          ${ParamSetSk.paramsetValueTemplate(ele, key, p[key] || [])}
        </td>`,
        ParamSetSk.optionalPlusSign(ele, key, p),
        ParamSetSk.optionalCopyContent(ele, key, p)
      )
    );
    return ret;
  };

  private static optionalPlusSign = (
    ele: ParamSetSk,
    key: string,
    p: ParamSet
  ): TemplateResult => {
    if (!ele.clickable_plus) {
      return html``;
    }
    return html` <td>
      <add-icon-sk
        data-key=${key}
        data-values=${JSON.stringify(p[key])}></add-icon-sk>
    </td>`;
  };

  private static optionalCopyContent = (
    ele: ParamSetSk,
    key: string,
    p: ParamSet
  ): TemplateResult => {
    if (!ele.copy_content) {
      return html``;
    }
    return html` <td>
      <div
        class="icon-sk copy-content"
        @click=${() => ele.copyContent(`${key}=${p[key]}`)}>
        content_copy
      </div>
    </td>`;
  };

  private static paramsetValueTemplate = (
    ele: ParamSetSk,
    key: string,
    params: string[]
  ) => {
    // Figure out if we are down to just one checkbox being checked. If so we'll
    // want to disable that checkbox so that it can't be unchecked, otherwise
    // all the data will disappear from the display.
    let downToJustOneCheckedCheckboxForThisKey = false;

    // Count the number of unchecked values for this key.
    let numUnchecked = 0;
    const uncheckedSet = ele.unchecked.get(key);
    if (uncheckedSet !== undefined) {
      numUnchecked = uncheckedSet.size;
    }

    if (params.length - numUnchecked <= 1) {
      downToJustOneCheckedCheckboxForThisKey = true;
    }
    return params.map((value) => {
      if (ele.checkbox_values) {
        let disabled = false;
        const currentCheckboxChecked =
          uncheckedSet === undefined || !uncheckedSet.has(value);
        if (downToJustOneCheckedCheckboxForThisKey && currentCheckboxChecked) {
          disabled = true;
        }

        return html`
          <div
            class=${ele._highlighted(key, value)}
            data-key=${key}
            data-value=${value}>
            <checkbox-sk
              id="checkbox-${key}-${value}"
              name=""
              @change=${(e: MouseEvent) =>
                ele.checkboxValueClickHandler(e, key, value)}
              label=""
              checked
              ?disabled=${disabled}
              title="Select/Unselect this value from the graph.">
            </checkbox-sk>
            ${value}
          </div>
        `;
      }
      return html`<div
        class=${ele._highlighted(key, value)}
        data-key=${key}
        data-value=${value}>
        ${value} ${ParamSetSk.cancelIconTemplate(ele, key, value)}
      </div> `;
    });
  };

  private static cancelIconTemplate = (
    ele: ParamSetSk,
    key: string,
    value: string
  ): TemplateResult => {
    if (ele.removable_values) {
      return html`<cancel-icon-sk
        id="${key}-${value}-remove"
        data-key=${key}
        data-value=${value}
        title="Negative"></cancel-icon-sk>`;
    }
    return html``;
  };

  private _titles: string[] = [];

  private _paramsets: ParamSet[] = [];

  private _sortedKeys: string[] = [];

  private _highlight: { [key: string]: string } = {};

  // unchecked maps the param keys to the values that are unchecked. Note we use
  // the unchecked state because the checkboxes start off as all checked by
  // default.
  private unchecked: Map<string, Set<string>> = new Map();

  private toast: ToastSk | null = null;

  constructor() {
    super(ParamSetSk.template);
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._upgradeProperty('paramsets');
    this._upgradeProperty('highlight');
    this._upgradeProperty('clickable');
    this._upgradeProperty('clickable_values');
    this._upgradeProperty('checkbox_values');
    this._render();
    this.toast = $$<ToastSk>('toast-sk', this);
  }

  private _computeClass() {
    if (this.clickable_values) {
      return 'clickable_values';
    }
    if (this.clickable) {
      return 'clickable';
    }
    return '';
  }

  private _highlighted(key: string, value: string) {
    return this._highlight[key] === value ? 'highlight' : '';
  }

  private async copyContent(body: string) {
    await navigator.clipboard.writeText(body);
    this.toast!.show();
  }

  private fixUpDisabledStateOnRemainingCheckboxes(
    isChecked: boolean,
    key: string,
    value: string
  ) {
    // Update the unchecked status and then re-render.
    const set = this.unchecked.get(key) || new Set();
    if (isChecked) {
      set.delete(value);
    } else {
      set.add(value);
    }
    this.unchecked.set(key, set);

    this._render();
  }

  private checkboxValueClickHandler(e: MouseEvent, key: string, value: string) {
    const isChecked = (e.target! as HTMLInputElement).checked;
    const detail: ParamSetSkCheckboxClickEventDetail = {
      selected: isChecked,
      key: key,
      value: value,
    };
    this.dispatchEvent(
      new CustomEvent<ParamSetSkCheckboxClickEventDetail>(
        'paramset-checkbox-click',
        {
          detail,
          bubbles: true,
        }
      )
    );

    this.fixUpDisabledStateOnRemainingCheckboxes(isChecked, key, value);
  }

  private _click(e: MouseEvent) {
    if (
      !this.clickable &&
      !this.clickable_values &&
      !this.clickable_plus &&
      !this.removable_values
    ) {
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
      this.dispatchEvent(
        new CustomEvent<ParamSetSkClickEventDetail>('paramset-key-click', {
          detail,
          bubbles: true,
        })
      );
    } else if (t.nodeName === 'DIV') {
      const detail: ParamSetSkClickEventDetail = {
        key: t.dataset.key,
        value: t.dataset.value,
        ctrl: e.ctrlKey,
      };
      this.dispatchEvent(
        new CustomEvent<ParamSetSkClickEventDetail>(
          'paramset-key-value-click',
          {
            detail,
            bubbles: true,
          }
        )
      );
    } else if (t.nodeName === 'ADD-ICON-SK') {
      const detail: ParamSetSkPlusClickEventDetail = {
        key: t.dataset.key,
        values: JSON.parse(t.dataset.values!) as string[],
      };
      this.dispatchEvent(
        new CustomEvent<ParamSetSkPlusClickEventDetail>('plus-click', {
          detail,
          bubbles: true,
        })
      );
    } else if (t.nodeName === 'CANCEL-ICON-SK') {
      this.removeParam(t.dataset.key, t.dataset.value!);
    }
  }

  static get observedAttributes() {
    return [
      'clickable',
      'clickable_values',
      'clickable_plus',
      'copy-content',
      'checkbox_values',
    ];
  }

  /** Mirrors the clickable attribute.  */
  get clickable() {
    return this.hasAttribute('clickable');
  }

  set clickable(val) {
    if (val) {
      this.setAttribute('clickable', '');
    } else {
      this.removeAttribute('clickable');
    }
  }

  /** Mirrors the checkbox_values attribute */
  get checkbox_values() {
    return this.hasAttribute('checkbox_values');
  }

  set checkbox_values(val) {
    if (val) {
      this.setAttribute('checkbox_values', '');
    } else {
      this.removeAttribute('checkbox_values');
    }
  }

  /** Mirrors the clickable_values attribute.  */
  get clickable_values() {
    return this.hasAttribute('clickable_values');
  }

  set clickable_values(val) {
    if (val) {
      this.setAttribute('clickable_values', '');
    } else {
      this.removeAttribute('clickable_values');
    }
  }

  /** Mirrors the clickable_plus attribute.  */
  get clickable_plus() {
    return this.hasAttribute('clickable_plus');
  }

  set clickable_plus(val) {
    if (val) {
      this.setAttribute('clickable_plus', '');
    } else {
      this.removeAttribute('clickable_plus');
    }
  }

  get removable_values(): boolean {
    return this.hasAttribute('removable_values');
  }

  set removable_values(val: boolean) {
    if (val) {
      this.setAttribute('removable_values', '');
    } else {
      this.removeAttribute('removable_values');
    }
  }

  get copy_content(): boolean {
    return this.hasAttribute('copy_content');
  }

  set copy_content(val: boolean) {
    if (val) {
      this.setAttribute('copy_content', '');
    } else {
      this.removeAttribute('copy_content');
    }
  }

  attributeChangedCallback(): void {
    this._render();
  }

  /**
   * Titles for the ParamSets to display. The number of titles must match the
   * number of ParamSets, otherwise no titles will be shown.
   */
  get titles() {
    return this._titles;
  }

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
  get paramsets() {
    return this._paramsets;
  }

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
    this.unchecked = new Map();
    this._render();
  }

  /** A serialized paramtools.Params indicating the entries to highlight. */
  get highlight() {
    return this._highlight;
  }

  set highlight(val) {
    this._highlight = val;
    this._render();
  }

  removeParam(key: string, value: string) {
    // Let's remove it from the current param set
    const paramsets: ParamSet[] = [];
    this.paramsets.forEach((paramset) => {
      const values = paramset[key];
      const valIndex = values.indexOf(value);
      if (valIndex > -1) {
        values.splice(valIndex, 1);
        if (values.length === 0) {
          delete paramset[key];
        } else {
          paramset[key] = values;
        }
        paramsets.push(paramset);
      }
    });

    // Set the current paramsets to the updated value
    this.paramsets = paramsets;
    this._render();

    // Now that the state of paramsets is current,
    // let's dispatch the event to notify listeners
    const detail: ParamSetSkRemoveClickEventDetail = {
      key: key,
      value: value,
    };
    this.dispatchEvent(
      new CustomEvent<ParamSetSkRemoveClickEventDetail>(
        'paramset-value-remove-click',
        {
          detail,
          bubbles: true,
        }
      )
    );
  }
}

define('paramset-sk', ParamSetSk);
