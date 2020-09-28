import { ElementHandle, Serializable } from 'puppeteer';

// Custon type guard to tell DOM elements and Puppeteer element handles apart.
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
  private element: HTMLElement | ElementHandle<HTMLElement>;

  constructor(element: HTMLElement | ElementHandle<HTMLElement>) {
    if (element === null) {
      throw new TypeError('element cannot be null');
    }
    if (element === undefined) {
      throw new TypeError('element cannot be undefined');
    }
    this.element = element;
  }

  /////////////////////////////////////////////////////////////////
  // Wrappers around various HTMLElement methods and properties. //
  /////////////////////////////////////////////////////////////////

  // Please add any missing wrappers as needed.

  /** Analogous to HTMLElement#innerText. */
  get innerText() {
    return this.applyFnToDOMNode((el) => el.innerText);
  }

  /** Analogous to HTMLElement#className. */
  get className() {
    return this.applyFnToDOMNode((el) => el.className);
  }

  /** Analogous to HTMLAnchorElement#href. */
  get href() {
    return this.applyFnToDOMNode((el) => (el as HTMLAnchorElement).href)
  }

  /** Analogous to HTMLElement#focus(). */
  async focus() {
    await this.element.focus();
  }

  /** Analogous to HTMLElement#click(). */
  async click() {
    await this.element.click();
  }

  /** Analogous to HTMLElement#hasAttribute(). */
  async hasAttribute(attribute: string) {
    return this.applyFnToDOMNode(
      (el, attribute) => el.hasAttribute(attribute as string), attribute);
  }

  /** Analogous to the HTMLElement#value property getter (e.g. for text inputs, selects, etc.). */
  get value() {
    return this.applyFnToDOMNode((el) => (el as HTMLInputElement).value);
  }

  /**
   * Sends a single key press.
   *
   * Sends actual key presses on Puppeteer. Simulates events "keydown", "keypress" y "keyup" on the
   * browser.
   *
   * @param key The "key" attribute of the KeyboardEvent to be dispatched.
   */
  async typeKey(key: string) {
    if (isPptrElement(this.element)) {
      return this.element.type(key);
    }
    this.element.dispatchEvent(new KeyboardEvent('keydown', {bubbles: true, key: key}));
    this.element.dispatchEvent(new KeyboardEvent('keypress', {bubbles: true, key: key}));
    this.element.dispatchEvent(new KeyboardEvent('keyup', {bubbles: true, key: key}));
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
      fn: (element: HTMLElement, ...args: Serializable[]) => T, ...args: Serializable[]) {
    if (isPptrElement(this.element)) {
      return await this.element.evaluate(fn, ...args) as T;
    }
    return fn(this.element, ...args);
  }

  ////////////////////////////////////////////////////////////////////
  // Query selectors and convenience methods using query selectors. //
  ////////////////////////////////////////////////////////////////////

  /** Analogous to HTMLElement#querySelector(). */
  async selectOnePOE(selector: string) {
    if (isPptrElement(this.element)) {
      // Note that common-sk functions $ and $$ are aliases for HTMLElement#querySelectorAll() and
      // HTMLElement#querySelector(), respectively, whereas Puppeteer's ElementHandle#$() and
      // ElementHandle#$$() methods are the other way around.
      const handle = await this.element.$(selector);
      return handle ? new PageObjectElement(handle) : null;
    }

    const element = this.element.querySelector<HTMLElement>(selector);
    return element ? new PageObjectElement(element) : null;
  }

  /** Analogous to HTMLElement#querySelectorAll(). */
  async selectAllPOE(selector: string) {
    if (isPptrElement(this.element)) {
      // Note that common-sk functions $ and $$ are aliases for HTMLElement#querySelectorAll() and
      // HTMLElement#querySelector(), respectively, whereas Puppeteer's ElementHandle#$() and
      // ElementHandle#$$() methods are the other way around.
      const handles = await this.element.$$(selector);
      return handles.map((handle) => new PageObjectElement(handle));
    }

    const elements = Array.from(this.element.querySelectorAll<HTMLElement>(selector));
    return elements.map((element) => new PageObjectElement(element));
  }

  /**
   * Applies the given function to the first descendant PageObjectElement matching the selector.
   *
   * Throws an error if the selector does not match any elements.
   *
   * Useful for any operation on child elements that can be done via the PageObjectElement's API
   * (as opposed to against the underlying HTMLElement, i.e. the raw DOM node), for example
   * clicking a child button:
   *
   * @example
   * await pageObjectElement.selectOnePOEThenApplyFn('button.submit', (btn) => btn.click());
   */
  async selectOnePOEThenApplyFn<T>(
      selector: string, fn: (element: PageObjectElement) => Promise<T>) {
    const element = await this.selectOnePOE(selector);
    if (!element) {
      throw new Error(`selector "${selector}" did not match any elements`);
    }
    return fn(element);
  }

  /**
   * Applies the given function to the first descendant HTMLElement matching the selector.
   *
   * Throws an error if the selector does not match any elements.
   *
   * Useful for operations on child elements that require direct access to a child element's
   * HTMLElement (i.e. the raw DOM node), for example invoking a method on a custom web component:
   *
   * @example
   * const retval =
   *   await poe.selectOneDOMNodeThenApplyFn('my-component', (c as MyComponent) => c.foo());
   */
  async selectOneDOMNodeThenApplyFn<T extends Serializable | void>(
      selector: string, fn: (element: HTMLElement) => T, ...args: Serializable[]) {
    const element = await this.selectOnePOE(selector);
    if (!element) {
      throw new Error(`selector "${selector}" did not match any elements`);
    }
    return element.applyFnToDOMNode(fn, ...args);
  }

  /**
   * Returns an array with the results of applying the given function to every descendant
   * PageObjectElement matching the selector.
   */
  async selectAllPOEThenMap<T>(
      selector: string, fn: (element: PageObjectElement, index: number) => Promise<T>) {
    const elements = await this.selectAllPOE(selector);
    return await Promise.all(elements.map(fn));
  }

  /** Applies the given function to every descendant PageObjectElement matching the selector. */
  async selectAllPOEThenForEach
      (selector: string, fn: (element: PageObjectElement, index: number) => Promise<void>) {
    await this.selectAllPOEThenMap(selector, fn);
  }

  /**
   * Returns the first descendant PageObjectElement matching the selector that satisfies the
   * provided testing function, or null if none satisfies it.
   */
  async selectAllPOEThenFind(
      selector: string, fn: (element: PageObjectElement, index: number) => Promise<boolean>) {
    let i = 0;
    for (const element of await this.selectAllPOE(selector)) {
      if (await fn(element, i++)) {
        return element;
      }
    }
    return null;
  }
}
