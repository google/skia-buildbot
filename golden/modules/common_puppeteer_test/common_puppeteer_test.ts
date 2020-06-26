import { launchBrowser, startDemoPageServer, TestBed } from '../../../puppeteer-tests/util';
import * as path from 'path';
import puppeteer from 'puppeteer';

let browser: puppeteer.Browser;
let testBed: Partial<TestBed>;
export async function loadGoldWebpack() {
    if (testBed) {
        return testBed as TestBed;
    }
    const newTestBed: Partial<TestBed> = {};

    const pathToWebpackConfigTs = path.join(__dirname, '..', '..', 'webpack.config.ts');
    let baseUrl;
    ({ baseUrl } = await startDemoPageServer(pathToWebpackConfigTs));
    newTestBed.baseUrl = baseUrl;
    browser = await launchBrowser();
    testBed = newTestBed;
    setBeforeAfterHooks();
    return testBed as TestBed;
}

function setBeforeAfterHooks() {
    beforeEach(async () => {
        testBed.page = await browser.newPage(); // Make page available to tests.

        // Tell demo pages this is a Puppeteer test. Demo pages should not fake RPC
        // latency, render animations or exhibit any other non-deterministic
        // behavior that could result in differences in the screenshots uploaded to
        // Gold.
        await testBed.page.setCookie({
            url: testBed.baseUrl,
            name: 'puppeteer',
            value: 'true'
        });
    });

    afterEach(async () => {
        await testBed.page!.close();
    });
}
