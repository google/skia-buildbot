import type { WorkerConfig } from './worker/filter.worker';

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
    private onSuggestResult: (payload: any, idx: number, requestId: number) => void,
    private onStatus: (message: string) => void
  ) {}

  public init() {
    try {
      console.log('WorkerController: Creating Web Worker');
      let workerUrl = (window as any).WORKER_URL || '/dist/explore-multi-v2-sk/filter.worker.js';
      const version = (window as any).perf?.app_version;
      if (version && !workerUrl.startsWith('data:')) {
        const separator = workerUrl.includes('?') ? '&' : '?';
        workerUrl = `${workerUrl}${separator}v=${version}`;
      }

      const setupWorker = (w: Worker) => {
        this.worker = w;
        this.worker.onmessage = (e: MessageEvent) => this.handleMessage(e);
        this.worker.onerror = (e: ErrorEvent) => {
          console.error('WorkerController: Worker error event:', e);
          this.onError(`Worker error: ${e.message}`);
        };
      };

      if (workerUrl.startsWith('data:')) {
        setupWorker(new Worker(workerUrl));
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
            setupWorker(new Worker(blobUrl));
            console.log('WorkerController: Worker created via Blob URL');
          })
          .catch((e) => {
            console.error('WorkerController: Failed to fetch or create worker:', e);
            this.onError(`Failed to fetch or create worker: ${e.message}`);
          });
      }
    } catch (e: any) {
      console.error('WorkerController: Failed to initialize worker:', e);
      this.onError(`Failed to initialize worker: ${e.message}`);
    }
  }

  private handleMessage(e: MessageEvent) {
    const { type, payload } = e.data;

    if (type === 'LOADED') {
      console.log('WorkerController: Worker loaded, sending INIT config');
      this.onLoaded();

      const origin = window.location.origin;
      // Send config to worker
      const config: WorkerConfig = {
        metaUrl: `${origin}/_/wasm/meta.json`,
        paramsUrlTemplate: `${origin}/_/wasm/params.json?v={version}`,
        wasmUrlTemplate: `${origin}/dist/explore-multi-v2-sk/filter.wasm?v={version}`,
        tracesUrlTemplate: `${origin}/_/wasm/traces.bin?v={version}`,
      };
      this.worker!.postMessage({
        type: 'INIT',
        payload: config,
      });
    } else if (type === 'STATUS') {
      this.onStatus(payload.message);
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

  public filter(queries: any[], numUserQueries: number, requestId?: number): number {
    if (!this.worker || !this.ready) {
      console.warn('WorkerController: Worker not ready for filtering');
      return -1;
    }
    const finalRequestId = requestId ?? ++this.workerRequestId;
    this.workerRequestId = Math.max(this.workerRequestId, finalRequestId);
    this.worker.postMessage({
      type: 'FILTER',
      queries: queries,
      numUserQueries: numUserQueries,
      requestId: finalRequestId,
    });
    return finalRequestId;
  }

  public suggest(
    query: string,
    currentQuery: any,
    idx: number,
    availableParams?: any[],
    includeParams?: string[]
  ): number {
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
      includeParams: includeParams,
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
