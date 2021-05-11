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
  /**
   * A property decorator that returns the first child element that matches the selector.
   *
   * @example
   *     class MyElementPO extends PageObject {
   *       @BySelector('button.submit')
   *       submitBtn!: Promise<PageObjectElement>;
   *
   *       async clickSubmitBtn() {
   *         await (await this.submitBtn).click();
   *       }
   *
   *       ...
   *     }
   */
  public static BySelector(selector: string): any {
    const decorator: PropertyDecorator = (target: Object, propertyKey: string | symbol) => {
      const propertyDescriptor: PropertyDescriptor = {
        get(): any {
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
   *       rows!: Promise<PageObjectElement[]>;
   *
   *       async getNumRows() {
   *         return (await this.rows).length;
   *       }
   *
   *       ...
   *     }
   */
  public static BySelectorAll(selector: string): any {
    const decorator: PropertyDecorator = (target: Object, propertyKey: string | symbol) => {
      const propertyDescriptor: PropertyDescriptor = {
        get(): any {
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
   *       anotherElementPO!: Promise<AnotherElementPO>;
   *
   *       async doSomething() {
   *         await (await this.anotherElementPO).clickSomeBtn();
   *       }
   *
   *       ...
   *     }
   */
  public static POBySelector<T extends PageObject>(
      selector: string, ctor: { new(...args: any[]): T }): any {
    const decorator: PropertyDecorator = (target: Object, propertyKey: string | symbol) => {
      const propertyDescriptor: PropertyDescriptor = {
        async get(): Promise<any> {
          const po = this as PageObject;
          return new ctor(await po.bySelector(selector));
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
   *       anotherElementPOs!: Promise<AnotherElementPO[]>;
   *
   *       async clickNthElement(n: number) {
   *         await (await this.anotherElementPOs)[n].clickSomeBtn();
   *       }
   *
   *       ...
   *     }
   */
  public static POBySelectorAll<T extends PageObject>(
      selector: string, ctor: { new(...args: any[]): T }): any {
    const decorator: PropertyDecorator = (target: Object, propertyKey: string | symbol) => {
      const propertyDescriptor: PropertyDescriptor = {
        get(): any {
          const po = this as PageObject;
          return po.bySelectorAll(selector).then((poes) => poes.map((poe) => new ctor(poe)));
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
  get empty() {
    return this.element.empty;
  }

  /**
   * Returns the result of calling PageObjectElement#bySelector() on the underlying
   * PageObjectElement.
   */
  protected bySelector(selector: string) {
    return this.element.bySelector(selector);
  }

  /**
   * Returns the result of calling PageObjectElement#bySelectorAll() on the underlying
   * PageObjectElement.
   */
  protected bySelectorAll(selector: string) {
    return this.element.bySelectorAll(selector);
  }
}

// Re-export decorators as module-level constants for brevity on the client code's side.

export const BySelector = PageObject.BySelector;
export const BySelectorAll = PageObject.BySelectorAll;
export const POBySelector = PageObject.POBySelector;
export const POBySelectorAll = PageObject.POBySelectorAll;
