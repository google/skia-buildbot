/**
 * @module modules/sort
 * @description
 *
 * Provides SortHistory, which provides functionality for sorting a a generic
 * type <T> on various attributes, and reflecting the state of the sort to/from
 * a serialized string, which is useful for passing to/from stateReflector.
 *
 */

/** The direction a column is sorted in. */
export type direction = 1 | -1;

export const up: direction = 1;

export const down: direction = -1;

/** Represents a how a single column in the table is to be sorted.
 */
export class SortSelection {
  // The column to sort on, the value is an index into a columnSortFunctions
  // array.
  column: number = 0;

  dir: direction = up;

  constructor(column: number, dir: direction) {
    this.column = column;
    this.dir = dir;
  }

  toggleDirection(): void {
    if (this.dir === down) {
      this.dir = up;
    } else {
      this.dir = down;
    }
  }

  /** Returns 1 if sorting in the up direction, and -1 if sorting in the down
   * direction. */
  directionMultiplier(): number {
    return this.dir;
  }

  /** Encodes the SortSelection as a string. */
  encode(): string {
    const encodedDir = this.dir === up ? 'u' : 'd';
    return `${encodedDir}${this.column}`;
  }

  /** Decode an encoded SortSelection from a string encoded by
   * SortSelection.encode(). */
  static decode(s: string): SortSelection {
    const dir = s[0] === 'u' ? up : down;
    const column = +s.slice(1);
    return new SortSelection(column, dir);
  }
}

/** Type for a function that can be passed to Array.sort().
 *
 *  It should always sort a column in an ascending direction.
 */
export type compareFunc<T> = (a: T, b: T)=> number;

/** An array of the sort functions for all the columns. Note that the index of a
  *  sort function does not need to correspond to location of a column on the
  *  display. Every value for the full length of columnSortFunctions should be
  *  populated, even if populated with a noop function, e.g. a function that
  *  return 0 for all inputs.
  */
export type columnSortFunctions<T> = compareFunc<T>[];

/**
 * Keeps one SortSelection for each column being displayed. As the user clicks
 * on columns the function `selectColumnToSortOn` can be called to keep
 * `this.history` up to date.
 *
 * This enables better sorting behavior, i.e. when you click on col A to sort,
 * then on col B to sort, if there are ties in col B they are broken by the
 * existing order in col A, just like you would get when sorting by columns in a
 * spreadsheet.
 *
 * This is not technically 'stable sort', while each sort action by the user
 * looks like it is doing a stable sort, which is the goal, we are really doing
 * an absolute sort based on a memory of all previous sort actions.
 */
export class SortHistory<T> {
  /** Columns will be sorted by the first entry in history. If that yields a
   * tie, then the second entry in history will be used to break the tie, etc.
   */
  history: SortSelection[] = []

  sortFunctions: columnSortFunctions<T> = []

  constructor(sortFunctions: columnSortFunctions<T>) {
    this.sortFunctions = sortFunctions;
    this.history = this.sortFunctions.map((_, column) => new SortSelection(column, up));
  }

  /** Moves the selected column to the front of the list for sorting, and also
   * reverses its current direction.
   */
  selectColumnToSortOn(column: number): void {
    // Remove the matching SortSelection from history.
    let removed: SortSelection[] = [];
    for (let i = 0; i < this.history.length; i++) {
      if (column === this.history[i].column) {
        removed = this.history.splice(i, 1);
        break;
      }
    }

    // Toggle its direction.
    removed[0].toggleDirection();

    // Then add back to the beginning of the list.
    this.history.unshift(removed[0]);
  }

  /** compare is a compareFunc that sorts based on the state of all the
   *  SortSelections in history.
   */
  compare(a: T, b: T): number {
    let ret = 0;

    // Call each compareFunc in `history` until one of them produces a non-zero
    // result. If all calls return 0 then this compare function also returns 0.
    this.history.some((sel: SortSelection) => {
      ret = sel.directionMultiplier() * this.sortFunctions[sel.column](a, b);
      return ret;
    });
    return ret;
  }

  /** Encodes the SortHistory as a string.
   *
   * The format is of all the serialized history members joined by
   * dashes.
   */
  encode(): string {
    return this.history.map((sel: SortSelection) => sel.encode()).join('-');
  }

  /** Decodes a string previously encoded via this.encode() and uses it to set
   * the history state. */
  decode(s: string): void {
    if (s === '') {
      return;
    }
    const oldHistory = [...this.history];
    this.history = s.split('-').map((encodedSortSelection: string) => SortSelection.decode(encodedSortSelection));

    // Now add in all the members of oldHistory that don't appear in this.history.
    oldHistory.forEach((oldSelection: SortSelection) => {
      if (!this.history.some((sel: SortSelection) => sel.column === oldSelection.column)) {
        this.history.push(oldSelection);
      }
    });

    const isValid = this.history.every((ss: SortSelection): boolean => (ss.column >= 0) && (ss.column < this.sortFunctions.length));
    if (!isValid) {
      this.history = oldHistory;
    }
  }
}
