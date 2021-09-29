import http from 'http';
import net from 'net';
import express from 'express';
import {
  Browser, Page, ElementHandle, Serializable,
} from 'puppeteer';
import { launchBrowser } from '../../../puppeteer-tests/util';
import { PageObjectElement } from './page_object_element';
import { TestBed, describePageObjectElement } from './page_object_element_test_cases';

describe('PageObjectElement on Puppeter', () => {
  let browser: Browser;
  let page: Page;

  let httpServer: http.Server;
  let testPageUrl: string;
  let testPageHtml: string;

  before(async () => {
    // Configure an Express app to serve the test page.
    const app = express();
    app.get('/', (_, res) => res.send(testPageHtml));

    // Launch HTTP server on random port.
    await new Promise((resolve) => {
      httpServer = app.listen(0, () => resolve(undefined));
    });
    const port = (httpServer!.address() as net.AddressInfo).port;

    // The test page will be served on this URL.
    testPageUrl = `http://localhost:${port}/`;

    // Launch browser.
    browser = await launchBrowser();
  });

  after(async () => {
    // Shut down browser and HTTP server.
    await browser.close();
    await new Promise((resolve) => httpServer.close(resolve));
  });

  beforeEach(async () => { page = await browser.newPage(); });
  afterEach(async () => { await page.close(); });

  // A handle for the top-level element in the HTML provided via the test bed.
  let container: ElementHandle<HTMLElement>;

  const testBed: TestBed = {
    setUpPageObjectElement: async (html: string) => {
      // Set up the test page.
      testPageHtml = `
        <html>
          <head><title>Test page</title></head>
          <body>${html}</body>
        </html>
      `;
      await page.goto(testPageUrl);

      // Make sure there is only one top-level element.
      const elements = await page.$$('body > *');
      if (elements.length !== 1) {
        throw new Error('the given HTML contains more than one top-level element');
      }

      // Retrieve the top-level element and wrap it inside a PageObjectElement.
      container = elements[0];
      return new PageObjectElement(container);
    },

    evaluate: async <T extends Serializable | void = void>(fn: (el: HTMLElement)=> T) => await container.evaluate(fn) as T,
  };

  describePageObjectElement(testBed);
});
