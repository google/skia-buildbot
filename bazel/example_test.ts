/**
 * Example Go test for the test_on_env Bazel rule.
 */

import { expect } from 'chai';
import fs from 'fs';
import path from 'path';
import puppeteer from 'puppeteer';

const ENV_PORT_FILE_BASE_NAME = 'port';

const readPort = () => {
  const envDir = process.env.ENV_DIR;
  if (!envDir) throw new Error('required environment variable ENV_DIR is unset');
  return parseInt(fs.readFileSync(path.join(envDir, ENV_PORT_FILE_BASE_NAME), 'utf8'));
}

describe('example test', () => {
  let baseUrl: string;
  let browser: puppeteer.Browser;
  let page: puppeteer.Page;

  before(async () => {
    baseUrl = 'http://localhost:' + readPort();
    browser = await puppeteer.launch({args: ['--disable-dev-shm-usage', '--no-sandbox']})
  });
  after(async () => { await browser.close() });
  beforeEach(async () => { page = await browser.newPage(); });

  const getStatusCodeAndPageText = async (url: string): Promise<[number, string]> => {
    const response = await page.goto(url);
    const pageText = await page.$eval('body', el => (el as HTMLBodyElement).innerText);
    return [response!.status(), pageText.trim()]
  }

  it('should get a 404 error at /', async () => {
    expect(await getStatusCodeAndPageText(baseUrl + '/'))
        .to.deep.equal([404, '404 page not found']);
  });

  it('should get a 400 error at /echo', async () => {
    expect(await getStatusCodeAndPageText(baseUrl + '/echo'))
        .to.deep.equal([400, 'Error: Query parameter "msg" is empty or missing.']);
  });

  it('should echo the message', async () => {
    expect(await getStatusCodeAndPageText(baseUrl + '/echo?msg=Hello%2C%20world!'))
        .to.deep.equal([200, 'Hello, world!']);
  });
});
