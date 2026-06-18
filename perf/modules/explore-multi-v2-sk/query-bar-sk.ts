import { LitElement, html, css, PropertyValues } from 'lit';
import { customElement, property, state, query } from 'lit/decorators.js';
import { repeat } from 'lit/directives/repeat.js';
import { scoreParamAny, fuzzyScore } from './fuzzy';
import './multi-select-sk';

import '@material/web/textfield/outlined-text-field';
import '@material/web/iconbutton/icon-button';
import '@material/web/icon/icon';
import {
  MultiSelectSelectionEventDetail,
  MultiSelectSetSelectionEventDetail,
  MultiSelectReplaceEventDetail,
} from './multi-select-sk';

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

@customElement('query-bar-sk')
export class QueryBarSk extends LitElement {
  @property({ type: Object }) query: Record<string, string[]> = {};

  @property({ type: Array }) availableParams: { key: string; value: string; count?: number }[] = [];

  @property({ type: Object }) optionsByKey: Record<string, { value: string; count: number }[]> = {};

  @property({ type: Array }) includeParams: string[] = [];

  @property({ type: Object }) defaults: any = null;

  @property({ type: Object }) splitKeys: Set<string> = new Set();

  @property({ type: Boolean }) showRemoveQueryButton = false;

  @property({ type: Array }) externalSuggestions: Suggestion[] | null = null;

  @state() private _inputValue = '';

  @state() private _isOpen = false;

  @state() private _focusedIndex = 0;

  @state() private _suggestions: Suggestion[] = [];

  @state() private _isLoadingSuggestions = false;

  @state() private _selectedCategory: string | null = null;

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
      box-shadow: 0 1px 2px color-mix(in srgb, var(--transparent-overlay) 10%, transparent);
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
      max-height: 300px;
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
  `;

  connectedCallback() {
    super.connectedCallback();
    document.addEventListener('mousedown', this._handleClickOutside);
  }

  disconnectedCallback() {
    document.removeEventListener('mousedown', this._handleClickOutside);
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

  private _updateSuggestions() {
    const trimmed = this._inputValue.trim();
    if (!trimmed) {
      this.externalSuggestions = null;
      if (this._isOpen) {
        if (!this._selectedCategory) {
          const keys = Object.keys(this.optionsByKey).filter((k) => !this.query[k]);
          this._suggestions = keys.map((k) => {
            const options = this.optionsByKey[k] || [];
            const totalCount = options.reduce((sum, o) => sum + o.count, 0);
            return {
              params: [{ key: k, value: '' }],
              score: 100,
              count: totalCount,
              countIsLowerBound: false,
            };
          });
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
          const options = this.optionsByKey[this._selectedCategory] || [];
          this._suggestions = options.map((o) => ({
            params: [{ key: this._selectedCategory!, value: o.value }],
            score: 100,
            count: o.count,
            countIsLowerBound: false,
          }));
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

      const scored = this.availableParams
        .filter((p) => !this.query[p.key])
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

      this._suggestions = matches.slice(0, 50).map((s) => ({
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

          for (const opt of this.optionsByKey[key]) {
            const matches = regexes.some((r) => r.test(opt.value));
            if (matches) {
              matchesAnything = true;
              totalCount += opt.count;
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
        this._suggestions = globSuggestions.slice(0, 50);
        return;
      } catch (_e) {
        this._suggestions = [];
        return;
      }
    }

    const scored = this.availableParams
      .filter((p) => !this.query[p.key])
      .map((p) => {
        return { p, score: scoreParamAny(p, trimmed) };
      });

    const matches = scored.filter((s) => s.score > -Infinity);
    console.log('[_updateSuggestions] matches count:', matches.length);

    this._boostPriorityScores(matches);
    matches.sort((a, b) => b.score - a.score);

    this._suggestions = matches.slice(0, 50).map((s) => ({
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

  private _handleFocus() {
    this._isOpen = true;
    this._updateSuggestions();
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
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      this._focusedIndex = (this._focusedIndex + 1) % (this._suggestions.length || 1);
      this._scrollFocusedIntoView();
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      this._focusedIndex =
        (this._focusedIndex - 1 + (this._suggestions.length || 1)) %
        (this._suggestions.length || 1);
      this._scrollFocusedIntoView();
    } else if (e.key === 'Enter') {
      e.preventDefault();
      if (this._isOpen && this._suggestions.length > 0) {
        const item = this._suggestions[this._focusedIndex];
        if (item) {
          this._handleSelect(item);
        }
      }
    } else if (e.key === 'Backspace' && this._inputValue === '') {
      const keys = Object.keys(this.query);
      if (keys.length > 0) {
        const lastKey = keys[keys.length - 1];
        this._dispatchEvent('remove-key', { key: lastKey });
      }
    } else if (e.key === 'Escape') {
      this._isOpen = false;
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

  render() {
    return html`
      <div class="query-bar-container" @click=${() => this._inputElement?.focus()}>
        <div class="query-pills">
          ${repeat(
            this._sortKeys(Object.keys(this.query)).map(
              (key) => [key, this.query[key]] as [string, string[]]
            ),
            ([key]) => key,
            ([key, values]) => {
              const options = this.optionsByKey[key] || [];
              const sortedOptions = this._sortOptions(options, values);
              return html`
                <explore-multi-v2-select-sk
                  @click=${(e: Event) => e.stopPropagation()}
                  .label=${key}
                  .variant=${'pill'}
                  .options=${sortedOptions}
                  .selected=${values}
                  .isSplit=${this.splitKeys.has(key)}
                  .showSplitButton=${true}
                  .showDiffButton=${true}
                  @open=${() => {
                    void this._handleMultiSelectOpen(key);
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
              placeholder=${this._getPlaceholderTip()}
              @click=${(e: Event) => e.stopPropagation()}></md-outlined-text-field>
            ${this._isLoadingSuggestions ? html`<div class="input-spinner"></div>` : ''}
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
                                <span class="s-count right"
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

        <div class="query-actions">
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
        </div>
      </div>
    `;
  }
}
