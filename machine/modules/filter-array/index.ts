/**
 * FilterArray is a class for filtering a live array of rich objects based on
 * a case-insensitive substring search.
 *
 * The array being monitored needs to be passed to updateArray() every time it
 * changes.
 *
 * Each time the substring being filtered for changes, call filterChanged(),
 * and pass it in. This is typically done from an event listener on an input
 * field.
 *
 * Initially, the FilterArray will represent an empty list. If updateArray() is
 * called before filterChanged(), it will represent an unfiltered view of the
 * list. Then, once filterChanged() is called, filtration will begin.
 *
 * The matchingValues() function is expected to be used in a lit-html template
 * and returns all the matches for the filter.
 */
export class FilterArray<T> {
  private filter: string = '';

  private elements: T[] = [];

  /** Lowercase-folded JSON representations of my elements, good for searching */
  private jsonifiedElements: string[] = [];

  /**
   * Call this every time the array being filtered changes.
   *
   * @param arr - The array to be filtered
   */
  updateArray(arr: T[]): void {
    this.elements = arr;
    this.jsonifiedElements = arr.map((e) => JSON.stringify(e).toLowerCase());
  }

  /**
   * Returns the elements of the array passed in via updateArray() that match
   * the current filter.
   *
   * If updateArray() hasn't yet been called, return an empty array. If no
   * filter has been set (via connect()), return unfiltered results.
   *
   * Note that this currently searches each JSONified array element as a single
   * string, so JSON delimiters like { and " do get matched, and matches can
   * span properties.
   */
  matchingValues(): T[] {
    const ret: T[] = [];
    this.jsonifiedElements.forEach((v, i) => {
      if (v.includes(this.filter)) {
        ret.push(this.elements[i]);
      }
    });
    return ret;
  }

  /** Inform me that the string I'm filtering for has changed. */
  filterChanged(value: string): void {
    this.filter = value.toLowerCase();
  }
}
