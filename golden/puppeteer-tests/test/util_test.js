const expect = require('chai').expect;
const express = require('express');
const addEventListenersToPuppeteerPage = require('./util').addEventListenersToPuppeteerPage;
const launchBrowser = require('./util').launchBrowser;
const startDemoPageServer = require('./util').startDemoPageServer;

describe('util', async () => {
  let browser;
  before(async () => { browser = await launchBrowser(); });
  after(async () => { await browser.close(); });

  let page;
  beforeEach(async () => { page = await browser.newPage(); });
  afterEach(async () => { await page.close(); });

  describe('addEventListenersToPuppeteerPage', () => {
    let server, testPageUrl;

    beforeEach(async () => {
      // Start an HTTP server on a random, unused port.
      const app = express();
      await new Promise(resolve => { server = app.listen(0, resolve); });
      testPageUrl = `http://localhost:${server.address().port}/`;

      // Serve a test page that triggers various custom events.
      app.get('/', (req, res) => res.send(`
        <html>
          <head>
            <script>
              async function triggerEvent(name, detail) {
                document.dispatchEvent(
                    new CustomEvent(name, {detail: detail, bubbles: true}));
                await new Promise((resolve) => setTimeout(resolve, 10));
              }

              // This is necessary as await is not allowed outside a function.
              async function main() {
                await triggerEvent('foo-event',
                                   {msg: 'Only occurrence of foo-event'});

                await triggerEvent('bar-event',
                                   {msg: '1st occurrence of bar-event'});
                await triggerEvent('bar-event',
                                   {msg: '2nd occurrence of bar-event'});
                await triggerEvent('bar-event',
                                   {msg: '3rd occurrence of bar-event'});

                // No test case should create a promise for this event. The goal
                // is to test that no promise accidentally resolves to this guy.
                await triggerEvent('ignored-event',
                                   {msg: 'No test case should catch me'});

                await triggerEvent('baz-event',
                                   {msg: '1st occurrence of baz-event'});
                await triggerEvent('baz-event',
                                   {msg: '2nd occurrence of baz-event'});
                await triggerEvent('baz-event',
                                   {msg: '3rd occurrence of baz-event'});
              }
              main();
            </script>
          </head>
          <body>
            <h1>Hello, world!</h1>
            <p>I am just a test page.</p>
          </body>
        </html>
      `));
    });

    afterEach(async () => {
      await new Promise(resolve => server.close(resolve));
    });

    it('test page renders correctly', async () => {
      await page.goto(testPageUrl);
      expect(await page.$eval('h1', (el) => el.innerText))
          .to.equal('Hello, world!');
      expect(await page.$eval('p', (el) => el.innerText))
          .to.equal('I am just a test page.');
    });

    it('catches events in the right order', async () => {
      // Add event listeners.
      const eventNames = [
          'foo-event',
          'bar-event',
          'baz-event',
          'ignored-event'
      ];
      const eventPromise =
          await addEventListenersToPuppeteerPage(page, eventNames);

      // We will collect the Event object details in the order they are caught.
      const eventsInOrder = [];
      const trackCaughtOrder = async (anEventPromise) => {
        const detail = await anEventPromise;
        eventsInOrder.push(detail);
        return detail;
      };

      // We create event promises in an arbitrary order to test that this has no
      // effect in the order that events are caught.
      const allEventsPromise = Promise.all([
          trackCaughtOrder(eventPromise('baz-event')),
          trackCaughtOrder(eventPromise('foo-event')),
          trackCaughtOrder(eventPromise('baz-event')),
          trackCaughtOrder(eventPromise('bar-event')),
          trackCaughtOrder(eventPromise('baz-event')),
          trackCaughtOrder(eventPromise('bar-event')),
          trackCaughtOrder(eventPromise('bar-event')),
      ]);

      await page.goto(testPageUrl);

      // Assert that each promise returned the expected event detail.
      const allEvents = await allEventsPromise;
      expect(allEvents).to.have.length(7);
      expect(allEvents[0].msg).to.equal('1st occurrence of baz-event');
      expect(allEvents[1].msg).to.equal('Only occurrence of foo-event');
      expect(allEvents[2].msg).to.equal('2nd occurrence of baz-event');
      expect(allEvents[3].msg).to.equal('1st occurrence of bar-event');
      expect(allEvents[4].msg).to.equal('3rd occurrence of baz-event');
      expect(allEvents[5].msg).to.equal('2nd occurrence of bar-event');
      expect(allEvents[6].msg).to.equal('3rd occurrence of bar-event');

      // Assert that promises were resolved in the expected order.
      expect(eventsInOrder).to.have.length(7);
      expect(eventsInOrder[0].msg).to.equal('Only occurrence of foo-event');
      expect(eventsInOrder[1].msg).to.equal('1st occurrence of bar-event');
      expect(eventsInOrder[2].msg).to.equal('2nd occurrence of bar-event');
      expect(eventsInOrder[3].msg).to.equal('3rd occurrence of bar-event');
      expect(eventsInOrder[4].msg).to.equal('1st occurrence of baz-event');
      expect(eventsInOrder[5].msg).to.equal('2nd occurrence of baz-event');
      expect(eventsInOrder[6].msg).to.equal('3rd occurrence of baz-event');
    });

    it('fails if event promise function is called with unknown event',
        async () => {
          const eventPromise =
              await addEventListenersToPuppeteerPage(page, ['foo-event']);
          expect(() => eventPromise('invalid-event'))
              .to.throw('no event listener for "invalid-event"');
    });
  });

  describe('startDemoPageServer', () => {
    let baseUrl, stopDemoPageServer;
    before(async () => {
      ({baseUrl, stopDemoPageServer} = await startDemoPageServer());
    });
    after(async () => { await stopDemoPageServer(); });

    it('should serve some demo pages', async () => {
      // Load changelists-page-sk-demo.html and perform a basic sanity check.
      await page.goto(`${baseUrl}/dist/changelists-page-sk.html`);
      expect(await page.$$('changelists-page-sk')).to.have.length(1);

      // Same with triagelog-page-sk-demo.html.
      await page.goto(`${baseUrl}/dist/triagelog-page-sk.html`);
      expect(await page.$$('triagelog-page-sk')).to.have.length(1);

      // This demo page contains three instances of corpus-selector-sk.
      await page.goto(`${baseUrl}/dist/corpus-selector-sk.html`);
      expect(await page.$$('corpus-selector-sk')).to.have.length(3);
    })
  });
});
