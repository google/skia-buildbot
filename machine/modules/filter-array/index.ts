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

  /** Lowercase-folded JSON representations of my elements, good for searching */
  private jsonifiedElements: string[] = [];

  /**
   * Initially, the element will represent an empty list. If updateArray() is
   * called before the element is connect()ed, it will represent an unfiltered
   * view of the list. Then, once connect()ed to a filtration UI, filtration
   * will begin.
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

  private filterChanged(): void {
    this.filter = this.inputElement!.value.toLowerCase();
    this.newFilterValueCallback?.();
  }
}
