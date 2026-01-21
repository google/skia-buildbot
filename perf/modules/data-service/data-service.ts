import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import {
  FrameRequest,
  FrameResponse,
  ShiftRequest,
  ShiftResponse,
  progress,
  GraphConfig,
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
}

/**
 * Handles all data fetching and manipulation requests to the backend.
 */
export class DataService {
  private static instance: DataService;

  private constructor() {}

  public static getInstance(): DataService {
    if (!DataService.instance) {
      DataService.instance = new DataService();
    }
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
   * Fetches the initial page data.
   */
  async getInitPage(tz: string): Promise<any> {
    return await this.fetchJson(`/_/initpage/?tz=${tz}`, {
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
   * Creates a shortcut for the keys.
   */
  async createShortcut(state: { keys: string[] }): Promise<{ id: string }> {
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
