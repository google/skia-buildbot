/** @module infra-sk/test_util
 * @description
 *
 * <p>
 *  A general set of useful functions for tests and demos,
 *  e.g. reducing boilerplate.
 * </p>
 */

import { expect } from 'chai';
import { deepCopy } from './object';
import { LitElement } from 'lit';
import { ElementSk } from './ElementSk';

/**
 * Takes a DOM element name (e.g. 'my-component-sk') and returns a factory
 * function that can be used to obtain new instances of that element.
 *
 * The element returned by the factory function is attached to document.body,
 * and an afterEach() hook is set to automatically remove the element from the
 * DOM after each test.
 *
 * The returned factory function optionally takes a callback function that will
 * be called with the newly instantiated element before it is attached to the
 * DOM, giving client code a chance to finish setting up the element before e.g.
 * the element's connectedCallback() method is invoked.
 *
 * Sample usage:
 *
 *   describe('my-component-sk', () => {
 *     const newInstance = setUpElementUnderTest('my-component-sk');
 *
 *     it('should be correctly instantiated', () => {
 *       const myComponentSk = newInstance((el) => {
 *         // This is called before attaching the element to the DOM.
 *         el.setAttribute('hello', 'world');
 *       });
 *
 *       expect(myComponentSk.parentElement).to.equal(document.body);
 *       expect(myComponentSk.getAttribute('hello')).to.equal('world');
 *     });
 *   });
 *
 * @param elementName Name of the element to test, e.g. 'foo-sk'.
 * @return A factory function that optionally takes a callback which is invoked
 *     with the newly instantiated element before it is attached to the DOM.
 */
export function setUpElementUnderTest<T extends HTMLElement>(
  elementName: string
): (finishSetupCallback?: (instance: T) => void) => T {
  let element: T | null;

  afterEach(() => {
    if (element) {
      document.body.removeChild(element);
      element = null;
    }
  });

  return (finishSetupCallbackFn?: (instance: T) => void) => {
    element = document.createElement(elementName) as T;
    if (finishSetupCallbackFn) {
      finishSetupCallbackFn(element);
    }
    document.body.appendChild(element);
    return element;
  };
}

/**
 * Returns a promise that will resolve when the given DOM event is caught at
 * document, or reject if the event isn't caught within the given
 * amount of time.
 *
 * Sample usage:
 *
 *   // Code under test.
 *   function doSomethingThatTriggersCustomEvent() {
 *     ...
 *     this.dispatchEvent(
 *         new CustomEvent('my-event', {detail: 'hello world', bubbles: true});
 *   }
 *
 *   // Test.
 *   it('should trigger a custom event', async () => {
 *     const myEvent = eventPromise('my-event');
 *     doSomethingThatTriggersCustomEvent();
 *     const ev = await myEvent;
 *     expect(ev.detail).to.equal('hello world');
 *   });
 *
 * Set timeoutMillis = 0 to skip call to setTimeout(). This is necessary on
 * tests that simulate the passing of time with sinon.useFakeTimers(), which
 * could trigger the timeout before the promise has a chance to catch the event.
 *
 * @param event Name of event to catch.
 * @param timeoutMillis How long to wait for the event before rejecting the
 *     returned promise.
 * @return A promise that will resolve to the caught event.
 */
export function eventPromise<T extends Event>(event: string, timeoutMillis = 5000) {
  const eventCaughtCallback = (resolve: (event: T) => void, _: any, e: T) => resolve(e);
  const timeoutCallback = (_: any, reject: (reason: any) => void) =>
    reject(
      new Error(`timed out after ${timeoutMillis} ms while waiting to catch event "${event}"`)
    );
  return buildEventPromise<T>(event, timeoutMillis, eventCaughtCallback, timeoutCallback);
}

/**
 * Returns a promise that will resolve if the given DOM event is *not* caught at
 * document after the given amount of time, or reject if the
 * event is caught.
 *
 * Useful for testing code that emits an event based on some condition.
 *
 * Sample usage:
 *
 *   // Code under test.
 *   function maybeTriggerCustomEvent(condition) {
 *     if (condition) {
 *       this.dispatchEvent(
 *         new CustomEvent('my-event', {detail: 'hello world', bubbles: true});
 *     } else {
 *       // Do nothing.
 *     }
 *   }
 *
 *   // Test.
 *   it('should not trigger a custom event', async () => {
 *     const noEvent = noEventPromise('my-event');
 *     maybeTriggerCustomEvent(false);
 *     await noEvent;
 *   });
 *
 * Set timeoutMillis = 0 to skip call to setTimeout(). This is necessary on
 * tests that simulate the passing of time with sinon.useFakeTimers(), which
 * could trigger the timeout before the promise has a chance to catch the event.
 *
 * @param event Name of event to catch.
 * @param timeoutMillis  How long to wait for the event before rejecting the
 *     returned promise.
 * @return A promise that will resolve to the caught event.
 */
export function noEventPromise(event: string, timeoutMillis = 200) {
  const eventCaughtCallback = (_: any, reject: (reason: any) => void) =>
    reject(new Error(`event "${event}" was caught when none was expected`));
  const timeoutCallback = (resolve: () => void) => resolve();
  return buildEventPromise<void>(event, timeoutMillis, eventCaughtCallback, timeoutCallback);
}

/**
 * Helper function to construct promises based on DOM events.
 *
 * @param event Name of event to add a listener for at document.
 * @param timeoutMillis Milliseconds to wait before timing out.
 * @param eventCaughtCallback Called when the event is caught with parameters
 *     (resolve, reject, event), where resolve and reject are the functions
 *     passed to the promise's executor function, and event is the Event object
 *     that was caught.
 * @param timeoutCallback Called after timeoutMillis if no event is caught,
 *     with arguments (resolve, reject) as passed to eventCaughtCallback.
 * @return A promise that will resolve or reject based exclusively on what the
 *     callback functions do with the resolve and reject parameters.
 */
function buildEventPromise<T extends Event | void>(
  event: string,
  timeoutMillis: number,
  eventCaughtCallback: (
    resolve: (value: T | PromiseLike<T>) => void,
    reject: (reason?: any) => void,
    event: T
  ) => void,
  timeoutCallback: (
    resolve: (value: T | PromiseLike<T>) => void,
    reject: (reason?: any) => void
  ) => void
) {
  // The executor function passed as a constructor argument to the Promise
  // object is executed immediately. This guarantees that the event handler
  // is added to document before returning.
  return new Promise<T>((resolve, reject) => {
    let timeout: number;

    const handler = (e: Event) => {
      document.removeEventListener(event, handler);
      window.clearTimeout(timeout);
      eventCaughtCallback(resolve, reject, e as T);
    };

    // Skip setTimeout() call with timeoutMillis = 0. Useful when faking time in
    // tests with sinon.useFakeTimers(). See
    // https://sinonjs.org/releases/v7.5.0/fake-timers/.
    if (timeoutMillis !== 0) {
      timeout = window.setTimeout(() => {
        document.removeEventListener(event, handler);
        timeoutCallback(resolve, reject);
      }, timeoutMillis);
    }
    document.addEventListener(event, handler);
  });
}

/**
 * Returns a promise that will resolve when the given sequence of DOM events is caught, or reject
 * after timeoutMillis. The returned promise will resolve to an array with the caught events.
 *
 * This is a generalization of eventPromise() which can be used to catch multiple events of the same
 * kind. Any out-of-sequence events will be ignored.
 *
 * @example
 *
 *   // Code under test.
 *   function doSomething() {
 *     this.dispatchEvent(new CustomEvent('hey',     {detail: 'a', bubbles: true});
 *     this.dispatchEvent(new CustomEvent('there',   {detail: 'b', bubbles: true});  // Unknown.
 *     this.dispatchEvent(new CustomEvent('hey',     {detail: 'c', bubbles: true});
 *     this.dispatchEvent(new CustomEvent('hey',     {detail: 'd', bubbles: true});  // Ignored.
 *     this.dispatchEvent(new CustomEvent('hello',   {detail: 'e', bubbles: true});
 *     this.dispatchEvent(new CustomEvent('world',   {detail: 'f', bubbles: true});
 *     this.dispatchEvent(new CustomEvent('hey',     {detail: 'g', bubbles: true});  // Ignored.
 *     this.dispatchEvent(new CustomEvent('world',   {detail: 'h', bubbles: true});  // Ignored.
 *     this.dispatchEvent(new CustomEvent('ok',      {detail: 'i', bubbles: true});  // Unknown.
 *     this.dispatchEvent(new CustomEvent('goodbye', {detail: 'j', bubbles: true});
 *   }
 *
 *   // Test.
 *   it('should trigger events', async () => {
 *     const sequence =
 *       eventSequencePromise<CustomEvent<string>>(['hey', 'hey', 'hello', 'world', 'goodbye']);
 *     doSomething();
 *     const events = await sequence;
 *     expect(events).to.have.length(5)
 *     expect(events[0].detail).to.equal('a');
 *     expect(events[1].detail).to.equal('c');
 *     expect(events[2].detail).to.equal('e');
 *     expect(events[3].detail).to.equal('f');
 *     expect(events[4].detail).to.equal('j');
 *   });
 */
export async function eventSequencePromise<T extends Event>(events: string[], timeoutMillis = 200) {
  if (events.length === 0) {
    return [];
  }

  return new Promise<T[]>((resolve, reject) => {
    const eventsToGo = deepCopy(events); // We'll remove events from this list as we catch them.
    const caughtEvents: T[] = []; // Will store any caught in-sequence events.

    // We'll keep a reference to each event listener we add so we can remove them later.
    const eventHandlers = new Map<string, (event: T) => void>();

    let timeout: number | null = null;

    // Called after resolving or rejecting to remove any event listeners and clear the timeout.
    const cleanUp = () => {
      if (timeout) {
        window.clearTimeout(timeout);
      }

      eventHandlers.forEach((handler, eventName) => {
        document.removeEventListener(eventName, handler as (event: Event) => void);
      });
    };

    // Set up the event handlers.
    for (const eventName of events) {
      // Adding a handler for each event once allows us to catch sequences with multiple instances
      // of the same event.
      if (!eventHandlers.has(eventName)) {
        const handler = (event: T) => {
          // Skip if the caught event is out of sequence.
          if (eventsToGo[0] !== eventName) {
            return;
          }

          eventsToGo.splice(0, 1); // Remove the 0th event name from the sequence.
          caughtEvents.push(event);

          // Resolve promise if we're done.
          if (eventsToGo.length === 0) {
            cleanUp();
            resolve(caughtEvents);
          }
        };

        eventHandlers.set(eventName, handler);
        document.addEventListener(eventName, handler as (event: Event) => void);
      }
    }

    // Set up the timeout.
    if (timeoutMillis !== 0) {
      timeout = window.setTimeout(() => {
        cleanUp();
        reject(
          `timed out after ${timeoutMillis} ms while waiting to catch events ` +
            `"${eventsToGo.join('", "')}"`
        );
      }, timeoutMillis);
    }
  });
}

/**
 * Asserts that there the given string exactly matches the current query string
 * of the url bar. For example, expectQueryStringToEqual('?foo=bar&foo=orange');
 */
export function expectQueryStringToEqual(expected: string) {
  expect(window.location.search).to.equal(expected);
}

/**
 * Sets the query string to be the provided value. Does *not* cause a page reload.
 */
export function setQueryString(q: string) {
  window.history.pushState(null, '', window.location.origin + window.location.pathname + q);
}

/**
 * Waits for the next render cycle of a LitElement or ElementSk component, including browser paint.
 * Useful for ensuring DOM updates have completed in tests.
 *
 * @param el The element to wait for.
 */
export async function waitForRender(el: ElementSk | LitElement) {
  const domUpdate = () => new Promise((resolve) => setTimeout(resolve, 0));
  if (!el) return;
  await domUpdate();
  if (el instanceof LitElement && el.updateComplete) {
    await el.updateComplete;
  }
  // LitElement updates are done, now wait for browser to paint
  await new Promise((resolve) => requestAnimationFrame(resolve));
  // Final yield
  await domUpdate();
}
