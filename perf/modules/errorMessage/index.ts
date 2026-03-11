import { errorMessage as elementsErrorMessage } from '../../../elements-sk/modules/errorMessage';
import { CountMetric, telemetry } from '../telemetry/telemetry';

export interface TelemetryErrorOptions {
  countMetricSource?: CountMetric;
  source?: string;
  errorCode?: string;
}

/**
 * Helper method to convert different error body types into a string.
 */
export const convertToErrorString = (
  errorBody: string | { message: string } | { resp: Response } | object
): string => {
  if (typeof errorBody === 'string') {
    return errorBody;
  }
  if (typeof errorBody === 'object' && errorBody !== null) {
    if ('message' in errorBody) {
      return (errorBody as { message: string }).message;
    }
    if ('resp' in errorBody && (errorBody as { resp: Response }).resp instanceof window.Response) {
      return (errorBody as { resp: Response }).resp.statusText;
    }
  }
  try {
    return JSON.stringify(errorBody);
  } catch (e) {
    return `Failed to report log message from frontend: ${(e as Error).message}`;
  }
};

/**
 * errorMessage dispatches an event with the error message in it.
 * It also optionally tracks error occurrences via a telemetry system
 * and logs the error to the server if a source is provided.
 *
 * duration default to 0, which means the toast doesn't close automatically.
 */
export const errorMessage = (
  message: string | { message: string } | { resp: Response } | object,
  duration: number = 0,
  options: TelemetryErrorOptions = {}
): void => {
  if (options.source) {
    logErrorMessage(message, options.source);
  }

  if (options.countMetricSource) {
    let errorCode = options.errorCode;

    if (!errorCode && isMessageWithResponse(message)) {
      errorCode = message.resp.status.toString();
    }

    if (!errorCode) {
      errorCode = '500';
    }
    telemetry.increaseCounter(options.countMetricSource, {
      source: options.source || 'default',
      errorCode: errorCode,
    });
  }
  elementsErrorMessage(message, duration);
};

/**
 * logErrorMessage logs the error message to the server.
 */
export const logErrorMessage = (
  errorBody: string | { message: string } | { resp: Response } | object,
  errorSource: string
): void => {
  telemetry.reportErrorToServer(convertToErrorString(errorBody), errorSource);
};

/**
 * Type guard to check if an unknown object contains a valid Fetch API Response.
 */
function isMessageWithResponse(msg: unknown): msg is { resp: Response } {
  return (
    typeof msg === 'object' &&
    msg !== null &&
    'resp' in msg &&
    (msg as Record<string, unknown>).resp instanceof Response
  );
}
