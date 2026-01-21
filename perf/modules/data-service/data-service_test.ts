import { assert } from 'chai';
import fetchMock from 'fetch-mock';
import { DataService } from './data-service';
import { GraphConfig, ShiftRequest, CommitNumber, FrameResponse } from '../json';

describe('DataService', () => {
  let dataService: DataService;

  beforeEach(() => {
    dataService = DataService.getInstance();
    fetchMock.reset();
  });

  afterEach(() => {
    fetchMock.reset();
  });

  describe('updateShortcut', () => {
    it('returns empty string if graphConfigs is empty', async () => {
      const result = await dataService.updateShortcut([]);
      assert.equal(result, '');
    });

    it('sends POST request and returns id', async () => {
      const graphConfigs: GraphConfig[] = [
        {
          keys: '',
          queries: ['test=query'],
          formulas: [],
        },
      ];
      const response = { id: 'test-shortcut-id' };
      fetchMock.post('/_/shortcut/update', response);

      const result = await dataService.updateShortcut(graphConfigs);

      assert.equal(result, 'test-shortcut-id');
      assert.isTrue(fetchMock.called('/_/shortcut/update'));
      const options = fetchMock.lastOptions('/_/shortcut/update');
      assert.isDefined(options);
      assert.equal(options!.method, 'POST');
      assert.deepEqual(JSON.parse(options!.body as unknown as string), { graphs: graphConfigs });
    });

    it('throws error on failure', async () => {
      const graphConfigs: GraphConfig[] = [
        {
          keys: '',
          queries: ['test=query'],
          formulas: [],
        },
      ];
      fetchMock.post('/_/shortcut/update', 500);

      try {
        await dataService.updateShortcut(graphConfigs);
        assert.fail('Should have thrown an error');
      } catch (_) {
        // Expected
      }
    });
  });

  describe('getInitPage', () => {
    it('sends GET request', async () => {
      const tz = 'America/Los_Angeles';
      const response = { some: 'data' };
      fetchMock.get(`/_/initpage/?tz=${tz}`, response);

      const result = await dataService.getInitPage(tz);
      assert.deepEqual(result, response);
    });
  });

  describe('shift', () => {
    it('sends POST request', async () => {
      const req: ShiftRequest = {
        begin: 123 as CommitNumber,
        end: 456 as CommitNumber,
      };
      const response = { begin: 100, end: 500 };
      fetchMock.post('/_/shift/', response);

      const result = await dataService.shift(req);
      assert.deepEqual(result, response);
      const options = fetchMock.lastOptions('/_/shift/');
      assert.isDefined(options);
      assert.equal(options!.method, 'POST');
      assert.deepEqual(JSON.parse(options!.body as unknown as string), req);
    });
  });

  describe('createShortcut', () => {
    it('sends POST request', async () => {
      const state = { keys: ['k1', 'k2'] };
      const response = { id: 'new-id' };
      fetchMock.post('/_/keys/', response);

      const result = await dataService.createShortcut(state);
      assert.deepEqual(result, response);
    });
  });

  describe('sendFrameRequest', () => {
    it('sends POST request and returns results when finished', async () => {
      const body: any = {
        some: 'request',
      };
      const progressResponse = {
        status: 'Finished',
        messages: [],
        results: {
          some: 'result',
        } as unknown as FrameResponse,
      };

      fetchMock.post('/_/frame/start', progressResponse);

      const result = await dataService.sendFrameRequest(body);
      assert.deepEqual(result, progressResponse.results);
      assert.isTrue(fetchMock.called('/_/frame/start'));
    });
  });
});
