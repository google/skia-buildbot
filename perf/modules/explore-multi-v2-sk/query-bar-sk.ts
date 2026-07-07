import { LitElement, html, css, PropertyValues } from 'lit';
import { customElement, property, state, query } from 'lit/decorators.js';
import { repeat } from 'lit/directives/repeat.js';
import { scoreParamAny, fuzzyScore, MAX_SUGGESTIONS } from './fuzzy';
import './multi-select-sk';

import '@material/web/textfield/outlined-text-field';
import '@material/web/iconbutton/icon-button';
import '@material/web/icon/icon';
import {
  MultiSelectSelectionEventDetail,
  MultiSelectSetSelectionEventDetail,
  MultiSelectReplaceEventDetail,
} from './multi-select-sk';

interface PillRect {
  index: number;
  left: number;
  right: number;
  top: number;
  bottom: number;
}

function calculatePillFocusIndex(
  clientX: number,
  clientY: number,
  containerRect: DOMRect | undefined,
  pillRects: PillRect[]
): number {
  const isAbove = containerRect && clientY < containerRect.top;

  if (isAbove) {
    return 0;
  } else {
    let lastPillInRow: PillRect | null = null;
    for (const pr of pillRects) {
      if (clientY >= pr.top && clientY <= pr.bottom) {
        lastPillInRow = pr;
        if (clientX <= pr.right) {
          return pr.index;
        }
      }
    }

    if (lastPillInRow) {
      return lastPillInRow.index;
    }

    if (pillRects.length > 0) {
      return pillRects[pillRects.length - 1].index;
    }
  }
  return -1;
}

export interface Suggestion {
  params: { key: string; value: string; count?: number }[];
  score: number;
  count?: number;
  countIsLowerBound?: boolean;
}

export interface QueryAddEventDetail {
  key: string;
  value: string;
}

export interface QueryRemoveEventDetail {
  key: string;
  value: string;
}

export interface QuerySetSelectedEventDetail {
  key: string;
  values: string[];
}

export interface QueryRemoveKeyEventDetail {
  key: string;
}

export interface QueryDiffBaseEventDetail {
  key: string;
  value: string;
}

export interface FormulaOption {
  id: string;
  name: string;
  desc: string;
  category: 'prep' | 'agg' | 'trans';
}

// TODO(b/381280000): Consider generating this list from go/calc/parser.go
// so that the backend supported functions remain the single source of truth.
export const FORMULA_OPTIONS: FormulaOption[] = [
  { id: 'fill', name: 'fill()', desc: 'Fill missing data points', category: 'prep' },
  { id: 'iqrr', name: 'iqrr()', desc: 'Remove outliers (IQR)', category: 'prep' },

  { id: 'ave', name: 'ave()', desc: 'Average matching traces', category: 'agg' },
  { id: 'sum', name: 'sum()', desc: 'Sum matching traces', category: 'agg' },
  { id: 'min', name: 'min()', desc: 'Minimum value across traces', category: 'agg' },
  { id: 'max', name: 'max()', desc: 'Maximum value across traces', category: 'agg' },
  { id: 'count', name: 'count()', desc: 'Count non-missing values', category: 'agg' },
  { id: 'geo', name: 'geo()', desc: 'Geometric mean across traces', category: 'agg' },

  { id: 'norm', name: 'norm()', desc: 'Normalize (mean 0, stddev 1)', category: 'trans' },
  { id: 'log', name: 'log()', desc: 'Base-10 logarithm', category: 'trans' },
  { id: 'step', name: 'step()', desc: 'Step detection ratio', category: 'trans' },
  { id: 'scale_by_ave', name: 'scale_by_ave()', desc: 'Scale by 1/average', category: 'trans' },
  { id: 'trace_ave', name: 'trace_ave()', desc: 'Trace mean timeline', category: 'trans' },
  {
    id: 'trace_stddev',
    name: 'trace_stddev()',
    desc: 'Trace standard deviation',
    category: 'trans',
  },
  { id: 'trace_cov', name: 'trace_cov()', desc: 'Coefficient of variation', category: 'trans' },
];

@customElement('query-bar-sk')
export class QueryBarSk extends LitElement {
  private static readonly DRAG_THRESHOLD_PX = 5;

  public pendingPillSelection: string | null = null;

  @property({ type: Object }) query: Record<string, string[]> = {};

  @property({ type: Array }) availableParams: { key: string; value: string; count?: number }[] = [];

  @property({ type: Object }) optionsByKey: Record<string, { value: string; count: number }[]> = {};

  @property({ type: Array }) includeParams: string[] = [];

  @property({ type: Object }) defaults: any = null;

  @property({ type: Object }) splitKeys: Set<string> = new Set();

  @property({ type: Boolean }) showRemoveQueryButton = false;

  @property({ type: String }) formula = 'none';

  @property({ type: Array }) pipeline: string[] = [];

  @property({ type: Array }) externalSuggestions: Suggestion[] | null = null;

  @state() private _showFormulaPopover = false;

  @property({ type: Boolean }) loading = false;

  @state() private _inputValue = '';

  @state() private _isOpen = false;

  @state() private _focusedIndex = 0;

  @state() private _suggestions: Suggestion[] = [];

  @state() private _selectedCategory: string | null = null;

  @state() private _selectionAnchor: number | null = null;

  @state() private _selectionFocus: number | null = null;

  @state() private _isDragging = false;

  @state() private _dragStartPos: { x: number; y: number } | null = null;

  @state() private _startedInInput = false;

  @state() private _pillRects: PillRect[] = [];

  @state() private _canSelectPills = false;

  @state() private _openPillIndex: number | null = null;

  @state() private _selectedPills: Set<number> = new Set();

  private _isToTheLeft = false;

  private _savedEnd = 0;

  private _currentMouseX = 0;

  private _currentMouseY = 0;

  private _hasDragged = false;

  @query('.query-input') private _inputElement?: HTMLInputElement;

  @query('.suggestions-dropdown') private _listElement?: HTMLDivElement;

  static styles = css`
    :host {
      display: block;
      font-family: Roboto, sans-serif;
    }

    .query-bar-container {
      background: var(--md-sys-color-surface);
      border: 1px solid var(--md-sys-color-outline);
      border-radius: 4px;
      padding: 4px 8px;
      display: flex;
      flex-wrap: wrap;
      align-items: center;
      cursor: text;
      box-shadow: none;
      transition: all 0.2s ease;
    }

    .query-bar-container:focus-within {
      border-color: var(--md-sys-color-primary);
      box-shadow: 0 0 0 1px var(--md-sys-color-primary);
    }

    .query-pills {
      display: flex;
      flex-wrap: wrap;
      gap: 4px;
      flex: 1;
      align-items: center;
    }

    .query-input-wrapper {
      flex: 1;
      position: relative;
      min-width: 150px;
      align-self: center;
      display: flex;
      align-items: center;
      padding: 2px 0;
    }

    .query-input {
      --md-outlined-text-field-outline-color: transparent;
      --md-outlined-text-field-focus-outline-color: transparent;
      --md-outlined-text-field-container-color: transparent;
      --md-outlined-text-field-input-text-color: var(--on-surface);

      width: 100%;
      flex: 1;
      font-size: 13px;
    }

    .query-input::placeholder {
      color: var(--md-sys-color-on-surface-variant);
      font-style: normal;
    }

    .input-spinner {
      width: 14px;
      height: 14px;
      border: 2px solid var(--md-sys-color-outline-variant);
      border-top-color: var(--md-sys-color-primary);
      border-radius: 50%;
      animation: spin 0.8s linear infinite;
      margin-right: 6px;
    }

    @keyframes spin {
      to {
        transform: rotate(360deg);
      }
    }

    .suggestions-dropdown {
      position: absolute;
      top: 100%;
      left: 0;
      min-width: 350px;
      max-height: 450px;
      overflow-y: auto;
      background: var(--md-sys-color-surface);
      border: 1px solid var(--md-sys-color-outline-variant);
      border-radius: 4px;
      box-shadow: 0 4px 12px color-mix(in srgb, var(--transparent-overlay) 20%, transparent);
      z-index: 1000;
      margin-top: 4px;
      padding: 4px 0;
    }

    .suggestion-item {
      padding: 6px 12px;
      cursor: pointer;
      display: flex;
      flex-wrap: wrap;
      align-items: center;
      gap: 6px;
      font-family: monospace;
      font-size: 12px;
      transition: background-color 0.2s;
      color: var(--on-surface);
    }

    .suggestion-item.info {
      color: var(--md-sys-color-on-surface-variant);
      font-style: italic;
      cursor: default;
    }

    .suggestion-item:hover,
    .suggestion-item.focused {
      background-color: color-mix(in srgb, var(--on-surface) 10%, transparent);
    }

    .suggestion-pill {
      display: inline-flex;
      align-items: center;
      background: color-mix(in srgb, var(--on-surface) 15%, transparent);
      border-radius: 10px;
      padding: 1px 8px;
      font-size: 11px;
      max-width: 100%;
    }

    .s-key {
      color: var(--md-sys-color-on-surface-variant);
    }

    .s-sep {
      color: var(--md-sys-color-outline);
      margin: 0 2px;
    }

    .s-val {
      color: var(--on-surface);
      font-weight: 500;
    }

    .s-count.right {
      margin-left: auto;
      color: var(--md-sys-color-on-surface-variant);
      font-style: italic;
      font-size: 10px;
    }

    .s-count.stale {
      opacity: 0.5;
    }

    .query-actions {
      display: flex;
      align-items: center;
      gap: 4px;
      margin-left: auto;
    }

    .qb-clone-query-btn,
    .qb-remove-query-btn {
      background: none;
      border: none;
      color: var(--md-sys-color-on-surface-variant);
      font-size: 16px;
      cursor: pointer;
      padding: 0 4px;
      display: flex;
      align-items: center;
      justify-content: center;
      border-radius: 50%;
      width: 24px;
      height: 24px;
      transition: background-color 0.2s;
    }

    .qb-clone-query-btn:hover,
    .qb-remove-query-btn:hover {
      background-color: var(--md-sys-color-surface-container-highest);
    }

    .qb-formula-select {
      background: var(
        --surface-variant,
        var(--surface, var(--md-sys-color-surface-container-high))
      );
      border: 1px solid var(--outline, var(--border, var(--md-sys-color-outline-variant)));
      color: var(--on-surface, var(--on-background, var(--md-sys-color-on-surface)));
      border-radius: 6px;
      padding: 3px 8px;
      font-size: 11px;
      font-weight: 600;
      cursor: pointer;
      outline: none;
      height: 28px;
      box-sizing: border-box;
    }

    .qb-formula-select option {
      background-color: var(--surface, var(--background, var(--md-sys-color-surface)));
      color: var(--on-surface, var(--md-sys-color-on-surface));
    }

    .qb-formula-select:hover,
    .qb-formula-select:focus {
      border-color: var(--primary, var(--md-sys-color-primary));
    }

    .qb-pipeline-container {
      display: flex;
      align-items: center;
      gap: 6px;
      background: var(
        --md-sys-color-surface-container-low,
        var(--surface-container-low, var(--background))
      );
      border: 1px solid var(--md-sys-color-outline-variant, var(--border));
      border-radius: 8px;
      padding: 3px 8px;
      position: relative;
    }

    .qb-pipeline-step {
      font-size: 11px;
      font-weight: 600;
      border-radius: 4px;
      padding: 2px 8px;
      display: inline-flex;
      align-items: center;
      gap: 6px;
      background: color-mix(in srgb, var(--md-sys-color-primary, var(--primary)) 20%, transparent);
      color: var(--md-sys-color-primary, var(--primary));
      border: 1px solid
        color-mix(in srgb, var(--md-sys-color-primary, var(--primary)) 40%, transparent);
    }

    .qb-pipeline-arrow {
      color: var(--md-sys-color-on-surface-variant);
      font-size: 10px;
    }

    .qb-pipeline-remove {
      font-size: 14px;
      line-height: 1;
      color: var(--md-sys-color-on-surface-variant);
      cursor: pointer;
      margin-left: 2px;
    }

    .qb-pipeline-remove:hover {
      color: var(--md-sys-color-error, var(--error));
    }

    .qb-add-formula-btn {
      background: color-mix(in srgb, var(--on-surface) 10%, transparent);
      border: 1px dashed color-mix(in srgb, var(--on-surface) 30%, transparent);
      color: var(--on-surface);
      font-size: 11px;
      font-weight: 500;
      border-radius: 4px;
      padding: 3px 8px;
      cursor: pointer;
      display: inline-flex;
      align-items: center;
      gap: 4px;
      transition: all 0.2s ease;
    }

    .qb-add-formula-btn:hover {
      background: color-mix(in srgb, var(--primary) 20%, transparent);
      border-color: var(--primary);
      color: var(--primary);
    }

    .qb-formula-popover {
      position: absolute;
      right: 0;
      top: 42px;
      width: 320px;
      max-width: calc(100vw - 32px);
      max-height: 380px;
      overflow-y: auto;
      background: var(--surface);
      border: 1px solid color-mix(in srgb, var(--on-surface) 15%, transparent);
      border-radius: 8px;
      box-shadow: none;
      padding: 14px;
      z-index: 2000;
    }

    .qb-formula-popover-header {
      font-size: 11px;
      font-weight: 700;
      text-transform: uppercase;
      letter-spacing: 0.05em;
      color: var(--primary);
      margin: 10px 0 6px;
    }

    .qb-formula-popover-header:first-child {
      margin-top: 0;
    }

    .qb-formula-grid {
      display: grid;
      grid-template-columns: 1fr;
      gap: 4px;
    }

    .qb-formula-item {
      display: flex;
      flex-direction: column;
      align-items: flex-start;
      padding: 8px 10px;
      background: color-mix(in srgb, var(--on-surface) 5%, transparent);
      border: 1px solid color-mix(in srgb, var(--on-surface) 12%, transparent);
      border-radius: 6px;
      cursor: pointer;
      text-align: left;
      transition: all 0.2s ease;
    }

    .qb-formula-item:hover {
      background: color-mix(in srgb, var(--primary) 15%, transparent);
      border-color: var(--primary);
    }

    .qb-formula-item-name {
      font-size: 12px;
      font-weight: 600;
      color: var(--on-surface);
      font-family: monospace;
    }

    .qb-formula-item-desc {
      font-size: 11px;
      color: color-mix(in srgb, var(--on-surface) 75%, transparent);
      margin-top: 2px;
    }
  `;

  connectedCallback() {
    super.connectedCallback();
    document.addEventListener('mousedown', this._handleClickOutside);
    document.addEventListener('selectionchange', this._handleSelectionChange);
  }

  disconnectedCallback() {
    document.removeEventListener('mousedown', this._handleClickOutside);
    document.removeEventListener('selectionchange', this._handleSelectionChange);
    super.disconnectedCallback();
  }

  protected updated(changedProperties: PropertyValues) {
    if (
      changedProperties.has('query') ||
      changedProperties.has('availableParams') ||
      changedProperties.has('optionsByKey') ||
      changedProperties.has('_inputValue') ||
      changedProperties.has('externalSuggestions')
    ) {
      this._updateSuggestions();
    }

    if (changedProperties.has('_isOpen') && this._isOpen) {
      this._scrollFocusedIntoView();
    }
  }

  private _handleClickOutside = (e: MouseEvent) => {
    const path = e.composedPath();
    if (this._isOpen && !path.includes(this)) {
      this._isOpen = false;
    }
    if (this._showFormulaPopover && !path.includes(this)) {
      this._showFormulaPopover = false;
    }
  };

  private _sortKeys(keys: string[]): string[] {
    const order = this.includeParams || [];
    return [...keys].sort((a, b) => {
      const indexA = order.indexOf(a);
      const indexB = order.indexOf(b);
      if (indexA !== -1 && indexB !== -1) return indexA - indexB;
      if (indexA !== -1) return -1;
      if (indexB !== -1) return 1;
      return a.localeCompare(b);
    });
  }

  private _getAvailableKeys() {
    const keys = Object.keys(this.optionsByKey).filter((k) => !this.query[k]);
    return this._sortKeys(keys);
  }

  private _getPlaceholderTip() {
    const keys = this._getAvailableKeys();
    if (keys.length === 0) return 'All filters applied.';
    return `Filter by: ${[...keys].sort().join(', ')}`;
  }

  private _getAllParams(): { key: string; value: string; count: number }[] {
    const apMap = new Map<string, number>();
    (this.availableParams || []).forEach((p) => {
      apMap.set(`${p.key}=${p.value}`, p.count ?? 0);
    });

    const list: { key: string; value: string; count: number }[] = [];
    Object.keys(this.optionsByKey || {}).forEach((k) => {
      (this.optionsByKey[k] || []).forEach((o) => {
        const count = apMap.get(`${k}=${o.value}`) ?? 0;
        list.push({ key: k, value: o.value, count });
      });
    });
    return list;
  }

  private _updateSuggestions() {
    const trimmed = this._inputValue.trim();
    if (!trimmed) {
      this.externalSuggestions = null;
      if (this._isOpen) {
        if (!this._selectedCategory) {
          const keys = Object.keys(this.optionsByKey).filter((k) => !this.query[k]);
          this._suggestions = keys
            .map((k) => {
              let totalCount = 0;
              if (this.availableParams && this.availableParams.length > 0) {
                totalCount = this.availableParams
                  .filter((p) => p.key === k)
                  .reduce((sum, p) => sum + (p.count ?? 0), 0);
              } else {
                const options = this.optionsByKey[k] || [];
                totalCount = options.reduce((sum, o) => sum + o.count, 0);
              }
              return {
                params: [{ key: k, value: '' }],
                score: 100,
                count: totalCount,
                countIsLowerBound: false,
              };
            })
            .filter((s) => s.count > 0);

          // Sort by include_params order, then by count descending
          const order = this.includeParams || [];
          this._suggestions.sort((a, b) => {
            const keyA = a.params[0].key;
            const keyB = b.params[0].key;
            const indexA = order.indexOf(keyA);
            const indexB = order.indexOf(keyB);

            if (indexA !== -1 && indexB !== -1) return indexA - indexB;
            if (indexA !== -1) return -1;
            if (indexB !== -1) return 1;

            const aCount = a.count ?? 0;
            const bCount = b.count ?? 0;
            if (bCount !== aCount) return bCount - aCount;
            return keyA.localeCompare(keyB);
          });
        } else {
          const cat = this._selectedCategory;
          const apMap = new Map<string, number>();
          if (this.availableParams && this.availableParams.length > 0) {
            this.availableParams.forEach((p) => {
              if (p.key === cat) apMap.set(p.value, p.count ?? 0);
            });
          }

          const options = this.optionsByKey[cat] || [];
          this._suggestions = options
            .map((o) => ({
              params: [{ key: cat, value: o.value }],
              score: 100,
              count:
                this.availableParams && this.availableParams.length > 0
                  ? (apMap.get(o.value) ?? 0)
                  : o.count,
              countIsLowerBound: false,
            }))
            .filter((s) => s.count > 0);
        }
      } else {
        this._suggestions = [];
      }
      return;
    }

    if (this.externalSuggestions !== null && this.externalSuggestions.length > 0) {
      this._suggestions = this.externalSuggestions;
      return;
    }

    console.log(
      '[_updateSuggestions] trimmed:',
      trimmed,
      'availableParams count:',
      this.availableParams.length
    );

    if (trimmed.includes(' ')) {
      const tokens = trimmed
        .split(' ')
        .map((s) => s.trim())
        .filter(Boolean);
      if (tokens.length === 0) {
        this._suggestions = [];
        return;
      }

      const scored = this._getAllParams()
        .filter(
          (p) => !this.query[p.key]?.includes(p.value) && (p.count === undefined || p.count > 0)
        )
        .map((p) => {
          let bestScore = -Infinity;
          for (const token of tokens) {
            const s = scoreParamAny(p, token);
            if (s > bestScore) {
              bestScore = s;
            }
          }
          return { p, score: bestScore };
        });

      const matches = scored.filter((s) => s.score > -Infinity);
      matches.sort((a, b) => b.score - a.score);

      this._suggestions = matches.slice(0, MAX_SUGGESTIONS).map((s) => ({
        params: [s.p],
        score: s.score,
        count: s.p.count,
        countIsLowerBound: false,
      }));
      return;
    }

    const eqIdx = trimmed.indexOf('=');
    const vPartCheck = eqIdx !== -1 ? trimmed.substring(eqIdx + 1) : trimmed;
    const hasGlobChar =
      vPartCheck.includes('*') || vPartCheck.includes('?') || vPartCheck.includes(',');
    const isGlobSearch = hasGlobChar;

    if (isGlobSearch) {
      const kPart = eqIdx !== -1 ? trimmed.substring(0, eqIdx) : '';
      const vPart = eqIdx !== -1 ? trimmed.substring(eqIdx + 1) : trimmed;

      if (!vPart) {
        this._suggestions = [];
        return;
      }

      try {
        const parts = vPart
          .split(',')
          .map((s) => s.trim())
          .filter(Boolean);
        const regexes = parts.map((part) => {
          const escaped = part.replace(/[.+^${}()|[\]\\]/g, '\\$&');
          const pattern = '^' + escaped.replace(/\*/g, '.*').replace(/\?/g, '.') + '$';
          return new RegExp(pattern, 'i');
        });

        const globSuggestions: Suggestion[] = [];

        for (const key of Object.keys(this.optionsByKey)) {
          if (this.query[key]) continue;

          const keyScore = kPart ? fuzzyScore(key, kPart) : 0;
          if (kPart && keyScore === -Infinity) continue;

          let totalCount = 0;
          let matchesAnything = false;

          const apMap = new Map<string, number>();
          if (this.availableParams && this.availableParams.length > 0) {
            this.availableParams.forEach((p) => {
              if (p.key === key) apMap.set(p.value, p.count ?? 0);
            });
          }

          for (const opt of this.optionsByKey[key]) {
            const matches = regexes.some((r) => r.test(opt.value));
            if (matches) {
              const c =
                this.availableParams && this.availableParams.length > 0
                  ? (apMap.get(opt.value) ?? 0)
                  : opt.count;
              if (c > 0) {
                matchesAnything = true;
                totalCount += c;
              }
            }
          }

          if (matchesAnything) {
            globSuggestions.push({
              params: [{ key, value: vPart }],
              score: keyScore + 1000,
              count: totalCount,
              countIsLowerBound: false,
            });
          }
        }

        globSuggestions.sort((a, b) => b.score - a.score);
        this._suggestions = globSuggestions.slice(0, MAX_SUGGESTIONS);
        return;
      } catch (_e) {
        this._suggestions = [];
        return;
      }
    }

    const scored = this._getAllParams()
      .filter(
        (p) => !this.query[p.key]?.includes(p.value) && (p.count === undefined || p.count > 0)
      )
      .map((p) => {
        return { p, score: scoreParamAny(p, trimmed) };
      });

    const matches = scored.filter((s) => s.score > -Infinity);
    console.log('[_updateSuggestions] matches count:', matches.length);

    this._boostPriorityScores(matches);
    matches.sort((a, b) => b.score - a.score);

    this._suggestions = matches.slice(0, MAX_SUGGESTIONS).map((s) => ({
      params: [s.p],
      score: s.score,
      count: s.p.count,
      countIsLowerBound: false,
    }));

    void this._fetchCountsForSuggestions();
  }

  private _boostPriorityScores(matches: { p: { key: string; value: string }; score: number }[]) {
    const priorities = this.defaults?.default_trigger_priority;
    if (!priorities) return;

    matches.forEach((m) => {
      const priorityValues = priorities[m.p.key];
      if (priorityValues && priorityValues.includes(m.p.value)) {
        m.score += 1000; // Boost score
      }
    });
  }

  private async _fetchCountsForSuggestions() {
    // Rely on worker-provided counts
  }

  private async _handleMultiSelectOpen(_key: string) {
    this._isOpen = false;
    // Rely on worker-provided counts
  }

  private _isPillHighlighted(idx: number): boolean {
    return this._selectedPills.has(idx);
  }

  private _handlePillClick(e: MouseEvent, idx: number) {
    // Always stop propagation to prevent focusing the input and opening query bar suggestions
    e.stopPropagation();

    if (e.ctrlKey || e.metaKey || e.shiftKey) {
      e.preventDefault();

      if (e.shiftKey && this._selectionAnchor !== null) {
        // Shift+Click: select range from anchor to clicked idx
        const start = Math.min(this._selectionAnchor, idx);
        const end = Math.max(this._selectionAnchor, idx);
        const newSelection = new Set<number>();
        for (let i = start; i <= end; i++) {
          newSelection.add(i);
        }
        this._selectedPills = newSelection;
        this._selectionFocus = idx;
      } else {
        // Ctrl+Click or Cmd+Click: toggle individual pill
        const newSelection = new Set(this._selectedPills);
        if (newSelection.has(idx)) {
          newSelection.delete(idx);
        } else {
          newSelection.add(idx);
        }
        this._selectedPills = newSelection;
        this._selectionAnchor = idx;
        this._selectionFocus = idx;
      }
      this._inputElement?.focus();
    } else {
      // Normal click: clear selection, let the pill handle its own click (open dropdown)
      this._selectedPills = new Set();
      this._selectionAnchor = null;
      this._selectionFocus = null;
    }
  }

  private _handlePointerDownInput(e: PointerEvent) {
    this._isDragging = true;
    this._dragStartPos = { x: e.clientX, y: e.clientY };
    this._startedInInput = true;

    const target = e.currentTarget as HTMLElement;
    target.setPointerCapture(e.pointerId);

    const pills = this.renderRoot.querySelectorAll('explore-multi-v2-select-sk');
    const rects: PillRect[] = [];
    pills.forEach((pill) => {
      const index = parseInt(pill.getAttribute('data-index') || '-1');
      const rect = pill.getBoundingClientRect();
      if (index !== -1) {
        rects.push({
          index,
          left: rect.left,
          right: rect.right,
          top: rect.top,
          bottom: rect.bottom,
        });
      }
    });
    this._pillRects = rects;
  }

  private _handlePointerMoveInput(e: PointerEvent) {
    if (this._startedInInput && this._isDragging && this._dragStartPos) {
      this._currentMouseX = e.clientX;
      this._currentMouseY = e.clientY;

      const dx = e.clientX - this._dragStartPos.x;
      const distance = Math.abs(dx);

      if (distance > QueryBarSk.DRAG_THRESHOLD_PX) {
        this._hasDragged = true;
        const keys = this._sortKeys(Object.keys(this.query));

        const inputEl = e.currentTarget as HTMLElement;
        const rect = inputEl.getBoundingClientRect();
        const isHorizontallyOutsideLeft = e.clientX < rect.left;

        const wasToTheLeft = this._isToTheLeft;
        this._isToTheLeft = isHorizontallyOutsideLeft;

        const textField = inputEl as any;
        const selectionStart = textField.selectionStart ?? 0;
        const selectionEnd = textField.selectionEnd ?? 0;

        if (!wasToTheLeft && isHorizontallyOutsideLeft) {
          if (selectionStart === 0) {
            this._canSelectPills = true;
            this._savedEnd = selectionEnd;
          }
        }

        let focusIndex = -1;
        if (this._canSelectPills) {
          const anchorIndex = keys.length - 1;
          this._selectionAnchor = anchorIndex;

          const containerRect = this.renderRoot
            .querySelector('.query-bar-container')
            ?.getBoundingClientRect();
          focusIndex = calculatePillFocusIndex(
            e.clientX,
            e.clientY,
            containerRect,
            this._pillRects
          );

          if (focusIndex !== -1) {
            this._selectionFocus = focusIndex;
            const start = Math.min(this._selectionAnchor, focusIndex);
            const end = Math.max(this._selectionAnchor, focusIndex);
            const newSelection = new Set<number>();
            for (let i = start; i <= end; i++) {
              newSelection.add(i);
            }
            this._selectedPills = newSelection;
          } else {
            this._selectionAnchor = null;
            this._selectionFocus = null;
            this._selectedPills = new Set();
          }
        }
      }
    }
  }

  private _handlePointerUpInput(e: PointerEvent) {
    if (this._isDragging) {
      this._isDragging = false;
      this._dragStartPos = null;
      this._startedInInput = false;
      this._canSelectPills = false;

      const target = e.currentTarget as HTMLElement;
      target.releasePointerCapture(e.pointerId);
    }
  }

  private _handleSelectionChange = () => {
    const activeEl = this.shadowRoot?.activeElement;
    if (activeEl === this._inputElement) {
      const textField = this._inputElement as any;
      const start = textField.selectionStart || 0;
      const end = textField.selectionEnd || 0;

      if (start > 0 || (start === end && textField.value !== '')) {
        this._canSelectPills = false;
        this._selectionAnchor = null;
        this._selectionFocus = null;
        this._selectedPills = new Set();
      }
    }
  };

  private _handleFocusOutContainer(e: FocusEvent) {
    const currentTarget = e.currentTarget as HTMLElement;
    if (!currentTarget.contains(e.relatedTarget as Node)) {
      this._selectionAnchor = null;
      this._selectionFocus = null;
      this._canSelectPills = false;
      this._selectedPills = new Set();
    }
  }

  _focusInput() {
    this._inputElement?.focus();
  }

  private _handlePasteEvent(e: ClipboardEvent) {
    const text = e.clipboardData?.getData('text') || '';
    if (text.includes('=')) {
      if (
        this._inputValue === '' ||
        (this._selectionAnchor !== null && this._selectionFocus !== null)
      ) {
        e.preventDefault();
        this._handlePaste(text);
      }
    }
  }

  private _handlePaste(text: string) {
    const tokens = text.match(/(?:[^\s"]+|"[^"]*")+/g) || [];
    tokens.forEach((token) => {
      const eqIdx = token.indexOf('=');
      if (eqIdx !== -1) {
        const key = token.substring(0, eqIdx);
        const val = token.substring(eqIdx + 1);
        if (key && val) {
          const vals = val.match(/(?:[^,"]+|"[^"]*")+/g) || [];
          vals.forEach((v) => {
            const cleanedVal = v.replace(/^"|"$/g, '');
            this._dispatchEvent('add-query', { key, value: cleanedVal });
          });
        }
      }
    });
  }

  private _handleFocus() {
    this._isOpen = true;
    this._updateSuggestions();

    if (this.pendingPillSelection) {
      const keys = this._sortKeys(Object.keys(this.query));
      const idx = keys.indexOf(this.pendingPillSelection);
      if (idx !== -1) {
        this._selectionAnchor = idx;
        this._selectionFocus = idx;
        this._selectedPills = new Set([idx]);
        this._canSelectPills = true;
      }
      this.pendingPillSelection = null;
    }
  }

  private _handleInputChange(e: InputEvent) {
    const val = (e.target as HTMLInputElement).value;
    this._inputValue = val;
    this._focusedIndex = 0;

    const trimmed = val.trim();
    this._isOpen = true;
    if (!trimmed) {
      this._selectedCategory = null;
    }
    this._updateSuggestions();

    this.dispatchEvent(
      new CustomEvent('suggest', {
        detail: { query: val },
        bubbles: true,
        composed: true,
      })
    );
  }

  private _handleKeyDown(e: KeyboardEvent) {
    const keys = this._sortKeys(Object.keys(this.query));

    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      this._handleArrowUpDown(e, keys);
    } else if (e.key === 'Enter') {
      this._handleEnter(e);
    } else if (e.key === 'Escape') {
      this._handleEscape();
    } else if (e.key === 'ArrowLeft' || e.key === 'ArrowRight') {
      this._handleArrowLeftRight(e, keys);
    } else if (e.key === 'a' && (e.ctrlKey || e.metaKey)) {
      this._handleSelectAll(e, keys);
    } else if (e.key === 'c' && (e.ctrlKey || e.metaKey)) {
      this._handleCopy(e, keys);
    } else if (e.key === 'x' && (e.ctrlKey || e.metaKey)) {
      this._handleCut(e, keys);
    } else if (e.key === 'Backspace' || e.key === 'Delete') {
      this._handleBackspaceDelete(e, keys);
    } else {
      this._handleDefaultKey(e, keys);
    }
  }

  private _handleArrowUpDown(e: KeyboardEvent, keys: string[]) {
    e.preventDefault();
    const isDown = e.key === 'ArrowDown';
    if (this._isOpen && this._suggestions.length > 0) {
      const step = isDown ? 1 : -1;
      this._focusedIndex =
        (this._focusedIndex + step + this._suggestions.length) % this._suggestions.length;
      this._scrollFocusedIntoView();
    } else if (!this._isOpen) {
      const root = this.getRootNode() as ParentNode;
      const bars = Array.from(root.querySelectorAll('query-bar-sk'));
      const idx = bars.indexOf(this);
      if (idx !== -1) {
        const step = isDown ? 1 : -1;
        const nextIdx = (idx + step + bars.length) % bars.length;
        const nextBar = bars[nextIdx] as QueryBarSk;

        if (this._selectionAnchor !== null && this._selectionAnchor === this._selectionFocus) {
          nextBar.pendingPillSelection = keys[this._selectionAnchor];
        }

        nextBar._focusInput();
      }
    }
  }

  private _handleEnter(e: KeyboardEvent) {
    e.preventDefault();
    if (this._isOpen && this._suggestions.length > 0) {
      const item = this._suggestions[this._focusedIndex];
      if (item) {
        this._handleSelect(item);
      }
    } else if (
      this._selectionAnchor !== null &&
      this._selectionAnchor === this._selectionFocus &&
      (this._inputValue === '' ||
        this._inputElement?.selectionStart === this._inputElement?.selectionEnd)
    ) {
      this._openPillIndex = this._selectionFocus;
    }
  }

  private _handleEscape() {
    this._isOpen = false;
    this._selectionAnchor = null;
    this._selectionFocus = null;
    this._selectedPills = new Set();
  }

  private _handleArrowLeftRight(e: KeyboardEvent, keys: string[]) {
    const isRight = e.key === 'ArrowRight';
    const isLeft = e.key === 'ArrowLeft';
    const selectionStart = this._inputElement?.selectionStart ?? 0;

    const isPillFocused = this._selectionAnchor !== null && this._selectionFocus !== null;
    const canNavigate =
      this._inputValue === '' || (isLeft && selectionStart === 0) || (isRight && isPillFocused);

    if (!canNavigate) return;

    e.preventDefault();
    if (keys.length === 0) return;

    let nextFocus = this._selectionFocus;
    if (nextFocus === null) {
      if (isRight) return;
      nextFocus = keys.length - 1;
    } else {
      const step = isRight ? 1 : -1;
      nextFocus = Math.max(0, Math.min(keys.length, nextFocus + step));
    }

    if (nextFocus === keys.length) {
      this._selectionAnchor = null;
      this._selectionFocus = null;
      this._selectedPills = new Set();
      this._inputElement?.focus();
    } else {
      this._selectionFocus = nextFocus;
      if (!e.shiftKey) {
        this._selectionAnchor = nextFocus;
        this._selectedPills = new Set([nextFocus]);
      } else {
        if (this._selectionAnchor === null) {
          this._selectionAnchor = keys.length - 1;
        }
        const start = Math.min(this._selectionAnchor, nextFocus);
        const end = Math.max(this._selectionAnchor, nextFocus);
        const newSelection = new Set<number>();
        for (let i = start; i <= end; i++) {
          newSelection.add(i);
        }
        this._selectedPills = newSelection;
      }
    }
  }

  private _handleSelectAll(e: KeyboardEvent, keys: string[]) {
    e.preventDefault();
    if (keys.length > 0) {
      this._selectionAnchor = keys.length - 1;
      this._selectionFocus = 0;
      const newSelection = new Set<number>();
      for (let i = 0; i < keys.length; i++) {
        newSelection.add(i);
      }
      this._selectedPills = newSelection;
    }
    this._inputElement?.select();
  }

  private _handleCopy(e: KeyboardEvent, keys: string[]) {
    if (this._selectedPills.size > 0) {
      e.preventDefault();
      const selectedKeys = keys.filter((_, idx) => this._selectedPills.has(idx));
      let text = selectedKeys
        .map((k) => {
          const values = (this.query[k] || []).map((v) => (v.includes(' ') ? `"${v}"` : v));
          return `${k}=${values.join(',')}`;
        })
        .join(' ');

      const textSelectionStart = this._inputElement?.selectionStart ?? 0;
      const textSelectionEnd = this._inputElement?.selectionEnd ?? 0;
      if (textSelectionStart !== textSelectionEnd && this._inputValue !== '') {
        const selectedText = this._inputValue.substring(textSelectionStart, textSelectionEnd);
        if (text && selectedText) {
          text += ' ' + selectedText;
        } else if (selectedText) {
          text = selectedText;
        }
      }

      navigator.clipboard.writeText(text);
    }
  }

  private _handleCut(e: KeyboardEvent, keys: string[]) {
    if (this._selectedPills.size > 0) {
      this._handleCopy(e, keys);

      const textSelectionStart = this._inputElement?.selectionStart ?? 0;
      const textSelectionEnd = this._inputElement?.selectionEnd ?? 0;
      if (textSelectionStart !== textSelectionEnd && this._inputValue !== '') {
        this._inputValue =
          this._inputValue.substring(0, textSelectionStart) +
          this._inputValue.substring(textSelectionEnd);
      }

      const selectedKeys = keys.filter((_, idx) => this._selectedPills.has(idx));
      selectedKeys.forEach((k) => this._dispatchEvent('remove-key', { key: k }));
      this._selectionAnchor = null;
      this._selectionFocus = null;
      this._selectedPills = new Set();
    }
  }

  private _handleBackspaceDelete(e: KeyboardEvent, keys: string[]) {
    const hasSelection = this._selectedPills.size > 0;

    if (hasSelection) {
      e.preventDefault();
      const selectedKeys = keys.filter((_, idx) => this._selectedPills.has(idx));
      selectedKeys.forEach((k) => this._dispatchEvent('remove-key', { key: k }));
      this._selectionAnchor = null;
      this._selectionFocus = null;
      this._selectedPills = new Set();
    } else if (e.key === 'Backspace' && this._inputValue === '' && keys.length > 0) {
      e.preventDefault();
      const lastKey = keys[keys.length - 1];
      this._dispatchEvent('remove-key', { key: lastKey });
    }
  }

  private _handleDefaultKey(e: KeyboardEvent, keys: string[]) {
    if (this._selectionAnchor !== null && this._selectionFocus !== null) {
      const isModifier = e.ctrlKey || e.metaKey || e.altKey;
      const isNavigation = [
        'ArrowLeft',
        'ArrowRight',
        'ArrowUp',
        'ArrowDown',
        'Home',
        'End',
      ].includes(e.key);
      const isSelection = ['Shift', 'Control', 'Alt', 'Meta'].includes(e.key);

      if (!isModifier && !isNavigation && !isSelection && e.key.length === 1) {
        e.preventDefault();
        const selectedKeys = keys.filter((_, idx) => this._selectedPills.has(idx));
        selectedKeys.forEach((k) => this._dispatchEvent('remove-key', { key: k }));
        this._selectionAnchor = null;
        this._selectionFocus = null;
        this._selectedPills = new Set();

        this._inputValue = this._inputValue + e.key;
        this._isOpen = true;
        this._updateSuggestions();
      }
    }
  }

  private _handleSelect(suggestion: Suggestion) {
    const p = suggestion.params[0];
    if (p.value === '') {
      this._selectedCategory = p.key;
      this._inputValue = `${p.key}=`;
      this.externalSuggestions = null; // Clear stale suggestions
      this._updateSuggestions();
      return;
    }

    suggestion.params.forEach((param) => {
      this._dispatchEvent('add-query', { key: param.key, value: param.value });
    });
    this._inputValue = '';
    this._selectedCategory = null;
    this._isOpen = true;
    this.externalSuggestions = null; // Clear stale suggestions
    this._updateSuggestions();
    setTimeout(() => this._inputElement?.focus(), 0);
  }

  private _handlePillChange(key: string, value: string) {
    const currentValues = this.query[key] || [];
    if (currentValues.includes(value)) {
      this._dispatchEvent('remove-query', { key, value });
    } else {
      this._dispatchEvent('add-query', { key, value });
    }
  }

  private _handleReplace(key: string, value: string) {
    this._dispatchEvent('set-selected', { key, values: [value] });
  }

  private _scrollFocusedIntoView() {
    setTimeout(() => {
      if (this._listElement && this._focusedIndex >= 0) {
        const item = this._listElement.children[this._focusedIndex] as HTMLElement;
        if (item) {
          item.scrollIntoView({ block: 'nearest' });
        }
      }
    }, 0);
  }

  private _dispatchEvent(name: string, detail: any) {
    this.dispatchEvent(
      new CustomEvent(name, {
        detail,
        bubbles: true,
        composed: true,
      })
    );
  }

  private _sortOptions(options: { value: string; count: number }[], selectedValues: string[]) {
    return [...options].sort((a, b) => {
      const aSelected = selectedValues.includes(a.value);
      const bSelected = selectedValues.includes(b.value);

      if (aSelected && !bSelected) return -1;
      if (!aSelected && bSelected) return 1;

      return b.count - a.count;
    });
  }

  private _addPipelineStep(funcId: string) {
    const newPipeline = [...this.pipeline, funcId];
    this.pipeline = newPipeline;
    this._showFormulaPopover = false;
    this._dispatchEvent('pipeline-change', { pipeline: newPipeline });
  }

  private _removePipelineStep(idx: number) {
    const newPipeline = this.pipeline.filter((_, i) => i !== idx);
    this.pipeline = newPipeline;
    this._dispatchEvent('pipeline-change', { pipeline: newPipeline });
  }

  render() {
    const activePipeline =
      this.pipeline.length > 0
        ? this.pipeline
        : this.formula && this.formula !== 'none'
          ? [this.formula]
          : [];

    return html`
      <div
        class="query-bar-container"
        @click=${() => this._inputElement?.focus()}
        @focusout=${this._handleFocusOutContainer}>
        <div class="query-pills">
          ${repeat(
            this._sortKeys(Object.keys(this.query)).map(
              (key, idx) => [key, this.query[key], idx] as [string, string[], number]
            ),
            ([key]) => key,
            ([key, values, idx]) => {
              const options = this.optionsByKey[key] || [];
              const sortedOptions = this._sortOptions(options, values);
              return html`
                <explore-multi-v2-select-sk
                  @click=${(e: MouseEvent) => this._handlePillClick(e, idx)}
                  data-index=${idx}
                  .label=${key}
                  .variant=${'pill'}
                  .options=${sortedOptions}
                  .selected=${values}
                  .isSplit=${this.splitKeys.has(key)}
                  .isHighlighted=${this._isPillHighlighted(idx)}
                  .isOpen=${idx === this._openPillIndex}
                  .showSplitButton=${true}
                  .showDiffButton=${true}
                  .loading=${this.loading}
                  @open=${() => {
                    this._openPillIndex = idx;
                    void this._handleMultiSelectOpen(key);
                  }}
                  @close=${() => {
                    this._openPillIndex = null;
                  }}
                  @close-with-esc=${() => {
                    this._selectionAnchor = idx;
                    this._selectionFocus = idx;
                    this._openPillIndex = null;
                    this._inputElement?.focus();
                  }}
                  @selection-change=${(e: CustomEvent<MultiSelectSelectionEventDetail>) =>
                    this._handlePillChange(key, e.detail.value)}
                  @set-selection=${(e: CustomEvent<MultiSelectSetSelectionEventDetail>) =>
                    this._dispatchEvent('set-selected', { key, values: e.detail.values })}
                  @replace-selection=${(e: CustomEvent<MultiSelectReplaceEventDetail>) =>
                    this._handleReplace(key, e.detail.value)}
                  @remove-key=${(e: Event) => {
                    e.stopPropagation();
                    this._dispatchEvent('remove-key', { key });
                  }}
                  @split=${() => this._dispatchEvent('split', { key })}
                  @diff-base=${(e: CustomEvent) => {
                    e.stopPropagation();
                    this._dispatchEvent('diff-base', e.detail);
                  }}></explore-multi-v2-select-sk>
              `;
            }
          )}

          <div class="query-input-wrapper">
            <md-outlined-text-field
              type="text"
              class="query-input"
              .value=${this._inputValue}
              @input=${this._handleInputChange}
              @keydown=${this._handleKeyDown}
              @focus=${this._handleFocus}
              @pointerdown=${this._handlePointerDownInput}
              @pointermove=${this._handlePointerMoveInput}
              @pointerup=${this._handlePointerUpInput}
              @paste=${this._handlePasteEvent}
              placeholder=${this._getPlaceholderTip()}
              @click=${(e: Event) => e.stopPropagation()}></md-outlined-text-field>
            ${this.loading ? html`<div class="input-spinner"></div>` : ''}
            ${this._isOpen && this._suggestions.length > 0
              ? html`
                  <div class="suggestions-dropdown" @click=${(e: Event) => e.stopPropagation()}>
                    ${this._suggestions.map(
                      (s, idx) => html`
                        <div
                          class="suggestion-item ${idx === this._focusedIndex ? 'focused' : ''}"
                          @click=${() => this._handleSelect(s)}
                          @mouseenter=${() => (this._focusedIndex = idx)}>
                          ${s.params.map(
                            (p) => html`
                              <span class="suggestion-pill">
                                <span class="s-key">${p.key}</span>
                                <span class="s-sep">=</span>
                                <span class="s-val">${p.value}</span>
                              </span>
                            `
                          )}
                          ${s.count !== undefined
                            ? html`
                                <span class="s-count right ${this.loading ? 'stale' : ''}"
                                  >(${s.count.toLocaleString()}${s.countIsLowerBound
                                    ? '+'
                                    : ''})</span
                                >
                              `
                            : ''}
                        </div>
                      `
                    )}
                  </div>
                `
              : ''}
          </div>
        </div>

        <div class="query-actions" @click=${(e: Event) => e.stopPropagation()}>
          <md-icon-button
            class="qb-clone-query-btn"
            @click=${(e: Event) => {
              e.stopPropagation();
              this._dispatchEvent('clone-query', {});
            }}
            title="Duplicate this query"
            aria-label="Duplicate this query">
            <md-icon>content_copy</md-icon>
          </md-icon-button>

          ${this.showRemoveQueryButton
            ? html`
                <md-icon-button
                  class="qb-remove-query-btn"
                  @click=${(e: Event) => {
                    e.stopPropagation();
                    this._dispatchEvent('clear-query', {});
                  }}
                  title="Remove this query"
                  aria-label="Remove this query">
                  <md-icon>close</md-icon>
                </md-icon-button>
              `
            : ''}

          <div class="qb-pipeline-container">
            ${activePipeline.map(
              (step, idx) => html`
                <span class="qb-pipeline-step">
                  ${step}()
                  <span
                    class="qb-pipeline-remove"
                    title="Remove this step"
                    @click=${(e: Event) => {
                      e.stopPropagation();
                      this._removePipelineStep(idx);
                    }}
                    >&times;</span
                  >
                </span>
                ${idx < activePipeline.length - 1
                  ? html`<span class="qb-pipeline-arrow">&rarr;</span>`
                  : ''}
              `
            )}

            <button
              class="qb-add-formula-btn"
              @click=${(e: Event) => {
                e.stopPropagation();
                this._showFormulaPopover = !this._showFormulaPopover;
              }}
              title="Add formula transformation step">
              ${activePipeline.length > 0 ? 'Add Step' : 'Formula'}
            </button>

            ${this._showFormulaPopover
              ? html`
                  <div class="qb-formula-popover" @click=${(e: Event) => e.stopPropagation()}>
                    ${[
                      { id: 'prep', title: '1. Preprocessing' },
                      { id: 'agg', title: '2. Aggregation' },
                      { id: 'trans', title: '3. Transformation' },
                    ].map(
                      (cat) => html`
                        <div class="qb-formula-popover-header">${cat.title}</div>
                        <div class="qb-formula-grid">
                          ${FORMULA_OPTIONS.filter((f) => f.category === cat.id).map(
                            (f) => html`
                              <button
                                class="qb-formula-item"
                                @click=${(e: Event) => {
                                  e.stopPropagation();
                                  this._addPipelineStep(f.id);
                                }}>
                                <span class="qb-formula-item-name">${f.name}</span>
                                <span class="qb-formula-item-desc">${f.desc}</span>
                              </button>
                            `
                          )}
                        </div>
                      `
                    )}
                  </div>
                `
              : ''}
          </div>
        </div>
      </div>
    `;
  }
}
