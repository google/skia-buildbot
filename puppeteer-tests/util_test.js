const expect = require('chai').expect;
const express = require('express');
const path = require('path');
const addEventListenersToPuppeteerPage = require('./util').addEventListenersToPuppeteerPage;
const launchBrowser = require('./util').launchBrowser;
const startDemoPageServer = require('./util').startDemoPageServer;

describe('utility functions for Puppeteer tests', async () => {
  let browser;
  before(async () => { browser = await launchBrowser(); });
  after(async () => { await browser.close(); });

  let page;
  beforeEach(async () => { page = await browser.newPage(); });
  afterEach(async () => { await page.close(); });

  describe('addEventListenersToPuppeteerPage', () => {
    let server;
    let testPageUrl;

    beforeEach(async () => {
      // Start an HTTP server on a random, unused port.
      const app = express();
      await new Promise((resolve) => { server = app.listen(0, resolve); });
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
                await triggerEvent('alpha-event',
                                   {msg: 'Only occurrence of alpha-event'});

                await triggerEvent('beta-event',
                                   {msg: '1st occurrence of beta-event'});
                await triggerEvent('beta-event',
                                   {msg: '2nd occurrence of beta-event'});
                await triggerEvent('beta-event',
                                   {msg: '3rd occurrence of beta-event'});

                // No test case should create a promise for this event. The goal
                // is to test that no promise accidentally resolves to this guy.
                await triggerEvent('ignored-event',
                                   {msg: 'No test case should catch me'});

                await triggerEvent('gamma-event',
                                   {msg: '1st occurrence of gamma-event'});
                await triggerEvent('gamma-event',
                                   {msg: '2nd occurrence of gamma-event'});
                await triggerEvent('gamma-event',
                                   {msg: '3rd occurrence of gamma-event'});
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

    afterEach((done) => server.close(done));

    it('renders test page correctly', async () => {
      await page.goto(testPageUrl);
      expect(await page.$eval('h1', (el) => el.innerText))
        .to.equal('Hello, world!');
      expect(await page.$eval('p', (el) => el.innerText))
        .to.equal('I am just a test page.');
    });

    it('catches events in the right order', async () => {
      // Add event listeners.
      const eventNames = [
        'alpha-event',
        'beta-event',
        'gamma-event',
        // We purposefully add a listener for ignored-event, but we won't try
        // to catch any such events. This is to test that uncaught events do
        // not interfere with the events we're trying to catch.
        'ignored-event',
      ];
      const eventPromise = await addEventListenersToPuppeteerPage(page, eventNames);

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
        trackCaughtOrder(eventPromise('gamma-event')),
        trackCaughtOrder(eventPromise('alpha-event')),
        trackCaughtOrder(eventPromise('gamma-event')),
        trackCaughtOrder(eventPromise('beta-event')),
        trackCaughtOrder(eventPromise('gamma-event')),
        trackCaughtOrder(eventPromise('beta-event')),
        trackCaughtOrder(eventPromise('beta-event')),
      ]);

      await page.goto(testPageUrl);

      // Assert that each promise returned the expected event detail.
      const allEvents = await allEventsPromise;
      expect(allEvents).to.have.length(7);
      expect(allEvents[0].msg).to.equal('1st occurrence of gamma-event');
      expect(allEvents[1].msg).to.equal('Only occurrence of alpha-event');
      expect(allEvents[2].msg).to.equal('2nd occurrence of gamma-event');
      expect(allEvents[3].msg).to.equal('1st occurrence of beta-event');
      expect(allEvents[4].msg).to.equal('3rd occurrence of gamma-event');
      expect(allEvents[5].msg).to.equal('2nd occurrence of beta-event');
      expect(allEvents[6].msg).to.equal('3rd occurrence of beta-event');

      // Assert that promises were resolved in the expected order.
      expect(eventsInOrder).to.have.length(7);
      expect(eventsInOrder[0].msg).to.equal('Only occurrence of alpha-event');
      expect(eventsInOrder[1].msg).to.equal('1st occurrence of beta-event');
      expect(eventsInOrder[2].msg).to.equal('2nd occurrence of beta-event');
      expect(eventsInOrder[3].msg).to.equal('3rd occurrence of beta-event');
      expect(eventsInOrder[4].msg).to.equal('1st occurrence of gamma-event');
      expect(eventsInOrder[5].msg).to.equal('2nd occurrence of gamma-event');
      expect(eventsInOrder[6].msg).to.equal('3rd occurrence of gamma-event');
    });

    it('fails if event promise function is called with unknown event',
      async () => {
        const eventPromise = await addEventListenersToPuppeteerPage(page, ['foo-event']);
        expect(() => eventPromise('invalid-event'))
          .to.throw('no event listener for "invalid-event"');
      });
  });

  // TODO(lovisolo): Reenable once util.js and util_test.js have been ported to TypeScript and
  //                 we're thus able to require() a webpack.config.ts file from
  //                 startDemoPageServer().
  describe.skip('startDemoPageServer', () => {
    let baseUrl;
    let stopDemoPageServer;

    before(async () => {
      // Start a demo page server using Perfs's webpack.config.js file.
      const pathToPerfWebpackConfigJs = path.join(__dirname, '..', 'perf', 'webpack.config.js');
      ({ baseUrl, stopDemoPageServer } = await startDemoPageServer(pathToPerfWebpackConfigJs));
    });

    after(async () => { await stopDemoPageServer(); });

    it('should serve a demo page', async () => {
      // Load day-range-sk-demo.html and perform a basic smoke test on a known page.
      await page.goto(`${baseUrl}/dist/day-range-sk.html`);
      expect(await page.$$('day-range-sk')).to.have.length(1);
    });
  });
});
