/**
 * FilterArray is a class for filtering an array of things based on matching a
 * filter string.
 *
 * Once your page is drawn, call connect() to hook me up to an input element
 * (where the user types the filtration string) and a callback which draws the
 * filtered list.
 *
 * The array being monitored needs to be passed to updateArray() every time it
 * changes.
 *
 * The matchingValues() function is expected to be used in a lit-html template
 * and returns all the matches for the filter.
 */
export class FilterArray<T> {
  private inputElement: HTMLInputElement | null = null;

  private newFilterValueCallback?: ()=> void;

  private filter: string = '';

  private elements: T[] = [];

  /**
   * Initially, I will emit an empty array. If updateArray() is called before I
   * am connect()ed, I'll happily start returning an unfiltered view of the
   * array. Then, once connect()ed to a filtration UI, I will begin filtering.
   */
  constructor() {
  }

  /**
   * Hook me up to a page, once it's drawn.
   *
   * @param inputElement The text input that contains the text to filter the
   *   array with.
   * @param newFilterValueCallback - Callback that is triggered on every
   *   inputElement input event. Typically redraws the filtered list.
   */
  connect(
    inputElement: HTMLInputElement,
    newFilterValueCallback?: ()=> void,
  ) {
    this.inputElement = inputElement;
    this.newFilterValueCallback = newFilterValueCallback;
    this.inputElement.addEventListener('input', () => this.filterChanged());
    this.filter = this.inputElement.value.toLowerCase();
  }

  /**
   * Call this every time the array being filtered changes.
   *
   * @param arr - The array to be filtered.
   */
  updateArray(arr: T[]): void {
    this.elements = arr;
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
    // Optimization: could cache the stringification of array elements.
    return this.elements.filter(e => JSON.stringify(e).toLowerCase().includes(this.filter))
  }

  private filterChanged(): void {
    this.filter = this.inputElement!.value.toLowerCase();
    this.newFilterValueCallback?.();
  }
}
