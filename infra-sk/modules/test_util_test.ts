import { expect } from 'chai';
import { $, $$ } from 'common-sk/modules/dom';
import sinon from 'sinon';
import {
  setUpElementUnderTest,
  eventPromise,
  noEventPromise,
  eventSequencePromise,
  expectQueryStringToEqual,
  setQueryString,
} from './test_util';

describe('test utilities', () => {
  describe('setUpElementUnderTest', () => {
    // We'll save references to the instances of the element under test created
    // by setUpElementUnderTest, and make assertions against them later on.
    let instance1: HTMLMarqueeElement;
    let instance2: HTMLMarqueeElement;

    // We run setUpElementUnderTest inside its own nested describe block to
    // limit the scope of the afterEach hook it sets up.
    describe('test suite with setUpElementUnderTest', () => {
      // We'll use <marquee> as the element under test.
      const newInstance = setUpElementUnderTest<HTMLMarqueeElement>('marquee');

      let element: HTMLMarqueeElement; // Instance of the element under test.
      beforeEach(() => {
        expect(
          $('marquee'),
          'no other <marquee> elements should be present in the DOM '
                + 'prior to instantiating the element under test',
        )
          .to.have.length(0);

        // Instantiate the element under test.
        element = newInstance((el) => el.innerHTML = '<p>hello world</p>');
      });

      afterEach(() => {
        expect(
          $('marquee'),
          'no instances of the element under test should be found in '
                + 'the DOM after each test case',
        )
          .to.have.length(0);
        expect(
          element.parentElement,
          'element under test should be detached from its parent node '
                + 'after each test case',
        )
          .to.be.null;
      });

      it('should correctly instantiate the element', () => {
        instance1 = element; // Save a reference to the current instance.
        expect(element.tagName).to.equal('MARQUEE');
        expect($$<HTMLParagraphElement>('p', element)!.innerText).to.equal('hello world');
      });

      it('should attach instance of element under test to document.body',
        () => {
          instance2 = element; // Save a reference to the current instance.
          expect($('marquee')).to.have.length(1);
          expect(element.parentElement).to.equal(document.body);
        });
    });

    // This describe block makes use of the fact that sibling describe blocks
    // are run in the order they are defined, as explained in
    // https://mochajs.org/#run-cycle-overview.
    describe('after the "setUpElementUnderTest" test suite runs', () => {
      // Assert that we've correctly captured the instances of the element under
      // test, which the test cases below rely on.
      it('should have correctly saved instances of the element under test',
        () => {
          expect(instance1.tagName).to.equal('MARQUEE');
          expect(instance2.tagName).to.equal('MARQUEE');
        });

      it('creates fresh instances before each test case', () => {
        expect(instance1).to.not.equal(instance2);
      });

      it('should detach instances from the DOM after each test case', () => {
        expect(instance1.parentElement).to.be.null;
        expect(instance2.parentElement).to.be.null;
      });

      it('no stray instances left on the test runner page after tests end',
        () => {
          expect($('marquee')).to.have.length(0);
        });
    });
  });

  describe('event promise functions', () => {
    let el: HTMLDivElement; // Element that we'll dispatch custom events from.
    let clock: sinon.SinonFakeTimers;

    beforeEach(() => {
      el = document.createElement('div');
      document.body.appendChild(el);
      clock = sinon.useFakeTimers();
    });

    afterEach(() => {
      document.body.removeChild(el);
      clock.restore();
    });

    describe('eventPromise', () => {
      it('resolves when event is caught', async () => {
        const hello = eventPromise<CustomEvent<string>>('hello');
        el.dispatchEvent(new CustomEvent('hello', { bubbles: true, detail: 'hi' }));
        const ev = await hello;
        expect(ev.detail).to.equal('hi');
      });

      it('catches document-level events', async () => {
        const hello = eventPromise<CustomEvent<string>>('hello');
        document.dispatchEvent(new CustomEvent('hello', { bubbles: true, detail: 'hi' }));
        const ev = await hello;
        expect(ev.detail).to.equal('hi');
      });

      it('one single event resolves multiple promises', async () => {
        const hello1 = eventPromise<CustomEvent<string>>('hello');
        const hello2 = eventPromise<CustomEvent<string>>('hello');

        // We'll emit two different events of the same type (see event detail).
        el.dispatchEvent(new CustomEvent('hello', { bubbles: true, detail: 'hi' }));
        el.dispatchEvent(new CustomEvent('hello', { bubbles: true, detail: 'goodbye' }));

        const ev1 = await hello1;
        const ev2 = await hello2;

        // The first event above should resolve both promises.
        expect(ev1.detail).to.equal('hi');
        expect(ev2.detail).to.equal('hi');
      });

      it('times out if event is not caught', async () => {
        const hello = eventPromise<CustomEvent<string>>('hello', 5000);
        el.dispatchEvent(new CustomEvent('bye', { bubbles: true }));
        clock.tick(10000);
        try {
          await hello;
          expect.fail('promise should not have resolved');
        } catch (error) {
          expect(error.message).to.equal(
            'timed out after 5000 ms while waiting to catch event "hello"',
          );
        }
      });

      it('never times out if timeoutMillis=0', async () => {
        const hello = eventPromise<CustomEvent<string>>('hello', 0);
        clock.tick(Number.MAX_SAFE_INTEGER);
        el.dispatchEvent(new CustomEvent('hello', { bubbles: true, detail: 'hi' }));
        const ev = await hello;
        expect(ev.detail).to.equal('hi');
      });
    });

    describe('noEventPromise', () => {
      it('resolves when event is NOT caught', async () => {
        const noHello = noEventPromise('hello', 200);
        el.dispatchEvent(new CustomEvent('bye', { bubbles: true }));
        clock.tick(10000);
        await noHello;
      });

      it('rejects if event is caught', async () => {
        const noHello = noEventPromise('hello', 200);
        el.dispatchEvent(new CustomEvent('hello', { bubbles: true }));
        try {
          await noHello;
          expect.fail('promise should not have resolved');
        } catch (error) {
          expect(error.message).to.equal(
            'event "hello" was caught when none was expected',
          );
        }
      });

      it('never resolves if timeoutMillis=0', async () => {
        const noHello = noEventPromise('hello', 0);
        clock.tick(Number.MAX_SAFE_INTEGER);
        el.dispatchEvent(new CustomEvent('hello', { bubbles: true }));
        try {
          await noHello;
          expect.fail('promise should not have resolved');
        } catch (error) {
          expect(error.message).to.equal(
            'event "hello" was caught when none was expected',
          );
        }
      });
    });

    describe('eventSequencePromise', () => {
      it('resolves immediately if the event sequence is empty', async () => {
        expect(await eventSequencePromise([])).to.have.length(0);
      });

      it('catches one event', async () => {
        const sequence = eventSequencePromise<CustomEvent<string>>(['hello']);
        el.dispatchEvent(new CustomEvent('hello', { bubbles: true, detail: 'hi' }));

        const events = await sequence;

        expect(events).to.have.length(1);
        expect(events[0].detail).to.equal('hi');
      });

      it('catches a sequence of events', async () => {
        const sequence = eventSequencePromise<CustomEvent<string>>(['hello', 'world', 'goodbye']);
        el.dispatchEvent(new CustomEvent('hello', { bubbles: true, detail: 'first' }));
        el.dispatchEvent(new CustomEvent('world', { bubbles: true, detail: 'second' }));
        el.dispatchEvent(new CustomEvent('goodbye', { bubbles: true, detail: 'third' }));

        const events = await sequence;

        expect(events).to.have.length(3);
        expect(events[0].detail).to.equal('first');
        expect(events[1].detail).to.equal('second');
        expect(events[2].detail).to.equal('third');
      });

      it('catches a sequence with repeated events', async () => {
        const sequence = eventSequencePromise<CustomEvent<string>>([
          'hey', 'hey', 'hello', 'world', 'hey', 'world', 'goodbye'
        ]);
        el.dispatchEvent(new CustomEvent('hey', { bubbles: true, detail: 'first' }));
        el.dispatchEvent(new CustomEvent('hey', { bubbles: true, detail: 'second' }));
        el.dispatchEvent(new CustomEvent('hello', { bubbles: true, detail: 'third' }));
        el.dispatchEvent(new CustomEvent('world', { bubbles: true, detail: 'fourth' }));
        el.dispatchEvent(new CustomEvent('hey', { bubbles: true, detail: 'fifth' }));
        el.dispatchEvent(new CustomEvent('world', { bubbles: true, detail: 'sixth' }));
        el.dispatchEvent(new CustomEvent('goodbye', { bubbles: true, detail: 'seventh' }));

        const events = await sequence;

        expect(events).to.have.length(7);
        expect(events[0].detail).to.equal('first');
        expect(events[1].detail).to.equal('second');
        expect(events[2].detail).to.equal('third');
        expect(events[3].detail).to.equal('fourth');
        expect(events[4].detail).to.equal('fifth');
        expect(events[5].detail).to.equal('sixth');
        expect(events[6].detail).to.equal('seventh');
      });

      it('ignores out-of-sequence events', async () => {
        const sequence = eventSequencePromise<CustomEvent<string>>(['hello', 'world', 'goodbye']);

        el.dispatchEvent(new CustomEvent('goodbye', { bubbles: true, detail: 'first' }));
        el.dispatchEvent(new CustomEvent('world', { bubbles: true, detail: 'second' }));
        el.dispatchEvent(new CustomEvent('hello', { bubbles: true, detail: 'third' }));
        el.dispatchEvent(new CustomEvent('hello', { bubbles: true, detail: 'fourth' }));
        el.dispatchEvent(new CustomEvent('goodbye', { bubbles: true, detail: 'fifth' }));
        el.dispatchEvent(new CustomEvent('world', { bubbles: true, detail: 'sixth' }));
        el.dispatchEvent(new CustomEvent('hello', { bubbles: true, detail: 'seventh' }));
        el.dispatchEvent(new CustomEvent('goodbye', { bubbles: true, detail: 'eigth' }));

        const events = await sequence;

        expect(events).to.have.length(3);
        expect(events[0].detail).to.equal('third');
        expect(events[1].detail).to.equal('sixth');
        expect(events[2].detail).to.equal('eigth');
      });

      it('does not catch incomplete sequences', async () => {
        const sequence = eventSequencePromise<CustomEvent<string>>([
          'i', 'am', 'a', 'very', 'long', 'event', 'sequence']);
        el.dispatchEvent(new CustomEvent('i', { bubbles: true, detail: 'first' }));
        el.dispatchEvent(new CustomEvent('am', { bubbles: true, detail: 'second' }));

        clock.tick(10000);

        try {
          await sequence;
          expect.fail('promise should not have resolved');
        } catch (error) {
          expect(error).to.equal(
            'timed out after 200 ms while waiting to catch events ' +
            '"a", "very", "long", "event", "sequence"');
        }
      });

      it('does not catch permutations', async () => {
        const sequence = eventSequencePromise<CustomEvent<string>>(['hello', 'world', 'goodbye']);
        el.dispatchEvent(new CustomEvent('goodbye', { bubbles: true, detail: 'first' }));
        el.dispatchEvent(new CustomEvent('world', { bubbles: true, detail: 'second' }));
        el.dispatchEvent(new CustomEvent('hello', { bubbles: true, detail: 'third' }));

        clock.tick(10000);

        try {
          await sequence;
          expect.fail('promise should not have resolved');
        } catch (error) {
          expect(error).to.equal(
            'timed out after 200 ms while waiting to catch events "world", "goodbye"');
        }
      });

      it('times out if nothing is caught', async () => {
        const sequence = eventSequencePromise<CustomEvent<string>>(['hello']);
        clock.tick(10000);

        try {
          await sequence;
          expect.fail('promise should not have resolved');
        } catch (error) {
          expect(error).to.equal(
            'timed out after 200 ms while waiting to catch events "hello"');
        }
      });

      it('never times out if timeoutMillis=0', async () => {
        const sequence = eventSequencePromise<CustomEvent<string>>(['hello'], 0);
        clock.tick(Number.MAX_SAFE_INTEGER);
        el.dispatchEvent(new CustomEvent('hello', { bubbles: true, detail: 'hi' }));

        const events = await sequence;

        expect(events).to.have.length(1);
        expect(events[0].detail).to.equal('hi');
      });
    });
  });

  describe('expectQueryStringToEqual', () => {
    it('matches empty string when query is empty', () => {
      history.pushState(null, '', // these are empty as they do not affect the test.
        window.location.origin + window.location.pathname);
      expectQueryStringToEqual('');
    });

    it('matches the query params when query is not emtpy', () => {
      // reset to known blank state
      history.pushState(null, '', // these are empty as they do not affect the test.
        window.location.origin + window.location.pathname);
      // push some query params
      history.pushState(null, '', '?foo=bar&alpha=beta&alpha=gamma');
      expectQueryStringToEqual('?foo=bar&alpha=beta&alpha=gamma');
    });
  });

  describe('setQueryString', () => {
    it('can set the query to be empty', () => {
      setQueryString('');
      expectQueryStringToEqual('');
    });

    it('can set the query to be not empty', () => {
      setQueryString('?alpha=beta');
      expectQueryStringToEqual('?alpha=beta');
    });
  });
});
