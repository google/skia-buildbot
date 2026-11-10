import { Locator, Page } from 'playwright-core';

/**
 * Page object for /e2 (explore-multi-v2-sk) covering faceted search bars,
 * pill dropdowns, Split By, Diff Base, formula pipelines, toolbar toggles,
 * and the interactive tour.
 */
export class ExploreV2Page {
  constructor(
    public readonly page: Page,
    public readonly baseUrl: string
  ) {}

  public async goto(queryAndParams: string = ''): Promise<void> {
    const suffix = queryAndParams
      ? queryAndParams.startsWith('?')
        ? queryAndParams
        : `?${queryAndParams}`
      : '';
    await this.page.goto(`${this.baseUrl}/e2/${suffix}`, { waitUntil: 'networkidle' });
    await this.root.waitFor({ state: 'visible', timeout: 10000 });
    await this.waitForWorkerReady();
  }

  public get root(): Locator {
    return this.page.locator('explore-multi-v2-sk');
  }

  public get queryBars(): Locator {
    return this.root.locator('query-bar-sk');
  }

  public get toolbar(): Locator {
    return this.root.locator('explore-toolbar-sk');
  }

  public get charts(): Locator {
    return this.root.locator('trace-chart-sk');
  }

  public async waitForWorkerReady(): Promise<void> {
    await this.page.waitForFunction(
      () => {
        const el = document.querySelector('explore-multi-v2-sk') as any;
        return !!el && !el._workerInitializing && el._defaults !== null;
      },
      undefined,
      { timeout: 10000 }
    );
    await this.waitForIdle();
  }

  public async waitForIdle(): Promise<void> {
    await this.page.waitForTimeout(150);
    await this.page.waitForFunction(
      () => {
        const el = document.querySelector('explore-multi-v2-sk') as any;
        return (
          !!el &&
          !el._workerInitializing &&
          !el._isCalculatingCounts &&
          !el._loading &&
          el._workerFilterTimer === null
        );
      },
      undefined,
      { timeout: 10000 }
    );
  }

  public async waitForChartCount(count: number): Promise<void> {
    await this.page.waitForFunction(
      (expected: number) => {
        const el = document.querySelector('explore-multi-v2-sk');
        const sr = el?.shadowRoot || el;
        return (sr?.querySelectorAll('trace-chart-sk').length || 0) === expected;
      },
      count,
      { timeout: 10000 }
    );
  }

  /**
   * Adds a key=value filter to the nth query-bar-sk.
   */
  public async addQueryFilter(barIndex: number, key: string, value: string): Promise<void> {
    const bar = this.queryBars.nth(barIndex);
    await bar.evaluate(
      (el: any, detail: { key: string; value: string }) => {
        el.dispatchEvent(
          new CustomEvent('add-query', {
            detail,
            bubbles: true,
            composed: true,
          })
        );
      },
      { key, value }
    );
    await this.waitForIdle();
  }

  /**
   * Adds multiple key=value filters sequentially to the nth query-bar-sk.
   */
  public async addQueryFilters(barIndex: number, filters: Record<string, string>): Promise<void> {
    for (const [key, value] of Object.entries(filters)) {
      await this.addQueryFilter(barIndex, key, value);
    }
  }

  /**
   * Sets multiple values on a key in the nth query-bar-sk.
   */
  public async setQueryValues(barIndex: number, key: string, values: string[]): Promise<void> {
    const bar = this.queryBars.nth(barIndex);
    await bar.evaluate(
      (el: any, detail: { key: string; values: string[] }) => {
        el.dispatchEvent(
          new CustomEvent('set-selected', {
            detail,
            bubbles: true,
            composed: true,
          })
        );
      },
      { key, values }
    );
    await this.waitForIdle();
  }

  /**
   * Types into the nth query-bar-sk input to trigger autocomplete suggestions.
   */
  public async typeInSearchInput(barIndex: number, text: string): Promise<void> {
    const input = this.queryBars.nth(barIndex).locator('.query-input input');
    await input.click();
    await input.fill(text);
    await this.page.waitForTimeout(150);
  }

  /**
   * Toggles splitting by the given parameter key on /e2 via the pill dropdown's
   * .ms-split-btn and verifies the toolbar's #yaxis-splitter control.
   */
  public async toggleSplitBy(key: string): Promise<void> {
    await this.toolbar.evaluate((el: any) => {
      const sr = el.shadowRoot || el;
      if (!sr.querySelector('details#yaxis-splitter')) {
        throw new Error('details#yaxis-splitter not found in explore-toolbar-sk');
      }
    });
    const bar = this.queryBars.first();
    await bar.evaluate(async (el: any, splitKey: string) => {
      const sr = el.shadowRoot || el;
      const selects = Array.from(sr.querySelectorAll('explore-multi-v2-select-sk')) as any[];
      const pill = selects.find((s) => s.label === splitKey);
      if (!pill) {
        throw new Error(`Pill "${splitKey}" not found`);
      }
      const pillSr = pill.shadowRoot || pill;
      const trigger = pillSr.querySelector('.multiselect-trigger') as HTMLElement | null;
      if (trigger && !pill.isOpen) {
        trigger.click();
        await el.updateComplete;
        await pill.updateComplete;
      }
      const splitBtn = pillSr.querySelector('button.ms-split-btn') as HTMLButtonElement | null;
      if (!splitBtn) {
        throw new Error(`button.ms-split-btn not found in pill "${splitKey}"`);
      }
      const stopNullDetailSplit = (e: Event) => {
        if (!(e as CustomEvent).detail) {
          e.stopImmediatePropagation();
          e.stopPropagation();
        }
      };
      sr.addEventListener('split', stopNullDetailSplit);
      try {
        splitBtn.click();
      } finally {
        sr.removeEventListener('split', stopNullDetailSplit);
      }
      await el.updateComplete;
      await pill.updateComplete;
    }, key);
    await this.waitForIdle();
  }

  /**
   * Sets or toggles the Diff Base (key, value) on /e2 via the pill dropdown's .ms-diff-btn.
   */
  public async toggleDiffBase(key: string, value: string): Promise<void> {
    const bar = this.queryBars.first();
    await bar.evaluate(
      async (el: any, detail: { key: string; value: string }) => {
        const sr = el.shadowRoot || el;
        const selects = Array.from(sr.querySelectorAll('explore-multi-v2-select-sk')) as any[];
        const pill = selects.find((s) => s.label === detail.key);
        if (!pill) {
          throw new Error(`Pill "${detail.key}" not found`);
        }
        const pillSr = pill.shadowRoot || pill;
        const trigger = pillSr.querySelector('.multiselect-trigger') as HTMLElement | null;
        if (trigger && !pill.isOpen) {
          trigger.click();
          await el.updateComplete;
          await pill.updateComplete;
        }
        const options = Array.from(pillSr.querySelectorAll('.multiselect-option')) as HTMLElement[];
        const target = options.find(
          (o) => (o.querySelector('.ms-opt-value')?.textContent || '').trim() === detail.value
        );
        if (!target) {
          throw new Error(`Option "${detail.value}" not found in pill "${detail.key}"`);
        }
        const diffBtn = target.querySelector('button.ms-diff-btn') as HTMLButtonElement | null;
        if (!diffBtn) {
          throw new Error(
            `button.ms-diff-btn not found for "${detail.value}" in pill "${detail.key}"`
          );
        }
        diffBtn.click();
        await el.updateComplete;
        await pill.updateComplete;
      },
      { key, value }
    );
    await this.waitForIdle();
  }

  /**
   * Adds a new query bar row using the "+" circle button.
   */
  public async addQueryRow(): Promise<void> {
    await this.root.locator('.add-query-circle-btn').click();
    await this.waitForIdle();
  }

  /**
   * Removes the nth query bar row and waits for the worker to settle.
   */
  public async removeQueryRow(barIndex: number): Promise<void> {
    await this.queryBars.nth(barIndex).locator('.qb-remove-query-btn').click();
    await this.waitForIdle();
  }

  /**
   * Adds a formula pipeline step (e.g. "ave", "norm", "sum") on the nth query bar.
   */
  public async addFormulaStep(barIndex: number, funcId: string): Promise<void> {
    const bar = this.queryBars.nth(barIndex);
    await bar.locator('.qb-add-formula-btn').click();
    const item = bar
      .locator('.qb-formula-item-name')
      .filter({ hasText: new RegExp(`^${funcId}\\(\\)$`) });
    await item.click();
    await this.waitForIdle();
  }

  /**
   * Adds multiple formula pipeline steps sequentially on the nth query bar.
   */
  public async addFormulaSteps(barIndex: number, funcIds: string[]): Promise<void> {
    for (const funcId of funcIds) {
      await this.addFormulaStep(barIndex, funcId);
    }
  }

  /**
   * Returns the series IDs rendered on the chartIndex-th trace-chart-sk.
   */
  public async getSeriesIds(chartIndex: number = 0): Promise<string[]> {
    return this.charts
      .nth(chartIndex)
      .evaluate((chart: any) => (chart?.series || []).map((s: any) => String(s.id)));
  }

  /**
   * Sets a toolbar control value on explore-toolbar-sk via its rendered DOM inputs.
   */
  public async setToolbarControl(name: string, value: any): Promise<void> {
    await this.toolbar.evaluate(
      async (el: any, detail: { name: string; value: any }) => {
        const sr = el.shadowRoot || el;
        const details = sr.querySelector(
          'details.advanced-options-details'
        ) as HTMLDetailsElement | null;
        if (!details || !details.open) {
          throw new Error(
            'explore-toolbar-sk details.advanced-options-details is missing or closed'
          );
        }

        const findCheckboxBySelector = (selector: string): HTMLInputElement => {
          const input = sr.querySelector(
            `${selector} input[type="checkbox"]`
          ) as HTMLInputElement | null;
          if (!input) {
            throw new Error(`Checkbox "${selector}" not found in explore-toolbar-sk`);
          }
          return input;
        };

        const findCheckboxByText = (labelText: string): HTMLInputElement => {
          const labels = Array.from(sr.querySelectorAll('label.custom-checkbox')) as HTMLElement[];
          const match = labels.find((l) => (l.textContent || '').trim() === labelText);
          const input = match?.querySelector('input[type="checkbox"]') as HTMLInputElement | null;
          if (!input) {
            throw new Error(`Checkbox with label "${labelText}" not found in explore-toolbar-sk`);
          }
          return input;
        };

        const findSelectByGroupLabel = (groupLabel: string): HTMLSelectElement => {
          const groups = Array.from(sr.querySelectorAll('.toolbar-group')) as HTMLElement[];
          const group = groups.find(
            (g) => (g.querySelector('.label')?.textContent || '').trim() === groupLabel
          );
          const select = group?.querySelector('select.custom-select') as HTMLSelectElement | null;
          if (!select) {
            throw new Error(`Select for "${groupLabel}" not found in explore-toolbar-sk`);
          }
          return select;
        };

        const setCheckbox = (input: HTMLInputElement, want: boolean) => {
          if (input.checked !== want) {
            input.click();
          }
        };

        const setSelect = (select: HTMLSelectElement, want: string) => {
          if (select.value !== want) {
            select.value = want;
            select.dispatchEvent(new Event('change', { bubbles: true }));
          }
        };

        switch (detail.name) {
          case 'dateMode':
            setCheckbox(findCheckboxBySelector('label.date-mode-checkbox'), Boolean(detail.value));
            break;
          case 'smooth':
            setCheckbox(findCheckboxBySelector('label.smooth-checkbox'), Boolean(detail.value));
            break;
          case 'splitAll': {
            if (!(window as any).perf?.enable_split_all_option) {
              (window as any).perf = (window as any).perf || {};
              (window as any).perf.enable_split_all_option = true;
              el.requestUpdate();
              await el.updateComplete;
            }
            setCheckbox(findCheckboxBySelector('label.split-all-checkbox'), Boolean(detail.value));
            break;
          }
          case 'evenXAxisSpacing':
            setCheckbox(findCheckboxByText('Even X-Axis Spacing'), Boolean(detail.value));
            break;
          case 'showZero':
            setCheckbox(findCheckboxByText('Show Zero'), Boolean(detail.value));
            break;
          case 'showDots':
            setCheckbox(findCheckboxByText('Dots'), Boolean(detail.value));
            break;
          case 'showRegressions':
            setCheckbox(findCheckboxByText('Show Regressions'), Boolean(detail.value));
            break;
          case 'showSparklines':
            setCheckbox(findCheckboxByText('Sparklines'), Boolean(detail.value));
            break;
          case 'tooltipDiffs':
            setCheckbox(findCheckboxByText('Tooltip Diffs'), Boolean(detail.value));
            break;
          case 'normalizeCentre':
            setSelect(findSelectByGroupLabel('Center:'), String(detail.value));
            break;
          case 'hoverMode':
            setSelect(findSelectByGroupLabel('Hover:'), String(detail.value));
            break;
          default:
            el.dispatchEvent(
              new CustomEvent('control-change', {
                detail,
                bubbles: true,
                composed: true,
              })
            );
            break;
        }
        await el.updateComplete;
      },
      { name, value }
    );
    await this.page.waitForTimeout(100);
  }

  /**
   * Batch-updates multiple toolbar controls on explore-toolbar-sk.
   */
  public async setToolbarControls(controls: Record<string, any>): Promise<void> {
    for (const [name, value] of Object.entries(controls)) {
      if (value !== undefined) {
        await this.setToolbarControl(name, value);
      }
    }
    await this.page.waitForFunction(() => window.location.search.includes('shortcut='));
  }

  /**
   * Reads the current toolbar toggle state from the rendered explore-toolbar-sk DOM.
   */
  public async getToolbarState(): Promise<{
    dateMode: boolean;
    evenXAxisSpacing: boolean;
    showZero: boolean;
    hoverMode: string;
    showDots: boolean;
    showRegressions: boolean;
    normalizeCentre: string;
  }> {
    return this.toolbar.evaluate((el: any) => {
      const sr = el.shadowRoot || el;
      const details = sr.querySelector(
        'details.advanced-options-details'
      ) as HTMLDetailsElement | null;
      if (!details || !details.open) {
        throw new Error('explore-toolbar-sk details.advanced-options-details is missing or closed');
      }
      if (!sr.querySelector('details#yaxis-splitter')) {
        throw new Error('details#yaxis-splitter not found in explore-toolbar-sk');
      }
      const dateInput = sr.querySelector(
        'label.date-mode-checkbox input[type="checkbox"]'
      ) as HTMLInputElement | null;
      if (!dateInput) {
        throw new Error('label.date-mode-checkbox not found in explore-toolbar-sk');
      }
      if (!sr.querySelector('label.smooth-checkbox input[type="checkbox"]')) {
        throw new Error('label.smooth-checkbox not found in explore-toolbar-sk');
      }
      const findCheckboxByText = (labelText: string): HTMLInputElement => {
        const labels = Array.from(sr.querySelectorAll('label.custom-checkbox')) as HTMLElement[];
        const match = labels.find((l) => (l.textContent || '').trim() === labelText);
        const input = match?.querySelector('input[type="checkbox"]') as HTMLInputElement | null;
        if (!input) {
          throw new Error(`Checkbox "${labelText}" not found in explore-toolbar-sk`);
        }
        return input;
      };
      const findSelectByGroupLabel = (groupLabel: string): HTMLSelectElement => {
        const groups = Array.from(sr.querySelectorAll('.toolbar-group')) as HTMLElement[];
        const group = groups.find(
          (g) => (g.querySelector('.label')?.textContent || '').trim() === groupLabel
        );
        const select = group?.querySelector('select.custom-select') as HTMLSelectElement | null;
        if (!select) {
          throw new Error(`Select "${groupLabel}" not found in explore-toolbar-sk`);
        }
        return select;
      };
      return {
        dateMode: Boolean(dateInput.checked),
        evenXAxisSpacing: Boolean(findCheckboxByText('Even X-Axis Spacing').checked),
        showZero: Boolean(findCheckboxByText('Show Zero').checked),
        hoverMode: String(findSelectByGroupLabel('Hover:').value),
        showDots: Boolean(findCheckboxByText('Dots').checked),
        showRegressions: Boolean(findCheckboxByText('Show Regressions').checked),
        normalizeCentre: String(findSelectByGroupLabel('Center:').value),
      };
    });
  }

  /**
   * Opens the pill dropdown for `key` on the nth query bar and clicks `optionValue`
   * to toggle it selected/unselected.
   */
  public async togglePillOption(barIndex: number, key: string, optionValue: string): Promise<void> {
    const bar = this.queryBars.nth(barIndex);
    await bar.evaluate(
      async (el: any, detail: { key: string; optionValue: string }) => {
        const sr = el.shadowRoot || el;
        const selects = Array.from(sr.querySelectorAll('explore-multi-v2-select-sk')) as any[];
        const pill = selects.find((s) => s.label === detail.key);
        if (!pill) {
          throw new Error(`Pill "${detail.key}" not found`);
        }
        const pillSr = pill.shadowRoot || pill;
        const trigger = pillSr.querySelector('.multiselect-trigger') as HTMLElement | null;
        if (trigger && !pill.isOpen) {
          trigger.click();
          await el.updateComplete;
          await pill.updateComplete;
        }
        const options = Array.from(pillSr.querySelectorAll('.multiselect-option')) as HTMLElement[];
        const target = options.find(
          (o) => (o.querySelector('.ms-opt-value')?.textContent || '').trim() === detail.optionValue
        );
        if (!target) {
          throw new Error(`Option "${detail.optionValue}" not found in pill "${detail.key}"`);
        }
        target.click();
        await el.updateComplete;
        await pill.updateComplete;
      },
      { key, optionValue }
    );
    await this.waitForIdle();
  }

  /**
   * Dispatches a Ctrl+Click on the `pillIndex`-th pill in the `barIndex`-th query bar
   * and returns the indices of highlighted pills.
   */
  public async ctrlClickPill(barIndex: number, pillIndex: number): Promise<number[]> {
    const bar = this.queryBars.nth(barIndex);
    return bar.evaluate(async (el: any, idx: number) => {
      const sr = el.shadowRoot || el;
      const selects = Array.from(sr.querySelectorAll('explore-multi-v2-select-sk')) as any[];
      const target = selects[idx];
      if (!target) {
        throw new Error(`Pill at index ${idx} not found`);
      }
      target.dispatchEvent(
        new MouseEvent('click', { ctrlKey: true, bubbles: true, composed: true })
      );
      await el.updateComplete;
      return selects.map((s, i) => (s.isHighlighted ? i : -1)).filter((i) => i >= 0);
    }, pillIndex);
  }

  /**
   * Dispatches a viewport-changed event on the first trace-chart-sk to zoom/synchronize
   * across all split charts.
   */
  public async selectViewportRange(minCommit: number, maxCommit: number): Promise<void> {
    await this.charts.first().evaluate(
      async (chart: any, range: { minCommit: number; maxCommit: number }) => {
        chart.dispatchEvent(
          new CustomEvent('viewport-changed', {
            detail: range,
            bubbles: true,
            composed: true,
          })
        );
        const root = document.querySelector('explore-multi-v2-sk') as any;
        await root?.updateComplete;
      },
      { minCommit, maxCommit }
    );
    await this.page.waitForTimeout(150);
  }

  /**
   * Returns { minCommit, maxCommit } for each rendered trace-chart-sk.
   */
  public async getChartViewports(): Promise<Array<{ minCommit: number; maxCommit: number }>> {
    return this.root.evaluate((el: any) => {
      const sr = el.shadowRoot || el;
      const charts = Array.from(sr.querySelectorAll('trace-chart-sk')) as any[];
      return charts.map((c) => {
        const rows = c.series?.[0]?.rows || [];
        const fallbackMin = rows.length > 0 ? rows[0].commit_number : 0;
        const fallbackMax = rows.length > 0 ? rows[rows.length - 1].commit_number : 0;
        return {
          minCommit: Number(c.viewportMinX ?? fallbackMin),
          maxCommit: Number(c.viewportMaxX ?? fallbackMax),
        };
      });
    });
  }

  /**
   * Clicks the Reset Zoom button in the toolbar.
   */
  public async resetZoom(): Promise<void> {
    await this.toolbar.evaluate((el: any) => {
      const sr = el.shadowRoot || el;
      const buttons = Array.from(sr.querySelectorAll('button.custom-btn')) as HTMLButtonElement[];
      const resetBtn = buttons.find((b) => (b.textContent || '').trim() === 'Reset Zoom');
      if (!resetBtn || resetBtn.disabled) {
        throw new Error('Reset Zoom button.custom-btn not found or disabled in explore-toolbar-sk');
      }
      resetBtn.click();
    });
    await this.page.waitForTimeout(150);
  }

  /**
   * Pins the hover tooltip on the anomaly point at `commitNumber` on `chartIndex`-th
   * trace-chart-sk and returns the rendered tooltip metadata.
   */
  public async hoverAnomalyOnChart(
    chartIndex: number,
    commitNumber: number
  ): Promise<{ text: string; hasTriageMenu: boolean; hasBisectButton: boolean }> {
    return this.charts.nth(chartIndex).evaluate(async (chart: any, commit: number) => {
      const regressions = chart.regressions || {};
      const seriesList: any[] = chart._processedSeries?.length
        ? chart._processedSeries
        : chart.series || [];
      let chosenSeries = seriesList.find((s) => regressions[s.id]?.[commit]);
      if (!chosenSeries) {
        chosenSeries = seriesList[0];
      }
      if (!chosenSeries) {
        throw new Error('No series found on trace-chart-sk');
      }
      const row = (chosenSeries.rows || []).find((r: any) => r.commit_number === commit);
      if (!row) {
        throw new Error(`Row for commit ${commit} not found on series ${chosenSeries.id}`);
      }
      chart._pinned = true;
      chart._hoveredPoint = {
        series: chosenSeries,
        row,
        x: 120,
        y: 80,
      };
      await chart.updateComplete;
      const sr = chart.shadowRoot || chart;
      const tooltip = sr.querySelector('trace-chart-tooltip-sk') as any;
      if (tooltip?.updateComplete) {
        await tooltip.updateComplete;
      }
      const tooltipSr = tooltip?.shadowRoot || tooltip;
      const triageMenu = tooltipSr?.querySelector('triage-menu-sk#tooltip-triage-menu');
      const bisectBtn = tooltipSr?.querySelector('button#bisect') as HTMLButtonElement | null;
      return {
        text: (tooltipSr?.textContent || '').replace(/\s+/g, ' ').trim(),
        hasTriageMenu: !!triageMenu,
        hasBisectButton: !!bisectBtn && !bisectBtn.hidden,
      };
    }, commitNumber);
  }

  /**
   * Exercises triage-menu-sk#tooltip-triage-menu inside trace-chart-tooltip-sk on the
   * chartIndex-th trace-chart-sk by clicking its rendered action buttons.
   */
  public async triageTooltipAnomaly(
    chartIndex: number,
    action: 'ignore' | 'new-bug' | 'existing-bug',
    existingBugId?: string
  ): Promise<void> {
    await this.charts.nth(chartIndex).evaluate(
      async (
        chart: any,
        params: { action: 'ignore' | 'new-bug' | 'existing-bug'; existingBugId?: string }
      ) => {
        const sr = chart.shadowRoot || chart;
        const tooltip = sr.querySelector('trace-chart-tooltip-sk') as any;
        const tooltipSr = tooltip?.shadowRoot || tooltip;
        const triageMenu = tooltipSr?.querySelector('triage-menu-sk#tooltip-triage-menu') as any;
        if (!triageMenu) {
          throw new Error('triage-menu-sk#tooltip-triage-menu not found');
        }
        if (params.action === 'ignore') {
          const ignoreBtn = triageMenu.querySelector(
            'button#ignore:not([disabled]):not([hidden])'
          ) as HTMLButtonElement | null;
          if (!ignoreBtn) {
            throw new Error('button#ignore not found or disabled in triage-menu-sk');
          }
          ignoreBtn.click();
        } else if (params.action === 'new-bug') {
          const newBugBtn = triageMenu.querySelector(
            'button#new-bug:not([disabled]):not([hidden])'
          ) as HTMLButtonElement | null;
          if (!newBugBtn) {
            throw new Error('button#new-bug not found or disabled in triage-menu-sk');
          }
          newBugBtn.click();
        } else if (params.action === 'existing-bug') {
          const existingBugBtn = triageMenu.querySelector(
            'button#existing-bug:not([disabled]):not([hidden])'
          ) as HTMLButtonElement | null;
          if (!existingBugBtn) {
            throw new Error('button#existing-bug not found or disabled in triage-menu-sk');
          }
          existingBugBtn.click();
          await triageMenu.updateComplete;
          const existingBugDialog = triageMenu.querySelector('existing-bug-dialog-sk') as any;
          const dialogEl = existingBugDialog?.querySelector(
            'dialog#existing-bug-dialog[open]'
          ) as HTMLDialogElement | null;
          if (!dialogEl) {
            throw new Error('dialog#existing-bug-dialog is not open');
          }
          const input = dialogEl.querySelector('input#bug_id') as HTMLInputElement | null;
          if (!input || !params.existingBugId) {
            throw new Error('input#bug_id or existingBugId missing in existing-bug-dialog-sk');
          }
          input.value = params.existingBugId;
          input.dispatchEvent(new Event('input', { bubbles: true }));
          const submitBtn = dialogEl.querySelector(
            'button#file-button:not([disabled])'
          ) as HTMLButtonElement | null;
          if (!submitBtn) {
            throw new Error('button#file-button not found or disabled in existing-bug-dialog-sk');
          }
          const form = dialogEl.querySelector('form#existing-bug-form') as HTMLFormElement | null;
          form?.addEventListener(
            'submit',
            (e: Event) => {
              e.preventDefault();
              e.stopImmediatePropagation();
            },
            { capture: true, once: true }
          );
          const changedPromise = new Promise<void>((resolve, reject) => {
            existingBugDialog.addEventListener(
              'anomaly-changed',
              (e: any) => {
                const updatedAnomalies = e.detail?.anomalies || [];
                const expectedBug = Number(params.existingBugId);
                if (
                  updatedAnomalies.length === 0 ||
                  !updatedAnomalies.every((a: any) => Number(a.bug_id) === expectedBug)
                ) {
                  reject(
                    new Error(
                      `existing-bug-dialog-sk did not update anomalies bug_id to ${expectedBug}`
                    )
                  );
                  return;
                }
                resolve();
              },
              { once: true }
            );
          });
          submitBtn.click();
          await changedPromise;
        }
      },
      { action, existingBugId }
    );
    await this.page.waitForTimeout(200);
  }

  /**
   * Clicks button#bisect inside trace-chart-tooltip-sk and submits the Pinpoint bisect dialog.
   */
  public async submitTooltipBisect(chartIndex: number): Promise<void> {
    await this.charts.nth(chartIndex).evaluate(async (chart: any) => {
      const sr = chart.shadowRoot || chart;
      const tooltip = sr.querySelector('trace-chart-tooltip-sk') as any;
      const tooltipSr = tooltip?.shadowRoot || tooltip;
      const bisectBtn = tooltipSr?.querySelector('button#bisect') as HTMLButtonElement | null;
      if (!bisectBtn) {
        throw new Error('button#bisect not found in trace-chart-tooltip-sk');
      }
      bisectBtn.click();
      await tooltip.updateComplete;
      const pinpointDialog = tooltipSr?.querySelector('#pinpoint-dialog-sk') as any;
      if (!pinpointDialog) {
        throw new Error('#pinpoint-dialog-sk not found in trace-chart-tooltip-sk');
      }
      if (!pinpointDialog.user) {
        pinpointDialog.user = 'test@example.com';
      }
      await pinpointDialog.updateComplete;
      pinpointDialog.submitBisect();
    });
    await this.page.waitForTimeout(250);
  }

  /**
   * Opens the Help Hub panel and starts the interactive tour.
   */
  public async startTour(): Promise<void> {
    const helpHub = this.root.locator('help-hub-sk');
    await helpHub.locator('.help-fab').evaluate((btn: HTMLButtonElement) => btn.click());
    await helpHub
      .locator('.tour-trigger-btn', { hasText: 'Start Interface Tour' })
      .evaluate((btn: HTMLButtonElement) => btn.click());
  }

  /**
   * Starts the interactive tour, steps through the first two tour bubbles,
   * dismisses via Skip, and returns the visited titles and remaining bubble count.
   */
  public async runInteractiveTour(): Promise<{
    visitedTitles: string[];
    activeButtonsCountAfterClose: number;
  }> {
    await this.startTour();
    const tourBubble = this.root.locator('interactive-tour-sk .tour-bubble');
    await tourBubble.waitFor({ state: 'visible', timeout: 5000 });
    const firstTitle = await tourBubble.locator('.bubble-title').innerText();
    await tourBubble.locator('.tour-btn-primary', { hasText: 'Next' }).click();
    const secondTitle = await tourBubble.locator('.bubble-title').innerText();
    await tourBubble.locator('.tour-btn-skip', { hasText: 'Skip' }).click();
    return {
      visitedTitles: [firstTitle, secondTitle],
      activeButtonsCountAfterClose: await tourBubble.count(),
    };
  }
}
