import { ElementHandle } from 'puppeteer';
import {AsyncList, PageObjectElement, PageObjectElementList} from './page_object_element';

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
  /**
   * A property decorator that returns the first child element that matches the selector.
   *
   * @example
   *     class MyElementPO extends PageObject {
   *       @BySelector('button.submit')
   *       submitBtn!: PageObjectElement;
   *
   *       async clickSubmitBtn() {
   *         await this.submitBtn.click();
   *       }
   *
   *       ...
   *     }
   */
  public static BySelector(selector: string): PropertyDecorator {
    const decorator: PropertyDecorator = (target: Object, propertyKey: string | symbol) => {
      const propertyDescriptor: TypedPropertyDescriptor<PageObjectElement> = {
        get(): PageObjectElement {
          const po = this as PageObject;
          return po.bySelector(selector);
        },
      };
      Object.defineProperty(target, propertyKey, propertyDescriptor);
    }
    return decorator;
  }

  /**
   * A property decorator that returns all the child elements that match the selector.
   *
   * @example
   *     class MyElementPO extends PageObject {
   *       @BySelectorAll('table.foo tr')
   *       rows!: PageObjectElementList;
   *
   *       async getNumRows() {
   *         return await this.rows.length;
   *       }
   *
   *       ...
   *     }
   */
  public static BySelectorAll(selector: string): PropertyDecorator {
    const decorator: PropertyDecorator = (target: Object, propertyKey: string | symbol) => {
      const propertyDescriptor: TypedPropertyDescriptor<PageObjectElementList> = {
        get(): PageObjectElementList {
          const po = this as PageObject;
          return po.bySelectorAll(selector);
        },
      };
      Object.defineProperty(target, propertyKey, propertyDescriptor);
    }
    return decorator;
  }

  /**
   * A property decorator that instantiates a nested PageObject for the first child element that
   * matches the selector.
   *
   * @example
   *     import { AnotherElementPO } from 'path/to/another_po';
   *
   *     class MyElementPO extends PageObject {
   *       @POBySelector('another-element', AnotherElementPO)
   *       anotherElementPO!: AnotherElementPO;
   *
   *       async doSomething() {
   *         await this.anotherElementPO.clickSomeBtn();
   *       }
   *
   *       ...
   *     }
   */
  public static POBySelector<T extends PageObject>(
      selector: string, ctor: { new(...args: any[]): T }): PropertyDecorator {
    const decorator: PropertyDecorator = (target: Object, propertyKey: string | symbol) => {
      const propertyDescriptor: TypedPropertyDescriptor<T> = {
        get(): T {
          const po = this as PageObject;
          return new ctor(po.bySelector(selector));
        },
      };
      Object.defineProperty(target, propertyKey, propertyDescriptor);
    }
    return decorator;
  }

  /**
   * A property decorator that instantiates nested PageObjects for all the child elements that
   * match the selector.
   *
   * @example
   *     class MyElementPO extends PageObject {
   *       @POBySelectorAll('another-element')
   *       anotherElementPOs!: PageObjectList<AnotherElementPO>;
   *
   *       async clickAll() {
   *         await this.anotherElementPOs.forEach((po) => po.clickSomeBtn());
   *       }
   *
   *       ...
   *     }
   */
  public static POBySelectorAll<T extends PageObject>(
      selector: string, ctor: { new(...args: any[]): T }): PropertyDecorator {
    const decorator: PropertyDecorator = (target: Object, propertyKey: string | symbol) => {
      const propertyDescriptor: TypedPropertyDescriptor<PageObjectList<T>> = {
        get(): PageObjectList<T> {
          const po = this as PageObject;
          return new PageObjectList<T>(
              po.bySelectorAll(selector).map(async (poe) => new ctor(poe)));
        },
      };
      Object.defineProperty(target, propertyKey, propertyDescriptor);
    }
    return decorator;
  }

  protected element: PageObjectElement;

  constructor(element: HTMLElement | ElementHandle | PageObjectElement) {
    if (element instanceof PageObjectElement) {
      this.element = element;
    } else {
      this.element = new PageObjectElement(element);
    }
  }

  /** Returns true if the underlying PageObjectElement is empty. */
  async isEmpty(): Promise<boolean> {
    return await this.element.isEmpty();
  }

  /**
   * Returns the result of calling PageObjectElement#bySelector() on the underlying
   * PageObjectElement.
   */
  protected bySelector(selector: string): PageObjectElement {
    return this.element.bySelector(selector);
  }

  /**
   * Returns the result of calling PageObjectElement#bySelectorAll() on the underlying
   * PageObjectElement.
   */
  protected bySelectorAll(selector: string): PageObjectElementList  {
    return this.element.bySelectorAll(selector);
  }
}

/** Convenience wrapper around a promise of a list of page objects. */
export class PageObjectList<T extends PageObject> extends AsyncList<T> {
  constructor(itemsPromise: Promise<T[]>) {
    super(itemsPromise);
  }
}

// Re-export decorators as module-level constants for brevity on the client code's side.

export const BySelector = PageObject.BySelector;
export const BySelectorAll = PageObject.BySelectorAll;
export const POBySelector = PageObject.POBySelector;
export const POBySelectorAll = PageObject.POBySelectorAll;
