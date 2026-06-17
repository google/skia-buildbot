/**
 * @module modules/anomalies-table-sk/triage-api
 * @description Standalone utility to make edit anomaly requests to the backend.
 */
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { errorMessage } from '../errorMessage';
import { Anomaly } from '../json';

/**
 * Sends a request to /_/triage/edit_anomalies API to edit anomaly data.
 *
 * If edit action is IGNORE, the anomaly will be ignored (bug_id = -2).
 * If edit action is RESET, the bug will be deassociated (bug_id = 0).
 *
 * @param anomalies - The anomalies to modify.
 * @param traceNames - Trace IDs for modified anomalies.
 * @param editAction - An action that corresponds to different behaviors.
 */
export async function makeEditAnomalyRequest(
  anomalies: Anomaly[],
  traceNames: string[],
  editAction: string
): Promise<void> {
  const keys: string[] = anomalies.map((a) => a.id);
  const body = {
    keys: keys,
    trace_names: traceNames,
    action: editAction,
  };

  try {
    const response = await fetch('/_/triage/edit_anomalies', {
      method: 'POST',
      body: JSON.stringify(body),
      headers: {
        'Content-Type': 'application/json',
      },
    });
    await jsonOrThrow(response);

    let bug_id: number | null = null;
    if (editAction === 'RESET') {
      bug_id = 0;
    } else if (editAction === 'IGNORE') {
      bug_id = -2;
    }
    if (bug_id !== null) {
      for (let i = 0; i < anomalies.length; i++) {
        anomalies[i].bug_id = bug_id;
      }
    }
  } catch (error) {
    errorMessage(
      'Edit anomalies request failed due to an internal server error. Please try again.'
    );
    throw error;
  }
}
