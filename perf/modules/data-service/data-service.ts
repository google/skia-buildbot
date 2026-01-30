import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import {
  FrameRequest,
  FrameResponse,
  GetUserIssuesForTraceKeysRequest,
  GetUserIssuesForTraceKeysResponse,
  GraphConfig,
  ShiftRequest,
  ShiftResponse,
  progress,
  QueryConfig,
} from '../json';
import {
  messageByName,
  messagesToErrorString,
  messagesToPreString,
  startRequest,
  RequestOptions,
} from '../progress/progress';

/**
 * Custom error class for DataService operations.
 */
export class DataServiceError extends Error {
  status?: number;

  constructor(message: string, status?: number) {
    super(message);
    this.name = 'DataServiceError';
    this.status = status;
  }
}

export interface SendFrameRequestOptions {
  onStart?: () => void;
  onProgress?: (msg: string) => void;
  onMessage?: (msg: string) => void;
  onSettled?: () => void;
  pollingIntervalMs?: number;
}

/**
 * Handles all data fetching and manipulation requests to the backend.
 */
export class DataService {
  private static instance: DataService = new DataService();

  private constructor() {}

  public static getInstance(): DataService {
    return DataService.instance;
  }

  /**
   * Helper to fetch JSON from a URL.
   */
  private async fetchJson<T>(url: string, init?: RequestInit): Promise<T> {
    try {
      const response = await fetch(url, init);
      return await jsonOrThrow(response);
    } catch (error: any) {
      throw new DataServiceError(error.message || error.toString(), error.status);
    }
  }

  /**
   * Creates a shortcut ID for the given Graph Configs.
   */
  async updateShortcut(graphConfigs: GraphConfig[]): Promise<string> {
    // Skip this call when running locally to avoid 500 errors from the proxy/backend.
    if ((window as any).perf && (window as any).perf.disable_shortcut_update) {
      console.log('Skipping updateShortcut due to configuration');
      return '';
    }

    if (graphConfigs.length === 0) {
      return '';
    }

    const body = {
      graphs: graphConfigs,
    };

    const json = await this.fetchJson<{ id: string }>('/_/shortcut/update', {
      method: 'POST',
      body: JSON.stringify(body),
      headers: {
        'Content-Type': 'application/json',
      },
    });
    return json.id;
  }

  /**
   * Fetches the Graph Configs for a given shortcut ID.
   */
  async getShortcut(id: string): Promise<GraphConfig[]> {
    const body = {
      ID: id,
    };

    const json = await this.fetchJson<{ graphs: GraphConfig[] }>('/_/shortcut/get', {
      method: 'POST',
      body: JSON.stringify(body),
      headers: {
        'Content-Type': 'application/json',
      },
    });
    return json.graphs;
  }

  /**
   * Fetches the initial page data.
   */
  async getInitPage(tz: string): Promise<any> {
    return await this.fetchJson(`/_/initpage/?tz=${tz}`, {
      method: 'GET',
    });
  }

  /**
   * Fetches the default configuration.
   */
  async getDefaults(): Promise<QueryConfig> {
    return await this.fetchJson('/_/defaults/', {
      method: 'GET',
    });
  }

  /**
   * Calculates the new range change based on a shift request.
   */
  async shift(req: ShiftRequest): Promise<ShiftResponse> {
    return await this.fetchJson('/_/shift/', {
      method: 'POST',
      body: JSON.stringify(req),
      headers: {
        'Content-Type': 'application/json',
      },
    });
  }

  /**
   * Fetches user issues for the given trace keys and commit range.
   */
  async getUserIssues(
    req: GetUserIssuesForTraceKeysRequest
  ): Promise<GetUserIssuesForTraceKeysResponse> {
    return await this.fetchJson('/_/user_issues/', {
      method: 'POST',
      body: JSON.stringify(req),
      headers: {
        'Content-Type': 'application/json',
      },
    });
  }

  /**
   * Creates a shortcut for the keys.
   */
  async createShortcut(state: { keys: string[] }): Promise<{ id: string }> {
    // Skip this call when running locally to avoid 500 errors from the proxy/backend.
    if ((window as any).perf && (window as any).perf.disable_shortcut_update) {
      console.log('Skipping createShortcut due to configuration');
      return { id: '' };
    }

    return await this.fetchJson('/_/keys/', {
      method: 'POST',
      body: JSON.stringify(state),
      headers: {
        'Content-Type': 'application/json',
      },
    });
  }

  /**
   * Starts the frame request and returns the resulting data.
   *
   * @param body - The frame request body.
   * @param options - Optional configuration for the request lifecycle and callbacks.
   */
  async sendFrameRequest(
    body: FrameRequest,
    options: SendFrameRequestOptions = {}
  ): Promise<FrameResponse> {
    body.tz = Intl.DateTimeFormat().resolvedOptions().timeZone;

    const requestOptions: RequestOptions = {
      onStart: options.onStart,
      onSettled: options.onSettled,
      pollingIntervalMs: options.pollingIntervalMs,
      onProgressUpdate: (prog: progress.SerializedProgress) => {
        if (options.onProgress) {
          options.onProgress(messagesToPreString(prog.messages || []));
        }
      },
    };

    const finishedProg = await startRequest('/_/frame/start', body, requestOptions);

    if (finishedProg.status !== 'Finished') {
      throw new DataServiceError(messagesToErrorString(finishedProg.messages));
    }
    const msg = messageByName(finishedProg.messages, 'Message');
    if (msg && options.onMessage) {
      options.onMessage(msg);
    }

    return finishedProg.results as FrameResponse;
  }
}
