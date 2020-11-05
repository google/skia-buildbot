/**
 * @module modules/commands-sk

 * @description A list view of draw commands for a single frame, and a tool for
 * filtering them. contains the logic for processing the parsed json object from
 * wasm, and extracting things like layer info and command counts which drive
 * other modules.
 *
 * Contains play-sk as a submodule, which playes over the filtered list of
 * commands.
 *
 * Data flows along this path in one direction depending on which end triggers a
 * change.
 * filter text box <=> this._includedSet <=> histogram-sk
 *
 * @evt histogram-update: An event containing the list of histogram entries.
 *      Emitted every time the histogram is recomputed.
 *
 * @evt move-position: When the play-sk module or user selects a different
 * command, this event is emitted, and it's detail contains the command index in
 * the unfiltered command list for this frame.
 *
 */
import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { PlaySk, PlaySkMoveToEventDetail } from '../play-sk/play-sk';
import { HistogramSkToggleEventDetail } from '../histogram-sk/histogram-sk'

import 'elements-sk/icon/save-icon-sk';
import 'elements-sk/icon/content-copy-icon-sk';
import 'elements-sk/icon/image-icon-sk';

import { SkpJsonCommandList, SkpJsonCommand, SkpJsonAuditTrail, SkpJsonGpuOp } from '../debugger';

import '../play-sk';

export interface CommandsSkMovePositionEventDetail {
  // the index of a command in the frame to which the wasm view should move.
  position: number,
}

export type CommandRange = [number, number];

// Represents one of the icons that can appear on a command
export interface PrefixItem {
  icon: string,
  color: string,
  count: number,
};

/** A processed command object, created from a SkpJsonCommand */
export interface Command {
  // Index of the command in the unfiltered list.
  index: number,
  // the save/restore depth before this command is executed.
  depth: number,
  // the parsed json representation of the command. exact type depends on the command.
  details: SkpJsonCommand,
  name: string,
  // if this command is one of an indenting pair, the command index range that the pair enclose
  // (save, restore)
  range?: CommandRange,
  prefixes: PrefixItem[],
  // Whether the command will be executed during playback
  visible: boolean,
  // index of any image referenced by this command
  imageIndex?: number,
};

/** An entry of the command histogram
 *  obtained by totalling up occurances in the range filtered command list
 */
export interface HistogramEntry {
  // name of a command (original CamelCase)
  name: string,
  // number of occurances in the current frame (or the whole file for a single-frame SKP)
  countInFrame: number,
  // number of occurances in the current range filter
  countInRange: number,
}

/** An event detail containing a new histogram
 * or new filter set to be displayed by the histogram-sk module.
 * The event may update one or both of the two fields.
 */
export interface CommandsSkHistogramEventDetail {
  /** A newly computed histogram that needs to be displayed by histogram-sk */
  hist?: HistogramEntry[];
  /** whether the command is include by the filter */
  included?: Set<string>;
}

// Colors to use for gpu op ids
const COLORS = [
    "#1B9E77",
    "#D95F02",
    "#7570B3",
    "#E7298A",
    "#66A61E",
    "#E6AB02",
    "#A6761D",
    "#666666"
  ];
// Commands that increase save/restore depth
const INDENTERS: {[key: string]: PrefixItem} = {
  'Save':             { icon: 'save-icon-sk',         color: '#B2DF8A', count: 1 },
  'SaveLayer':        { icon: 'content-copy-icon-sk', color: '#FDBF6F', count: 1 },
  'BeginDrawPicture': { icon: 'image-icon-sk',        color: '#A6CEE3', count: 1 },
};
// commands that decrease save/restore depth
const OUTDENTERS: string[] = ['Restore', 'EndDrawPicture'];

export class CommandsSk extends ElementSk {
  private static template = (ele: CommandsSk) =>
    html`
    <div>
      ${CommandsSk.filterTemplate(ele)}
      <div class="horizontal-flex">
        <button @click=${ele._opIdFilter} class="short">Show By Op-Id</button>
        <play-sk .visual=${'full'}></play-sk>
      </div>
      <div class="list">
        ${ ele._filtered.map((i: number, filtPos: number) =>
          CommandsSk.opTemplate(ele, filtPos, ele._cmd[i])) }
      </div>
    </div>`;

  private static opTemplate = (ele: CommandsSk, filtpos: number, op: Command) =>
    html`<div class="op" id="op-${op.index}" @click=${() => {ele.item = filtpos}}>
      <details>
        <summary class=${ ele.position == op.index ? 'selected' : ''}>
          <span class="index">${op.index}</span>
          ${ op.prefixes.map((pre: PrefixItem) =>
            CommandsSk.prefixItemTemplate(ele, pre)) }
          <span>${ op.name }</span>
          <code>${ op.details.shortDesc }</code>
          ${ op.range
            ? html`<button @click=${() => {ele.range = op.range!}}
        title="Range-filter the command list to this save/restore pair">Zoom</button>`
            : ''
          }
          ${ op.imageIndex
            ? html`<button @click=${ele._jumpToImage(op.imageIndex)}
                title="Show the image referenced by this command in the resource viewer"
                >Show image</button>`
            : ''
          }
          ${ (op.details.auditTrail && op.details.auditTrail.Ops)
            ? op.details.auditTrail.Ops.map((gpuOp: SkpJsonGpuOp) =>
                CommandsSk.gpuOpIdTemplate(ele, gpuOp) )
            : ''
          }
        </summary>
        <div>
          <checkbox-sk title="Toggle command visibility" checked=${ op.visible }
                       @change=${ele._toggleVisible(op.index)}></checkbox-sk>
          <strong>Index: </strong> <span class=index>${op.index}</span>
        </div>
        ${ele._renderRullOpRepresentation(op)}
      </details>
    </div>
    <hr>`;

  private static prefixItemTemplate = (ele: CommandsSk, item: PrefixItem) =>
    html`${ ele._icon(item) }
      ${ item.count > 1
        ? html`<span title="depth of indenting operation"
          class=count>${ item.count }</span>`
        : ''
      }`;

  private static gpuOpIdTemplate = (ele: CommandsSk, gpuOp: SkpJsonGpuOp) =>
    html`<span title="GPU Op ID - group of commands this was executed with on the GPU"
            class=gpu-op-id style="background: ${ ele._gpuOpColor(gpuOp.OpsTaskID) }"
      >${ gpuOp.OpsTaskID }</span>`;

  private static filterTemplate = (ele: CommandsSk) =>
    html`
    <div class="horizontal-flex">
      <label title="Filter command names (Single leading ! negates entire filter).
Command types can also be filted by clicking on their names in the histogram"
        >Filter</label>
      <input @change=${ele._textFilter} value="!DrawAnnotation"
             id="text-filter"></input>&nbsp;
      <label>Range</label>
      <input @change=${ele._rangeInputHandler} class=range-input value="${ ele._range[0] }"
             id="rangelo"></input>
      <b>:</b>
      <input @change=${ele._rangeInputHandler} class=range-input value="${ ele._range[1] }"
             id="rangehi"></input>
      <button @click=${ele.clearFilter} id="clear-filter-button">Clear</button>
    </div>`;

  // processed command list (no filtering applied). change with processCommands
  private _cmd: Command[] = [];
  // list of indices of commands that passed the range and name filters.
  private _filtered: number[] = [];
  // position in filtered (visible) command list
  private _item: number = 0;
  // range filter
  private _range: CommandRange = [0, 100];
  // counts of command occurances
  private _histogram: HistogramEntry[] = [];
  // known command names (set by processCommands) names are lowercased.
  private _available = new Set<string>();
  // subset of command names that should pass the command filter
  // (names are lowercased)
  private _includedSet = new Set<string>();
  // Play bar submodule
  private _playSk: PlaySk | null = null;

  // the command count with no filtering
  get count() {
    return this._cmd.length;
  }
  // the command count with all filters applied
  get countFiltered() {
    return this._filtered.length;
  }

  // set the current playback position in the list
  // (index in filtered list)
  set item(i: number) {
    this._item = i;
    this.querySelector<HTMLDivElement>('#op-' + this._filtered[this._item]
      )?.scrollIntoView({block: 'nearest'});
    this._render();
    // notify debugger-page-sk that it needs to draw this.position
    this.dispatchEvent(
      new CustomEvent<CommandsSkMovePositionEventDetail>(
        'move-position', {
          detail: {position: this.position},
          bubbles: true,
        }));
    this._playSk!.movedTo(this._item);
  }

  // get the playback index in _cmd after filtering is applied.
  get position() {
    return this._filtered[this._item];
  }

  set range(range: CommandRange) {
    this._range = range;
    this._applyRangeFilter();
  }

  set textFilter(q: string) {
    this.querySelector<HTMLInputElement>('#text-filter')!.value = q;
    if (!this.count) { return; }
    this._textFilter(); // does render
  }

  // Return a list of op indices that pass the current filters.
  get filtered(): number[] {
    return this._filtered;
  }

  constructor() {
    super(CommandsSk.template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();

    this._playSk = this.querySelector<PlaySk>('play-sk')!;

    this._playSk.addEventListener('moveto', (e) => {
      this.item = (e as CustomEvent<PlaySkMoveToEventDetail>).detail.item;
    });

    document.addEventListener('toggle-command-inclusion', (e) => {
      this._toggleName((e as CustomEvent<HistogramSkToggleEventDetail>).detail.name);
    });
  }

  // _processCommands iterates over the commands to extract several things.
  //  1. A depth at every command based on Save/Restore pairs.
  //  2. A histogram showing how many times each type of command is used.
  //  3. A map from layer node ids to the index of any layer use events in the
  //     command list.
  processCommands(cmd: SkpJsonCommandList) {
    const commands: Command[] = [];
    let depth = 0;
    const prefixes: PrefixItem[] = []; // A stack of indenting commands
    const matchup: number[] = []; // Match up saves and restores, a stack of indices
    // (not exhaustive)
    const renderNodeRe = /^RenderNode\(id=([0-9]+), name='([A-Za-z0-9_]+)'\)/;
    // all available command types
    this._available = new Set<string>();
    for (let i = 0; i < cmd.commands.length; i++) {
      const name = cmd.commands[i].command;
      this._available.add(name.toLowerCase());

      const out: Command = {
        index: i,
        depth: depth,
        details: cmd.commands[i], // unaltered object from json
        name: name,
        prefixes: [],
        visible: true,
      };

      if (cmd.commands[i].imageIndex) {
        out.imageIndex = cmd.commands[i].imageIndex;
      }

      if (name in INDENTERS) {
        depth++;

        matchup.push(i);
        // If this is the same type of indenting op we've already seen
        // then just increment the count, otherwise add as a new
        // op in prefixes.
        if (depth > 1 && prefixes[prefixes.length-1].icon
            == INDENTERS[name].icon) {
          prefixes[prefixes.length-1].count++;
        } else {
          prefixes.push(this._copyPrefix(INDENTERS[name]));
        }
      } else if (OUTDENTERS.indexOf(name) !== -1) {
        depth--;

        // Now that we can match an OUTDENTER with an INDENTER we can set
        // the _zoom property for both commands.
        const begin: number = matchup.pop()!;
        const range = [begin, i] as CommandRange;
        out.range = range;
        commands[begin].range = range;

        // Only pop the op from prefixes if its count has reached 1.
        if (prefixes[prefixes.length-1].count > 1) {
          prefixes[prefixes.length-1].count--;
        } else {
          prefixes.pop();
        }
        out.depth = depth;
      }
      // TODO(nifong): extract layer and image resource data

      // deep copy prefixes because we want a snapshot of the current list and counts
      out.prefixes = prefixes.map((p: PrefixItem) => this._copyPrefix(p));

      commands.push(out);
    }

    this._cmd = commands;
    this.range = [0, this._cmd.length-1]; // this assignment also triggers render
  }

  // User clicked the clear filter button, clear both filters
  clearFilter() {
    this.querySelector<HTMLInputElement>('#text-filter')!.value = '';
    if (!this.count) { return; }
    this.range = [0, this._cmd.length-1]; // setter triggers _applyRangeFilter, follow that
  }

  // Stop playback and move by a given offset in the filtered list.
  keyMove(offset: number) {
    this._playSk!.mode = 'pause';
    this.item = Math.max(0, Math.min(this._item + offset, this.countFiltered));
  }

  // filter change coming from histogram
  private _toggleName(name: string) {
    const lowerName = name.toLowerCase();
    if (!this._available.has(lowerName)) {
      return;
    }
    if (this._includedSet.has(lowerName)) {
      this._includedSet.delete(lowerName);
    } else {
      this._includedSet.add(lowerName);
    }

    // represent _includedSet as a negative text filter and put it in the box
    const diff = new Set(this._available);
    for (let c of this._includedSet) {
        diff.delete(c)
    }
    let filter = '';
    if (diff.size > 0) {
      filter = '!'+Array.from(diff).join(' ');
    }
    this.querySelector<HTMLInputElement>('#text-filter')!.value = filter;
    // don't trigger _textFilter() since that would send an event back to histogram and
    // start an infinite loop. this._includedSet is correct, apply it and render.
    this._applyCommandFilter();
  }

  // TODO(nifong): make this smarter, show matrices as tables, colors as colors, etc
  private _renderRullOpRepresentation(op: Command){
    return html`<pre>${ JSON.stringify(op.details, null, 2) }</pre>`;
  }

  // TODO(nifong): implement after adding resource tab
  private _jumpToImage(index: number){}

  // (index is in the unfiltered list)
  private _toggleVisible(index: number){
    this._cmd[index].visible = !this._cmd[index].visible;
  }

  // lit-html does not appear to support setting a tag's name with a ${} so here's
  // a crummy workaround
  private _icon(item: PrefixItem) {
    if (item.icon === 'save-icon-sk') {
      return html`<save-icon-sk style="fill: ${ item.color };"
        class=icon> </save-icon-sk>`;
    } else if (item.icon === 'content-copy-icon-sk') {
      return html`<content-copy-icon-sk style="fill: ${ item.color };"
        class=icon> </content-copy-icon-sk>`;
    } else if (item.icon === 'image-icon-sk') {
      return html`<image-icon-sk style="fill: ${ item.color };"
        class=icon> </image-icon-sk>`;
    }
  }

  // Any deterministic mapping between integers and colors will do
  private _gpuOpColor(index: number) {
    return COLORS[index % COLORS.length];
  }

  // deep copy
  private _copyPrefix(p: PrefixItem): PrefixItem {
    return {icon: p.icon, color: p.color, count: p.count};
  }

  private _rangeInputHandler(e: Event) {
    const lo = parseInt(this.querySelector<HTMLInputElement>('#rangelo')!.value);
    const hi = parseInt(this.querySelector<HTMLInputElement>('#rangehi')!.value);
    this.range = [lo, hi];
  }

  // parse the text filter input, and if it is possible to represent it purely as
  // a command filter, store it in this._includedSet
  private _textFilter() {
    let rawFilter = this.querySelector<HTMLInputElement>('#text-filter'
      )!.value.trim().toLowerCase();
    const negative = (rawFilter[0] == '!');

    // make sure to copy it so we don't alter this._available
    this._includedSet = new Set<string>(this._available);

    if (rawFilter !== '') {
      if (negative) {
        rawFilter = rawFilter.slice(1).trim();
        const tokens = rawFilter.split(/\s+/);
        // negative filters can always be represented with histogram selections
        for (const token of tokens) {
          this._includedSet.delete(token);
        }
      } else {
        // for positive filters, the text could either be a set of command names,
        // or a free text search.
        const tokens = rawFilter.split(/\s+/);
        this._includedSet = new Set<string>();
        for (const token of tokens) {
          if (this._available.has(token)) {
            this._includedSet.add(token);
          } else {
            // not a command name, bail out, reset this, do a free text search
            this._includedSet = new Set<string>(this._available);
            // since we just altered this._includedSet we have to let histogram know.
            this.dispatchEvent(new CustomEvent<CommandsSkHistogramEventDetail>(
              'histogram-update', {
                detail: { included: new Set<string>(this._includedSet) },
                bubbles: true,
              }));
            this._freeTextSearch(tokens);
            // TODO(nifong): need some visual feedback to let the user know
            console.log(`Query interpreted as free text search becauuse ${token}
doesn't appear to be a command name`);
            return;
          }
        }
      }
    }
    this.dispatchEvent(new CustomEvent<CommandsSkHistogramEventDetail>(
        'histogram-update', {
          detail: { included: new Set<string>(this._includedSet) },
          bubbles: true,
        }));
    this._applyCommandFilter(); // note we still do this for emtpy filters.
  }

  private _freeTextSearch(tokens: string[]) {
    // Free text search every command's json representation and include its index in
    // this._filtered if any token is found
    const matches = function(s: string) {
      for (const token of tokens) {
        if (s.indexOf(token) >= 0) {
          return true;
        }
      }
      return false;
    }
    this._filtered = [];
    for (let i = this._range[0]; i <= this._range[1]; i++) {
      const commandText = JSON.stringify(this._cmd[i].details).toLowerCase();
      if (matches(commandText)) {
        this._filtered.push(i);
      }
    }
    this._render();
    if (this._filtered.length > 0) {
      this.item = this._filtered.length - 1; // after render because it causes a scroll
    }
  }

  // Applies range filter and recalculates command name histogram.
  // The range filter is the first filter applied. The histogram shows any command
  // that passes
  // the range filter, and shows a nonzero count for any command that passes the command
  // filter.
  private _applyRangeFilter() {
    // Despite the name, there's not much to "apply" but
    // the histogram needs to change when the range filter changes which is
    // why this function is seperate from _applyCommandFilter

    // Calculate data for histogram
    // Each command type gets two different counts
    interface tally {
      count_in_frame: number,
      count_in_range_filter: number,
    }
    const counts = new Map<string, tally>();
    for (let i = 0; i < this._cmd.length; i++) {
      let c = this._cmd[i];
      if (!counts.get(c.name)) {
        counts.set(c.name, {
          count_in_frame: 0,
          count_in_range_filter: 0
        });
      }
      counts.get(c.name)!.count_in_frame += 1; // always increment first count
      if (i >= this._range[0] && i <= this._range[1]) {
        counts.get(c.name)!.count_in_range_filter += 1; // optionally increment filtered count.
      }
    }

    // Now format the histogram as a sorted array suitable for use in the template.
    // First convert the counts map into an Array of HistogramEntry.
    this._histogram = [];
    counts.forEach((value, key) => {
      this._histogram.push({
        name: key,
        countInFrame: value.count_in_frame,
        countInRange: value.count_in_range_filter,
      })
    });
    // Now sort the array, descending on the rangeCount, ascending
    // on the op name.
    // sort by rangeCount so entries don't move on enable/disable
    this._histogram.sort(function(a,b) {
      if (a.countInRange == b.countInRange) {
        if (a.name < b.name) {
          return -1;
        }
        if (a.name > b.name) {
          return 1;
        }
        return 0;
      } else {
        return b.countInRange - a.countInRange;
      }
    });

    // the user's selections are present in the text filter. Apply them now
    // triggers render
    this._textFilter();
    // that populated this._includedSet, which we also need to notify histogram of.

    // send this to the histogram element
    this.dispatchEvent(
      new CustomEvent<CommandsSkHistogramEventDetail>(
        'histogram-update', {
          detail: {
            hist: this._histogram,
            // Make a copy so listener can't accidently write to it.
            included: new Set<string>(this._includedSet),
          },
          bubbles: true,
        }));
  }

  // Apply a filter specified by this._includedSet and set the filtered list to be visible.
  private _applyCommandFilter() {
    // Try to retain the user's playback position in the unfiltered list when doing this
    // (it is not always possible)
    const oldPos = this._filtered[this._item];
    let newPos: number | null = null;
    this._filtered = [];
    for (let i = this._range[0]; i <= this._range[1]; i++) {
      if (this._includedSet.has(this._cmd[i].name.toLowerCase())) {
        this._filtered.push(i);
        if (i === oldPos) {
          newPos = this._filtered.length - 1;
        }
      }
    }
    this._playSk!.size = this._filtered.length;
    this._render(); // gotta render before you can scroll
    if (newPos !== null) {
      this.item = newPos; // setter triggers scroll
    } else {
      this.item = this._filtered.length - 1;
    }
  }

  // Filters out all but the last command of each gpu op group
  // Experimental, probably breaks assumptions elsewhere
  private _opIdFilter() {
    this._filtered = [];

    const commandsOfEachOp = new Map<number, number[]>();
    this._cmd.forEach((command, index) => {
      if (command.details.auditTrail &&
          command.details.auditTrail.Ops) {
        const opid = command.details.auditTrail.Ops[0].OpsTaskID;
        if (!commandsOfEachOp.has(opid)) {
          commandsOfEachOp.set(opid, []);
        }
        commandsOfEachOp.get(opid)!.push(index);
      }
    });
    const sortedKeys: number[] = Array.from(commandsOfEachOp.keys());
    sortedKeys.sort((a, b) => a - b); // force it to sort as a number, not a string
    sortedKeys.forEach((k) => {
      commandsOfEachOp.get(k)!.forEach((i) => {
        this._filtered.push(i);
      });
    });


    this._playSk!.size = this._filtered.length;
    this.item = this._filtered.length - 1;
  }
}

define('commands-sk', CommandsSk);
