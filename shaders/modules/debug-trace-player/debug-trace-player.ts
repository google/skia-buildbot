import { DebugTrace, SlotInfo } from '../debug-trace/debug-trace';

// The TraceOp enum must stay in sync with SkSL::SkVMTraceInfo::Op.
enum TraceOp {
  Line  = 0,
  Var   = 1,
  Enter = 2,
  Exit  = 3,
  Scope = 4,
}

// The NumberKind enum must stay in sync with SkSL::Type::NumberKind.
enum NumberKind {
  Float      = 0,
  Signed     = 1,
  Unsigned   = 2,
  Boolean    = 3,
  Nonnumeric = 4,
}

// Trace data comes in from the JSON as a number[]. We unpack it into a TraceInfo for ease of use.
type TraceInfo = {
  op: TraceOp;
  data: number[];
};

type StackFrame = {
  // A FunctionInfo from trace.functions.
  func: number;
  // The current line number within the function.
  line: number;
  // Any variable slots which have been touched in this function.
  displayMask: boolean[];
};

type Slot = {
  // The current raw value held in this slot (as a 32-bit integer, not bit-punned).
  value: number;
  // The scope depth associated with this slot (as indicated by trace_scope).
  scope: number;
  // When was the variable in this slot most recently written? (as a cursor position)
  writeTime: number;
};

export type VariableData = {
  // A SlotInfo from trace.slots.
  slotIndex: number;
  // Has this slot been written-to since the last step call?
  dirty: boolean;
  // The current value held in this slot (properly bit-punned/cast to the expected type)
  value: number | boolean;
};

export class DebugTracePlayer {
  private trace: DebugTrace | null = null;

  // The position of the read head within the trace array.
  private cursor: number = 0;

  // Tracks the current scope depth (as indicated by trace_scope).
  private scope: number = 0;

  // Tracks assignments into our data slots.
  private slots: Slot[] = [];

  // Tracks the trace stack (as indicated by trace_enter and trace_exit).
  private stack: StackFrame[] = []; // the execution stack

  // Tracks which line numbers are reached by the trace, and the number of times it's reached.
  private lineNumbers: Map<number, number> = new Map();

  // Tracks all the data slots which have been touched during the current step.
  private dirtyMask: boolean[] = [];

  // Tracks all the data slots which hold function return values.
  private returnValues: boolean[] = [];

  // Tracks line numbers that have breakpoints set on them.
  private breakpointLines: Set<number> = new Set();

  /** Throws an error if a precondition is not met. Indicates a logic bug or invalid trace. */
  private check(result: boolean): void {
    if (!result) {
      throw new Error('check failed');
    }
  }

  /** Copies trace info from the JSON number array into a TraceInfo struct. */
  private getTraceInfo(position: number): TraceInfo {
    this.check(position < this.trace!.trace.length);
    this.check(this.trace!.trace[position][0] in TraceOp);

    const info: TraceInfo = {
      op: this.trace!.trace[position][0] as TraceOp,
      data: this.trace!.trace[position].slice(1),
    };
    return info;
  }

  /** Resets playback to the start of the trace. Breakpoints are not cleared. */
  public reset(trace: DebugTrace | null): void {
    const nslots = trace?.slots?.length ?? 0;

    const globalStackFrame: StackFrame = {
      func: -1,
      line: -1,
      displayMask: Array<boolean>(nslots).map(() => (false)),
    };

    this.trace = trace;
    this.cursor = 0;
    this.slots = [];
    this.stack = [globalStackFrame];
    this.dirtyMask = Array<boolean>(nslots).map(() => (false));
    this.returnValues = Array<boolean>(nslots).map(() => (false));

    if (trace !== null) {
      this.slots = trace.slots.map((): Slot => ({
        value: 0,
        scope: Infinity,
        writeTime: 0,
      }));
      this.returnValues = trace.slots.map((slotInfo: SlotInfo): boolean =>
        (slotInfo.retval ?? -1) >= 0
      );

      // Build a map holding the number of times each line is reached.
      this.lineNumbers.clear();
      trace.trace.forEach((_, traceIdx: number) => {
        const info: TraceInfo = this.getTraceInfo(traceIdx);
        if (info.op === TraceOp.Line) {
          const lineNumber = info.data[0];
          const lineCount = this.lineNumbers.get(lineNumber) ?? 0;
          this.lineNumbers.set(lineNumber, lineCount + 1);
        }
      });
    }
  }

  /** Advances the simulation to the next Line op. */
  public step(): void {
    this.tidyState();
    while (!this.traceHasCompleted()) {
      if (this.execute(this.cursor++)) {
        break;
      }
    }
  }

  /**
   * Advances the simulation to the next Line op, skipping past matched Enter/Exit pairs.
   * Breakpoints will also stop the simulation even if we haven't reached an Exit.
   */
  public stepOver(): void {
    this.tidyState();
    const initialStackDepth = this.stack.length;

    while (!this.traceHasCompleted()) {
      const canEscapeFromThisStackDepth = (this.stack.length <= initialStackDepth);
      if (this.execute(this.cursor++)) {
        if (canEscapeFromThisStackDepth || this.atBreakpoint()) {
          break;
        }
      }
    }
  }

  public stepOut() : void {
    this.tidyState();
    const initialStackDepth = this.stack.length;

    while (!this.traceHasCompleted()) {
      if (this.execute(this.cursor++)) {
        const hasEscapedFromInitialStackDepth = (this.stack.length < initialStackDepth);
        if (hasEscapedFromInitialStackDepth || this.atBreakpoint()) {
          break;
        }
      }
    }
  }

  public run() : void {
    this.tidyState();

    while (!this.traceHasCompleted()) {
      if (this.execute(this.cursor++)) {
        if (this.atBreakpoint()) {
          break;
        }
      }
    }
  }

  /**
   * Cleans up temporary state between steps, such as the dirty mask and function return values.
   */
  private tidyState(): void {
    this.dirtyMask.fill(false);

    const stackTop = this.stack[this.stack.length - 1];
    this.returnValues.forEach((_, slotIdx: number) => {
      stackTop.displayMask[slotIdx] &&= !this.returnValues[slotIdx];
    });
  }

  /** Returns true if we have reached the end of the trace. */
  public traceHasCompleted(): boolean {
    return (this.trace == null) || (this.cursor >= this.trace.trace.length);
  }

  /** Reports the position of the cursor "read head" within the array of trace instructions. */
  public getCursor(): number {
    return this.cursor;
  }

  /** Returns true if the current line has a breakpoint set on it. */
  public atBreakpoint(): boolean {
    return this.breakpointLines.has(this.getCurrentLine());
  }

  /** Replaces all current breakpoints with a new set of them. */
  public setBreakpoints(breakpointLines: Set<number>): void {
    this.breakpointLines = breakpointLines;
  }

  /** Adds a breakpoint to a line (if one doesn't exist). */
  public addBreakpoint(line: number): void {
    this.breakpointLines.add(line);
  }

  /** Removes a breakpoint from a line (if one exists). */
  public removeBreakpoint(line: number): void {
    this.breakpointLines.delete(line);
  }

  /** Retrieves the current line. */
  public getCurrentLine(): number {
    this.check(this.stack.length > 0);
    return this.stack[this.stack.length - 1].line;
  }

  /**
   * Returns every line number reached inside this debug trace, along with the remaining number of
   * times that this trace will reach it. e.g. {100, 2} means line 100 will be reached twice.
   */
  public getLineNumbersReached(): Map<number, number> {
    return this.lineNumbers;
  }

  /** Returns the call stack as an array of FunctionInfo indices. */
  public getCallStack(): number[] {
    this.check(this.stack.length > 0);
    return this.stack.slice(1).map((frame: StackFrame) => {
      return frame.func;
    });
  }

  /** Returns the size of the call stack. */
  public getStackDepth(): number {
    this.check(this.stack.length > 0);
    return this.stack.length - 1;
  }

  /** Returns a slot's component as a variable-name suffix, e.g. ".x" or "[2][2]". */
  public getSlotComponentSuffix(slotIndex: number): string {
    const slot: SlotInfo = this.trace!.slots[slotIndex];

    if (slot.rows > 1) {
      return "[" + Math.floor(slot.index / slot.rows) + "][" + slot.index % slot.rows + "]";
    }
    if (slot.columns > 1) {
      switch (slot.index) {
        case 0:  return '.x';
        case 1:  return '.y';
        case 2:  return '.z';
        case 3:  return '.w';
        default: return '[???]';
      }
    }
    return '';
  }

  /** Bit-casts a value for a given slot into a double, honoring the slot's NumberKind. */
  private interpretValueBits(slotIdx: number, valueBits: number): number | boolean {
    const bitArray: Int32Array = new Int32Array(1);
    bitArray[0] = valueBits;
    switch (this.trace!.slots[slotIdx].kind) {
      case NumberKind.Float:    return new Float32Array(bitArray.buffer)[0];
      case NumberKind.Unsigned: return new Uint32Array(bitArray.buffer)[0];
      case NumberKind.Boolean:  return (valueBits !== 0);
      case NumberKind.Signed:   return valueBits;
      default:                  return valueBits;
    }
  }

  /** Returns a vector of the indices and values of each slot that is enabled in `bits`. */
  private getVariablesForDisplayMask(displayMask: boolean[]): VariableData[] {
    this.check(displayMask.length === this.slots.length);

    let vars: VariableData[] = [];
    displayMask.forEach((_, slot: number) => {
      if (displayMask[slot]) {
        const varData: VariableData = {
          slotIndex: slot,
          dirty: this.dirtyMask[slot],
          value: this.interpretValueBits(slot, this.slots[slot].value),
        };
        vars.push(varData);
      }
    });

    // Order the variable list so that the most recently-written variables are shown at the top.
    vars = vars.sort((a: VariableData, b: VariableData) => {
      // Order by descending write-time.
      const delta = this.slots[b.slotIndex].writeTime - this.slots[a.slotIndex].writeTime;
      if (delta !== 0) {
        return delta;
      }

      // If write times match, order by ascending slot index (preserving the existing order).
      return a.slotIndex - b.slotIndex;
    });

    return vars;
  }

  /** Returns the variables in a given stack frame. */
  public getLocalVariables(stackFrameIndex: number): VariableData[] {
    // The first entry on the stack is the "global" frame before we enter main, so offset our index
    // by one to account for it.
    ++stackFrameIndex;
    this.check(stackFrameIndex > 0);
    this.check(stackFrameIndex <= this.stack.length);
    return this.getVariablesForDisplayMask(this.stack[stackFrameIndex].displayMask);
  }

  /** Returns the variables at global scope. */
  public getGlobalVariables(): VariableData[] {
    if (this.stack.length < 1) {
      return [];
    }
    return this.getVariablesForDisplayMask(this.stack[0].displayMask);
  }

  /** Updates fWriteTime for the entire variable at a given slot. */
  private updateVariableWriteTime(slotIdx: number, cursor: number): void {
    // The slotIdx could point to any slot within a variable.
    // We want to update the write time on EVERY slot associated with this variable.
    // The SlotInfo gives us enough information to find the affected range.
    const changedSlot = this.trace!.slots[slotIdx];
    slotIdx -= changedSlot.index;
    const lastSlotIdx = slotIdx + (changedSlot.columns * changedSlot.rows);

    for (; slotIdx < lastSlotIdx; ++slotIdx) {
      this.slots[slotIdx].writeTime = cursor;
    }
  }

  /**
   * Executes the trace op at the passed-in cursor position. Returns true if we've reached a line
   * or exit trace op, which indicate a stopping point.
   */
  private execute(position: number): boolean {
    const trace = this.getTraceInfo(position);
    this.check(this.stack.length > 0);
    const stackTop: StackFrame = this.stack[this.stack.length - 1];
    switch (trace.op) {
      case TraceOp.Line: { // data: line number, (unused)
        const lineNumber = trace.data[0];
        const lineCount = this.lineNumbers.get(lineNumber) ?? 0;
        this.check(lineNumber >= 0);
        this.check(lineNumber < this.trace!.source.length);
        this.check(lineCount > 0);
        stackTop.line = lineNumber;
        this.lineNumbers.set(lineNumber, lineCount - 1);
        return true;
      }
      case TraceOp.Var: { // data: slot, value
        const slotIdx = trace.data[0];
        const value = trace.data[1];
        this.check(slotIdx >= 0);
        this.check(slotIdx < this.slots.length);
        this.slots[slotIdx].value = value;
        this.slots[slotIdx].scope = Math.min(this.slots[slotIdx].scope, this.scope);
        this.updateVariableWriteTime(slotIdx, position);
        if ((this.trace!.slots[slotIdx].retval ?? -1) < 0) {
          // Normal variables are associated with the current function.
          stackTop.displayMask[slotIdx] = true;
        } else {
          // Return values are associated with the parent function (since the current function
          // is exiting and we won't see them there).
          this.check(this.stack.length > 1);
          this.stack[this.stack.length - 2].displayMask[slotIdx] = true;
        }
        this.dirtyMask[slotIdx] = true;
        break;
      }
      case TraceOp.Enter: { // data: function index, (unused)
        const fnIdx = trace.data[0];
        this.check(fnIdx >= 0);
        this.check(fnIdx < this.trace!.functions.length);
        const enteredStackFrame: StackFrame = {
          func: fnIdx,
          line: -1,
          displayMask: Array<boolean>(this.slots.length).fill(false),
        };
        this.stack.push(enteredStackFrame);
        break;
      }
      case TraceOp.Exit: { // data: function index, (unused)
        const fnIdx = trace.data[0];
        this.check(stackTop.func === fnIdx);
        this.stack.pop();
        return true;
      }
      case TraceOp.Scope: { // data: scope delta, (unused)
        const scopeDelta = trace.data[0];
        this.scope += scopeDelta;
        if (scopeDelta < 0) {
          // If the scope is being reduced, discard variables that are now out of scope.
          this.slots.forEach((_, slotIdx: number) => {
            if (this.scope < this.slots[slotIdx].scope) {
              this.slots[slotIdx].scope = Infinity;
              stackTop.displayMask[slotIdx] = false;
            }
          });
        }
        break;
      }
      default: {
        throw new Error('unrecognized trace instruction');
      }
    }
    return false;
  }
}
