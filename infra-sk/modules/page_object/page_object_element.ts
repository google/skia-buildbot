import { ElementHandle, Serializable } from 'puppeteer';
import { asyncFilter, asyncFind, asyncForEach, asyncMap } from '../async';

// Custom type guard to tell DOM elements and Puppeteer element handles apart.
function isPptrElement(
    element: HTMLElement | ElementHandle<HTMLElement>): element is ElementHandle<HTMLElement> {
  return (element as ElementHandle).asElement !== undefined;
}

/**
 * A helper class to write page objects[1] that work both on in-browser and Puppeteer tests.
 *
 * It's essentially a wrapper class that contains either a DOM node (HTMLElement) or a Puppeteer
 * handle (ElementHandle). Its API is analogous to that of HTMLElement, with the exception that
 * most functions return promises due to Puppeteer's asynchronous nature.
 *
 * A number of select* async methods are included to facilitate common tasks involving query
 * selectors and reduce the number of await statements in client code.
 *
 * To ensure compatibility with both in-browser and Puppeteer tests, page objects must be built
 * exclusively using PageObjectElement without ever referencing DOM nodes or Puppeteer element
 * handles directly.
 *
 * PageObjectElement is inspired by PageLoader[2], a Dart framework for creating page objects
 * compatible with both in-browser and WebDriver tests.
 *
 * [1] https://martinfowler.com/bliki/PageObject.html
 * [2] https://github.com/google/pageloader
 */
export class PageObjectElement {
  private readonly elementPromise: Promise<HTMLElement | ElementHandle<HTMLElement> | null>;

  constructor(
      element:
          HTMLElement |
          ElementHandle<HTMLElement> |
          Promise<HTMLElement | ElementHandle<HTMLElement> | null>) {
    if (element instanceof Promise) {
      this.elementPromise = element;
    } else {
      this.elementPromise = new Promise((resolve) => resolve(element));
    }
  }

  /** Returns true if the underlying DOM node or Puppeteer handle is empty. */
  async isEmpty(): Promise<boolean> {
    return !(await this.elementPromise);
  }

  /////////////////////////////////////////////////////////////////
  // Wrappers around various HTMLElement methods and properties. //
  /////////////////////////////////////////////////////////////////

  // Please add any missing wrappers as needed.

  /** Analogous to HTMLElement#innerText. */
  get innerText(): Promise<string> {
    return this.applyFnToDOMNode((el) => el.innerText);
  }

  /** Returns true if the element's inner text equals the given string. */
  async isInnerTextEqualTo(text: string): Promise<boolean> {
    return (await this.innerText) === text;
  }

  /** Analogous to HTMLElement#className. */
  get className(): Promise<string> {
    return this.applyFnToDOMNode((el) => el.className);
  }

  /** Returns true if the element has the given CSS class. */
  async hasClassName(className: string) {
    return this.applyFnToDOMNode(
        (el, className) => el.classList.contains(className as string),
        className);
  }

  /** Analogous to HTMLElement#focus(). */
  async focus() {
    const element = await this.elementPromise;
    await element!.focus();
  }

  /** Analogous to HTMLElement#click(). */
  async click() {
    const element = await this.elementPromise;
    await element!.click();
  }

  /** Analogous to HTMLElement#hasAttribute(). */
  async hasAttribute(attribute: string): Promise<boolean> {
    return this.applyFnToDOMNode(
      (el, attribute) => el.hasAttribute(attribute as string), attribute);
  }

  /** Analogous to HTMLElement#getAttribute(). */
  async getAttribute(attribute: string): Promise<string | null> {
    return this.applyFnToDOMNode(
      (el, attribute) => el.getAttribute(attribute as string), attribute);
  }

  /** Analogous to the HTMLElement#value property getter (e.g. for text inputs, selects, etc.). */
  get value(): Promise<string> {
    return this.applyFnToDOMNode((el) => (el as HTMLInputElement).value);
  }

  /**
   * Sends a single key press.
   *
   * Sends actual key presses on Puppeteer. Simulates events "keydown", "keypress" and "keyup" on
   * the browser.
   *
   * @param key The "key" attribute of the KeyboardEvent to be dispatched.
   */
  async typeKey(key: string) {
    const element = await this.elementPromise;
    if (isPptrElement(element!)) {
      return element.type(key);
    }
    element!.dispatchEvent(new KeyboardEvent('keydown', {bubbles: true, key: key}));
    element!.dispatchEvent(new KeyboardEvent('keypress', {bubbles: true, key: key}));
    element!.dispatchEvent(new KeyboardEvent('keyup', {bubbles: true, key: key}));
  }

  /**
   * Analogous to the HTMLElement#value property setter (e.g. for text inputs, selects, etc.).
   *
   * Simulates events "input" and "change".
   *
   * Note: Only one "input" event is dispatched. The browser normally dispatches one "input" event
   * per keystroke.
   */
  async enterValue(value: string) {
    // A future version of this method might take advantage of Puppeteer's ElementHandle.type()
    // method and/or simulate DOM events in a more realistic way as done in the PageLoader library:
    // https://github.com/google/pageloader/blob/80766100da9fe05d99eb92edd69b7ddfa82cc10e/lib/src/html/html_page_loader_element.dart#L393.
    await this.applyFnToDOMNode((el, value) => {
      // The below type union is non-exhaustive and for illustration purposes only.
      (el as HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement).value = value as string;

      // Simulate a subset of the input events (just one). This should be enough for most tests.
      el.dispatchEvent(new Event('input', {bubbles: true}));
      el.dispatchEvent(new Event('change', {bubbles: true}));
    }, value);
  }

  /**
   * Returns the result of evaluating the given function, passing the wrapped HTMLElement (i.e.
   * the DOM node) as the first argument, followed by any number of Serializable parameters.
   *
   * The function will be evaluated natively or via Puppeteer according to the type of the wrapped
   * element.
   */
  async applyFnToDOMNode<T extends Serializable | void>(
      fn: (element: HTMLElement, ...args: Serializable[]) => T,
      ...args: Serializable[]): Promise<T> {
    const element = await this.elementPromise;
    if (isPptrElement(element!)) {
      return await element.evaluate(fn, ...args) as T;
    }
    return fn(element!, ...args);
  }

  ////////////////////////////////////////////////////////////////////
  // Query selectors and convenience methods using query selectors. //
  ////////////////////////////////////////////////////////////////////

  /** Analogous to HTMLElement#querySelector(). */
  bySelector(selector: string): PageObjectElement {
    return new PageObjectElement(this.elementPromise.then((element) => {
      if (!element) {
        return new Promise((resolve) => resolve(null));
      }
      if (isPptrElement(element)) {
        // Note that common-sk functions $ and $$ are aliases for HTMLElement#querySelectorAll() and
        // HTMLElement#querySelector(), respectively, whereas Puppeteer's ElementHandle#$() and
        // ElementHandle#$$() methods are the other way around.
        return element.$(selector);
      }
      return new Promise((resolve) => resolve(element.querySelector<HTMLElement>(selector)));
    }));
  }

  /** Analogous to HTMLElement#querySelectorAll(). */
  bySelectorAll(selector: string): PageObjectElementList {
    return new PageObjectElementList(this.elementPromise.then((element) => {
      if (!element) {
        return [];
      }
      if (isPptrElement(element)) {
        // Note that common-sk functions $ and $$ are aliases for HTMLElement#querySelectorAll() and
        // HTMLElement#querySelector(), respectively, whereas Puppeteer's ElementHandle#$() and
        // ElementHandle#$$() methods are the other way around.
        return element.$$(selector);
      }
      return new Promise((resolve) =>
          resolve(Array.from(element.querySelectorAll<HTMLElement>(selector))));
    }));
  }
}

/** Convenience wrapper around a promise that returns a list. */
export abstract class AsyncList<T> {
  private readonly itemsPromise: Promise<T[]>;

  protected constructor(items?: Promise<T[]>) {
    if (!items) {
      items = new Promise<T[]>((resolve) => resolve([]));
    }
    this.itemsPromise = items;
  }

  /** Returns the item with the given index from the list. */
  async item(index: number): Promise<T> {
    return (await this.itemsPromise)[index];
  }

  /** Analogous to Array.prototype.length. */
  get length(): Promise<number> {
    return this.itemsPromise.then((items) => items.length);
  }

  /** Analogous to Array.prototype.filter, where the callback function returns a promise. */
  filter(fn: (item: T, index: number) => Promise<boolean>): Promise<T[]> {
    return asyncFilter(this.itemsPromise, fn);
  }

  /** Analogous to Array.prototype.find, where the callback function returns a promise. */
  find(fn: (item: T, index: number) => Promise<boolean>): Promise<T | null> {
    return asyncFind(this.itemsPromise, fn);
  }

  /** Analogous to Array.prototype.forEach, where the callback function returns a promise. */
  forEach(fn: (item: T, index: number) => Promise<void>): Promise<void> {
    return asyncForEach(this.itemsPromise, fn);
  }

  /** Analogous to Array.prototype.map, where the callback function returns a promise. */
  map<U>(fn: (item: T, index: number) => Promise<U>): Promise<U[]> {
    return asyncMap(this.itemsPromise, fn);
  }
}

/** Convenience wrapper around a Promise<PageObjectElement[]>. */
export class PageObjectElementList extends AsyncList<PageObjectElement> {
  constructor(itemsPromise?: Promise<HTMLElement[] | ElementHandle<HTMLElement>[]>) {
    super(itemsPromise?.then((items) =>
        items.map((item: HTMLElement | ElementHandle<HTMLElement>) =>
            new PageObjectElement(item))));
  }
}
