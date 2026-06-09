import { TraceDatabase } from './db';

export interface WorkerMessage {
  type: string;
  payload?: any;
}

export class ExploreWorkerController {
  private worker: Worker | null = null;

  private ready = false;

  private workerRequestId = 0;

  constructor(
    private onLoaded: () => void,
    private onReady: () => void,
    private onProgress: (payload: any) => void,
    private onError: (message: string) => void,
    private onParamsReady: (payload: any) => void,
    private onResult: (payload: any) => void,
    private onSuggestResult: (payload: any, idx: number, requestId: number) => void
  ) {}

  public init() {
    try {
      console.log('WorkerController: Attempting to create Web Worker via Blob');
      const workerUrl = (window as any).WORKER_URL || '/dist/explore-multi-v2-sk/filter.worker.js';

      if (workerUrl.startsWith('data:')) {
        this.worker = new Worker(workerUrl);
        this.worker.onmessage = (e: MessageEvent) => this.handleMessage(e);
        this.worker.onerror = (e: ErrorEvent) => {
          console.error('WorkerController: Worker error event:', e);
          this.onError(`Worker error: ${e.message}`);
        };
        console.log('WorkerController: Worker created directly from Data URL');
      } else {
        console.log('WorkerController: Fetching worker code from:', workerUrl);
        fetch(workerUrl)
          .then(async (resp) => {
            if (!resp.ok) throw new Error(`Failed to fetch worker: ${resp.statusText}`);
            return await resp.text();
          })
          .then((code) => {
            const blob = new Blob([code], { type: 'application/javascript' });
            const blobUrl = URL.createObjectURL(blob);
            this.worker = new Worker(blobUrl);
            this.worker.onmessage = (e: MessageEvent) => this.handleMessage(e);
            this.worker.onerror = (e: ErrorEvent) => {
              console.error('WorkerController: Worker error event:', e);
              this.onError(`Worker error: ${e.message}`);
            };
            console.log('WorkerController: Worker created via Blob URL from fetched code');
          })
          .catch((e) => {
            console.error('WorkerController: Failed to fetch or create worker:', e);
            this.onError(`Failed to fetch or create worker: ${e.message}`);
          });
      }
    } catch (e: any) {
      console.error('WorkerController: Failed to initialize worker flow:', e);
      this.onError(`Failed to initialize worker flow: ${e.message}`);
    }
  }

  private handleMessage(e: MessageEvent) {
    const { type, payload } = e.data;
    console.log('WorkerController: Received message from worker:', type, payload);

    if (type === 'LOADED') {
      console.log('WorkerController: Worker loaded, starting data fetch on main thread');
      this.onLoaded();
      void this.fetchDataForWorker();
    } else if (type === 'PROGRESS') {
      this.onProgress(payload);
    } else if (type === 'READY') {
      console.log('WorkerController: Worker Ready');
      this.ready = true;
      this.onReady();
    } else if (type === 'ERROR') {
      this.onError(payload.message);
    } else if (type === 'PARAMS_READY') {
      this.onParamsReady(payload);
    } else if (type === 'RESULT') {
      this.onResult(payload);
    } else if (type === 'SUGGEST_RESULT') {
      const { idx, requestId } = e.data;
      this.onSuggestResult(payload, idx, requestId);
    }
  }

  private async fetchDataForWorker() {
    try {
      console.log('WorkerController: Fetching meta.json');
      const metaResp = await fetch('/_/wasm/meta.json?t=' + Date.now());
      const meta = await metaResp.json();
      const { version } = meta;
      console.log('WorkerController: meta.json loaded, version:', version);

      const db = new TraceDatabase();

      // Check cache
      const cachedParams = await db.get(`static:params:${version}`);
      const cachedWasm = await db.get(`static:wasm:${version}`);
      const cachedTraces = await db.get(`static:traces:${version}`);

      let params, wasmBuffer, tracesBuffer;

      if (cachedParams && cachedWasm && cachedTraces) {
        console.log('WorkerController: Serving static files from cache');
        params = cachedParams;
        wasmBuffer = cachedWasm;
        tracesBuffer = cachedTraces;
      } else {
        console.log('WorkerController: Fetching params, wasm, and traces');
        const [paramsResp, wasmResp, tracesResp] = await Promise.all([
          fetch(`/_/wasm/params.json?v=${version}`),
          fetch(`/dist/explore-multi-v2-sk/filter.wasm?v=${version}`),
          fetch(`/_/wasm/traces.bin?v=${version}`),
        ]);

        console.log('WorkerController: Reading responses');
        params = await paramsResp.json();
        wasmBuffer = await wasmResp.arrayBuffer();
        tracesBuffer = await tracesResp.arrayBuffer();

        console.log('WorkerController: Caching static files');
        await db.set(`static:params:${version}`, params);
        await db.set(`static:wasm:${version}`, wasmBuffer);
        await db.set(`static:traces:${version}`, tracesBuffer);
      }

      console.log('WorkerController: All data fetched, sending INIT to worker');
      this.worker!.postMessage(
        {
          type: 'INIT',
          payload: { meta, params, wasmBuffer, tracesBuffer },
        },
        [wasmBuffer, tracesBuffer]
      );
      console.log('WorkerController: INIT message sent');
    } catch (e: any) {
      console.error('WorkerController: Failed to fetch data for worker:', e);
      this.onError(`Failed to fetch data for worker: ${e.message}`);
    }
  }

  public filter(queries: any[], numUserQueries: number) {
    if (!this.worker || !this.ready) {
      console.warn('WorkerController: Worker not ready for filtering');
      return;
    }
    this.workerRequestId++;
    this.worker.postMessage({
      type: 'FILTER',
      queries: queries,
      numUserQueries: numUserQueries,
      requestId: this.workerRequestId,
    });
  }

  public suggest(query: string, currentQuery: any, idx: number, availableParams?: any[]): number {
    if (!this.worker || !this.ready) {
      console.warn('WorkerController: Worker not ready for suggestions');
      return 0;
    }
    this.workerRequestId++;
    this.worker.postMessage({
      type: 'SUGGEST',
      query: query,
      currentQuery: currentQuery,
      requestId: this.workerRequestId,
      idx: idx,
      availableParams: availableParams,
    });
    return this.workerRequestId;
  }

  public getRandomTrace(callback: (query: any) => void) {
    if (!this.worker || !this.ready) {
      callback(null);
      return;
    }
    const handler = (e: MessageEvent) => {
      if (e.data.type === 'RANDOM_TRACE_RESULT') {
        this.worker!.removeEventListener('message', handler);
        callback(e.data.payload);
      }
    };
    this.worker.addEventListener('message', handler);
    this.worker.postMessage({ type: 'GET_RANDOM_TRACE' });
  }

  public isReady(): boolean {
    return this.ready;
  }

  public terminate() {
    if (this.worker) {
      this.worker.terminate();
      this.worker = null;
      this.ready = false;
    }
  }
}
