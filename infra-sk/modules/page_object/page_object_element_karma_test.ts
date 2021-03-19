import { PageObjectElement } from './page_object_element';
import { TestBed, describePageObjectElement } from './page_object_element_test_cases';
import { Serializable } from 'puppeteer';

describe('PageObjectElement on the browser', () => {
  // This div will contain the top-level element in the HTML provided via the test bed.
  let container: HTMLDivElement;

  before(() => {
    // The container's innerHTML will be overwritten at the beginning of each test case.
    container = document.createElement('div');
    document.body.appendChild(container);
  });

  after(() => { container.remove(); });

  const testBed: TestBed = {
    setUpPageObjectElement: (html: string): Promise<PageObjectElement> => {
      container.innerHTML = html;

      // Make sure there is only one top-level element.
      if (container.childElementCount !== 1) {
        throw new Error('the given HTML contains more than one top-level element');
      }

      // Retrieve the top-level element and wrap it inside a PageObjectElement.
      const element = container.firstElementChild as HTMLElement;
      return Promise.resolve(new PageObjectElement(element));
    },

    evaluate: <T extends Serializable | void = void>(fn: (el: HTMLElement) => T) => {
      return Promise.resolve(fn(container.firstElementChild as HTMLElement));
    }
  };

  describePageObjectElement(testBed);
});
