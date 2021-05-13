import { ElementHandle } from 'puppeteer';
import { PageObjectElement } from './page_object_element';
import { asyncMap } from '../async';

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

  /** Returns true if the underlying PageObjectElement is empty. */
  isEmpty(): boolean {
    return this.element.isEmpty();
  }

  /**
   * Returns the result of calling PageObjectElement#selectOnePOE() on the underlying
   * PageObjectElement.
   */
  protected selectOnePOE(selector: string): Promise<PageObjectElement> {
    return this.element.selectOnePOE(selector);
  }

  /**
   * Returns the result of calling PageObjectElement#selectAllPOE() on the underlying
   * PageObjectElement.
   */
  protected selectAllPOE(selector: string): Promise<PageObjectElement[]> {
    return this.element.selectAllPOE(selector);
  }

  /** Instantiates a PageObject with the first element that matches the given selector. */
  protected async poBySelector<T extends PageObject>(
      selector: string, ctor: { new(...args: any): T }): Promise<T> {
    return new ctor(await this.selectOnePOE(selector));
  }

  /** Instantiates one PageObject for each element that match the given selector. */
  protected async poBySelectorAll<T extends PageObject>(
      selector: string, ctor: { new(...args: any): T }): Promise<T[]> {
    return asyncMap(this.selectAllPOE(selector), async (poe: PageObjectElement) => new ctor(poe));
  }
}
