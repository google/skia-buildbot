import { ElementHandle, Serializable } from 'puppeteer';
import { PageObjectElement } from './page_object_element';

/**
 * A base class for writing page objects[1] that work both on in-browser and Puppeteer tests.
 *
 * A page object written as a subclass of PageObject will have two layers of wrapping:
 *
 *   1. The PageObject wraps a PageObjectElement.
 *   2. The PageObjectElement wraps either a DOM node (HTMLElement) or a Puppeteer handle
 *      (ElementHandle).
 *
 * The PageObjectElement wraps the root node of the component under test, and provides an
 * abstraction layer on top of the underlying DOM node or Puppeteer handle. A page object interacts
 * with the component under test exclusively via the PageObjectElement's API. This guarantees
 * compatibility with both in-browser and Puppeteer tests.
 *
 * PageObject and PageObjectElement are inspired by PageLoader[2], a Dart framework for creating
 * page objects compatible with both in-browser and WebDriver tests.
 *
 * For best practices, please see PageLoader Best Practices[3].
 *
 * [1] https://martinfowler.com/bliki/PageObject.html
 * [2] https://github.com/google/pageloader
 * [3] https://github.com/google/pageloader/blob/master/best_practices.md
 */
export abstract class PageObject {
  protected element: PageObjectElement;

  constructor(element: HTMLElement | ElementHandle | PageObjectElement) {
    if (element instanceof PageObjectElement) {
      this.element = element;
    } else {
      this.element = new PageObjectElement(element);
    }
  }

  /**
   * Returns the result of calling PageObjectElement#selectOnePOE() on the underlying
   * PageObjectElement.
   */
  protected selectOnePOE(selector: string): Promise<PageObjectElement | null> {
    return this.element.selectOnePOE(selector);
  }

  /**
   * Returns the result of calling PageObjectElement#selectOnePOEThenApplyFn() on the underlying
   * PageObjectElement.
   */
  protected selectOnePOEThenApplyFn<T>(
      selector: string, fn: (element: PageObjectElement) => Promise<T>): Promise<T> {
    return this.element.selectOnePOEThenApplyFn<T>(selector, fn);
  }

  /**
   * Returns the result of calling PageObjectElement#selectOneDOMNodeThenApplyFn() on the
   * underlying PageObjectElement.
   */
  protected selectOneDOMNodeThenApplyFn<T extends Serializable | void>(
      selector: string,
      fn: (element: HTMLElement) => T, ...args: Serializable[]): Promise<T> {
    return this.element.selectOneDOMNodeThenApplyFn<T>(selector, fn, ...args);
  }

  /**
   * Returns the result of calling PageObjectElement#selectAllPOE() on the underlying
   * PageObjectElement.
   */
  protected selectAllPOE(selector: string): Promise<PageObjectElement[]> {
    return this.element.selectAllPOE(selector);
  }

  /**
   * Returns the result of calling PageObjectElement#selectAllPOEThenMap() on the underlying
   * PageObjectElement.
   */
  protected selectAllPOEThenMap<T>(
      selector: string,
      fn: (element: PageObjectElement, index: number) => Promise<T>): Promise<T[]> {
    return this.element.selectAllPOEThenMap<T>(selector, fn);
  }

  /**
   * Returns the result of calling PageObjectElement#selectAllPOEThenForEach() on the underlying
   * PageObjectElement.
   */
  protected selectAllPOEThenForEach(
      selector: string, fn: (element: PageObjectElement, index: number) => Promise<void>) {
    return this.element.selectAllPOEThenForEach(selector, fn);
  }

  /**
   * Returns the result of calling PageObjectElement#$find() on the underlying
   * PageObjectElement.
   */
  protected selectAllPOEThenFind(
      selector: string,
      fn: (element: PageObjectElement, index: number) => Promise<boolean>
  ): Promise<PageObjectElement | null> {
    return this.element.selectAllPOEThenFind(selector, fn);
  }
}
