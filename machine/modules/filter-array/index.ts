/** callback is the type of callback function that FilterArray accepts. */
interface callback {
    (): void;
}

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

    private newFilterValueCallback: callback | null;

    private filter: string;

    private arrayAsStrings: string[] = [];

    constructor(inputElement: HTMLInputElement, newFilterValueCallback: callback | null = null) {
      this.inputElement = inputElement;
      this.newFilterValueCallback = newFilterValueCallback;
      this.inputElement.addEventListener('input', () => this.filterChanged());
      this.filter = this.inputElement.value.toLowerCase();
    }

    updateArray<T>(arr: T[]): void {
      this.arrayAsStrings = arr.map((e) => JSON.stringify(e).toLowerCase());
    }

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
      if (this.newFilterValueCallback) {
        this.newFilterValueCallback();
      }
    }
}
