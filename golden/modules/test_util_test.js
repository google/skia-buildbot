import {
  eventPromise,
  noEventPromise,
  expectQueryStringToEqual
} from './test_util';

describe('test utilities', () => {
  describe('event promise functions', () => {
    let el; // Element that we'll dispatch custom events from.
    let clock;

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
        const hello = eventPromise('hello');
        el.dispatchEvent(new CustomEvent('hello', {bubbles: true, detail: 'hi'}));
        const ev = await hello;
        expect(ev.detail).to.equal('hi');
      });

      it('times out if event is not caught', async () => {
        const hello = eventPromise('hello', 5000);
        el.dispatchEvent(new CustomEvent('bye', {bubbles: true}));
        clock.tick(10000);
        try {
          await hello;
          expect.fail('promise should not have resolved');
        } catch(error) {
          expect(error.message).to.equal(
            'timed out after 5000 ms while waiting to catch event "hello"');
        }
      });

      it('never times out if timeoutMillis=0', async () => {
        const hello = eventPromise('hello', 0);
        clock.tick(Number.MAX_SAFE_INTEGER);
        el.dispatchEvent(new CustomEvent('hello', {bubbles: true, detail: 'hi'}));
        const ev = await hello;
        expect(ev.detail).to.equal('hi');
      });
    });

    describe('noEventPromise', () => {
      it('resolves when event is NOT caught', async () => {
        const noHello = noEventPromise('hello', 200);
        el.dispatchEvent(new CustomEvent('bye', {bubbles: true}));
        clock.tick(10000);
        await noHello;
      });

      it('rejects if event is caught', async () => {
        const noHello = noEventPromise('hello', 200);
        el.dispatchEvent(new CustomEvent('hello', {bubbles: true}));
        try {
          await noHello;
          expect.fail('promise should not have resolved');
        } catch(error) {
          expect(error.message).to.equal(
              'event "hello" was caught when none was expected');
        }
      });

      it('never resolves if timeoutMillis=0', async () => {
        const noHello = noEventPromise('hello', 0);
        clock.tick(Number.MAX_SAFE_INTEGER);
        el.dispatchEvent(new CustomEvent('hello', {bubbles: true}));
        try {
          await noHello;
          expect.fail('promise should not have resolved');
        } catch(error) {
          expect(error.message).to.equal(
              'event "hello" was caught when none was expected');
        }
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
});
