/**
 * @fileoverview This file defines a function to report frontend errors to the backend.
 */

interface FrontendErrorLog {
  message: string;
  source: string;
}

/**
 * Logs an error message to the backend.
 * Sends the data immediately to the `/_/fe_error_log` endpoint.
 */
export async function reportErrorToServer(errorBody: string, errorSource: string) {
  const errorLog: FrontendErrorLog = {
    message: errorBody,
    source: errorSource,
  };

  try {
    const response = await fetch('/_/fe_error_log', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(errorLog),
    });

    if (!response.ok) {
      console.error('Failed to send frontend error log. Status:', response.status);
    }
  } catch (e) {
    console.error(e, 'Failed to send frontend error log:', errorLog);
  }
}
