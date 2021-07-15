/**
 * FilterArray is a class for filtering an array of objects based on matching a
 * filter string.
 *
 * Pass it an input element that holds the text filter.
 *
 * The passed in newFilterValueCallback will be called every time the filter
 * value has changed.
 *
 * The array being monitored need to be passed to updateArray() every time it
 * changes.
 *
 * The matchingIndices() function is expected to be used in a lit-html template
 * and returns all the matches for the filter as an array of indices.
 */
export class FilterArray {
  private inputElement: HTMLInputElement;

  private newFilterValueCallback?: ()=> void;

  private filter: string;

  private arrayAsStrings: string[] = [];

  /**
   * FilterArray is a class for filtering an array of objects based on
   * matching a  filter string.
   *
   * @param inputElement The text input that contains the text to filter the
   * array with.
   * @param newFilterValueCallback - Callback that is triggered on every
   * inputElement input event.
   */
  constructor(
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
  updateArray<T>(arr: T[]): void {
    this.arrayAsStrings = arr.map((e) => JSON.stringify(e).toLowerCase());
  }

  /**
   * Returns an array of indices into the array passed in via updateArray() that
   * match the current filter.
   *
   * Note that this currently searches each JSONified array element as a single
   * string, so JSON delimiters like { and " do get matched, and matches can
   * span fields.
   */
  matchingIndices(): number[] {
    const ret: number[] = [];
    this.arrayAsStrings.forEach((s, i) => {
      if (s.includes(this.filter)) {
        ret.push(i);
      }
    });
    return ret;
  }

  private filterChanged(): void {
    this.filter = this.inputElement.value.toLowerCase();
    this.newFilterValueCallback?.();
  }
}
