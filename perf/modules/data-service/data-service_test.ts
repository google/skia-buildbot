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

  describe('getShortcut', () => {
    it('sends POST request and returns graphs', async () => {
      const id = 'test-id';
      const response = {
        graphs: [
          {
            keys: '',
            queries: ['test=query'],
            formulas: [],
          },
        ],
      };
      fetchMock.post('/_/shortcut/get', response);

      const result = await dataService.getShortcut(id);
      assert.deepEqual(result, response.graphs);
      const options = fetchMock.lastOptions('/_/shortcut/get');
      assert.isDefined(options);
      assert.equal(options!.method, 'POST');
      assert.deepEqual(JSON.parse(options!.body as unknown as string), { ID: id });
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

  describe('getUserIssues', () => {
    it('sends POST request', async () => {
      const req = {
        trace_keys: ['k1', 'k2'],
        begin_commit_position: 100,
        end_commit_position: 200,
      };
      const response = {
        UserIssues: [
          {
            UserId: 'user@example.com',
            TraceKey: 'k1',
            CommitPosition: 150,
            IssueId: 12345,
          },
        ],
      };
      fetchMock.post('/_/user_issues/', response);

      const result = await dataService.getUserIssues(req);
      assert.deepEqual(result, response);
      const options = fetchMock.lastOptions('/_/user_issues/');
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
