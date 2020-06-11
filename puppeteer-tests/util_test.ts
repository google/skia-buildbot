import * as path from 'path';
import * as http from 'http';
import * as net from 'net';
import express from 'express';
import puppeteer from 'puppeteer';
import { expect } from 'chai';
import { EventName, addEventListenersToPuppeteerPage, launchBrowser, startDemoPageServer } from './util';

describe('utility functions for Puppeteer tests', async () => {
  let browser: puppeteer.Browser;
  before(async () => { browser = await launchBrowser(); });
  after(async () => { await browser.close(); });

  let page: puppeteer.Page;
  beforeEach(async () => { page = await browser.newPage(); });
  afterEach(async () => { await page.close(); });

  describe('addEventListenersToPuppeteerPage', () => {
    let server: http.Server;
    let testPageUrl: string;

    // Type for the custom event details used in this test.
    interface TestEventDetail {
      msg: string;
    };

    beforeEach(async () => {
      // Start an HTTP server on a random, unused port.
      const app = express();
      await new Promise((resolve) => { server = app.listen(0, resolve); });
      testPageUrl = `http://localhost:${(server!.address() as net.AddressInfo).port}/`;

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
      expect(await page.$eval('h1', (el) => (el as HTMLElement).innerText))
        .to.equal('Hello, world!');
      expect(await page.$eval('p', (el) => (el as HTMLElement).innerText))
        .to.equal('I am just a test page.');
    });

    it('catches events in the right order', async () => {
      // Add event listeners.
      const eventNames: EventName[] = [
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
      const eventDetailsInOrder: TestEventDetail[] = [];
      const trackCaughtOrder = async (anEventPromise: Promise<TestEventDetail>) => {
        const detail = await anEventPromise;
        eventDetailsInOrder.push(detail);
        return detail;
      };

      // We create event promises in an arbitrary order to test that this has no
      // effect in the order that events are caught.
      const allEventDetailsPromise = Promise.all([
        trackCaughtOrder(eventPromise<TestEventDetail>('gamma-event')),
        trackCaughtOrder(eventPromise<TestEventDetail>('alpha-event')),
        trackCaughtOrder(eventPromise<TestEventDetail>('gamma-event')),
        trackCaughtOrder(eventPromise<TestEventDetail>('beta-event')),
        trackCaughtOrder(eventPromise<TestEventDetail>('gamma-event')),
        trackCaughtOrder(eventPromise<TestEventDetail>('beta-event')),
        trackCaughtOrder(eventPromise<TestEventDetail>('beta-event')),
      ]);

      await page.goto(testPageUrl);

      // Assert that each promise returned the expected event detail.
      const allEventDetails = await allEventDetailsPromise;
      expect(allEventDetails).to.have.length(7);
      expect(allEventDetails[0].msg).to.equal('1st occurrence of gamma-event');
      expect(allEventDetails[1].msg).to.equal('Only occurrence of alpha-event');
      expect(allEventDetails[2].msg).to.equal('2nd occurrence of gamma-event');
      expect(allEventDetails[3].msg).to.equal('1st occurrence of beta-event');
      expect(allEventDetails[4].msg).to.equal('3rd occurrence of gamma-event');
      expect(allEventDetails[5].msg).to.equal('2nd occurrence of beta-event');
      expect(allEventDetails[6].msg).to.equal('3rd occurrence of beta-event');

      // Assert that promises were resolved in the expected order.
      expect(eventDetailsInOrder).to.have.length(7);
      expect(eventDetailsInOrder[0].msg).to.equal('Only occurrence of alpha-event');
      expect(eventDetailsInOrder[1].msg).to.equal('1st occurrence of beta-event');
      expect(eventDetailsInOrder[2].msg).to.equal('2nd occurrence of beta-event');
      expect(eventDetailsInOrder[3].msg).to.equal('3rd occurrence of beta-event');
      expect(eventDetailsInOrder[4].msg).to.equal('1st occurrence of gamma-event');
      expect(eventDetailsInOrder[5].msg).to.equal('2nd occurrence of gamma-event');
      expect(eventDetailsInOrder[6].msg).to.equal('3rd occurrence of gamma-event');
    });

    it('fails if event promise function is called with unknown event',
      async () => {
        const eventPromise = await addEventListenersToPuppeteerPage(page, ['foo-event']);
        expect(() => eventPromise('invalid-event'))
          .to.throw('no event listener for "invalid-event"');
      });
  });

  describe('startDemoPageServer', () => {
    let baseUrl: string;
    let stopDemoPageServer: () => Promise<void>;

    before(async () => {
      // Start a demo page server using Perfs's webpack.config.ts file.
      const pathToPerfWebpackConfigTs = path.join(__dirname, '..', 'perf', 'webpack.config.ts');
      ({ baseUrl, stopDemoPageServer } = await startDemoPageServer(pathToPerfWebpackConfigTs));
    });

    after(async () => { await stopDemoPageServer(); });

    it('should serve a demo page', async () => {
      // Load day-range-sk-demo.html and perform a basic smoke test on a known page.
      await page.goto(`${baseUrl}/dist/day-range-sk.html`);
      expect(await page.$$('day-range-sk')).to.have.length(1);
    });
  });
});
