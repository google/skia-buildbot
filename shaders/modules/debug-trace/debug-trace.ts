// These functions will throw an error if the JSON doesn't match the DebugTrace interface, even if
// the JSON is valid.

import {
  Convert as GencodeConvert,
  DebugTrace,
  Function as FunctionInfo,
  Slot as SlotInfo
} from './generate/debug-trace-quicktype';

export { DebugTrace, FunctionInfo, SlotInfo };

export class Convert extends GencodeConvert {
  public static toDebugTrace(json: string): DebugTrace {
    // Use the quicktype library to parse and check the validity of the passed-in JSON.
    const out = GencodeConvert.toDebugTrace(json);

    // Confirm the version of the JSON trace data.
    const expectedVersion1: string = '20220119b';
    const expectedVersion2: string = '20220209';
    if (out.version != expectedVersion1 && out.version != expectedVersion2) {
      throw Error(
        `Version mismatch. Trace version is '${out.version}', expected version ` +
        `is '${expectedVersion1}' or '${expectedVersion2}'`);
    }

    // The trace data consists of three values--one trace-op and two data fields.
    // https://github.com/google/skia/blob/2ac310266912687a2266d45f5008b942d56fc35e/src/sksl/tracing/SkVMDebugTrace.h#L52-L53
    // Our JSON trace arrays omit trailing zeros from the data to save space. Re-insert them here.
    for (let index = 0; index < out.trace.length; ++index) {
      while (out.trace[index].length < 3) {
        out.trace[index].push(0);
      }
    }

    // If the groupIdx field is missing from a slot, have it mirror `index` (the component index).
    for (let index = 0; index < out.slots.length; ++index) {
      if ((out.slots[index].groupIdx ?? -1) < 0) {
        out.slots[index].groupIdx = out.slots[index].index;
      }
    }

    return out;
  }
}
