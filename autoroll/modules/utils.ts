// Utilities for autoroll frontend modules.

import { html, TemplateResult } from 'lit-html';
import { diffDate } from 'common-sk/modules/human';
import { AutoRollMiniStatus } from './rpc';

/**
 * lastCheckInTooOldThresholdMs is the threshold at which we consider a roller's
 * last check-in time to be too old.
 */
const lastCheckInTooOldThresholdMs = 12 * 60 * 60 * 1000; // 12 hours.

/**
 * checkInTimesAddedAt is the approximate Date at which timestamps were added to
 * the autoroller status check-ins. It is used in case a roller has not checked
 * in since before the timestamps were added.
 */
const checkInTimesAddedAt = new Date(1666296000000);

/**
 * GetLastCheckInTime returns the Date at which the roller last checked in.
 */
export function GetLastCheckInTime(st: AutoRollMiniStatus): Date {
    // If the timestamp is missing or is zero or less, then the roller has not
    // checked in since before timestamps were added to the status updates. Use
    // the timestamp at which timestamps were added as an approximation.
    if (!st || !st.timestamp) {
        return checkInTimesAddedAt;
    }
    let lastCheckedIn = new Date(st.timestamp);
    if (lastCheckedIn.getTime() <= 0) {
        lastCheckedIn = checkInTimesAddedAt;
    }
    return lastCheckedIn;
}

/**
 * LastCheckInMessage returns a string indicating the last check-in time of the
 * roller, if it checked in longer than lastCheckInTooOldThresholdMs ago.
 */
export function LastCheckInMessage(st: AutoRollMiniStatus | null | undefined): String {
    if (!st || !st.timestamp) {
        return '';
    }
    const lastedCheckedIn = GetLastCheckInTime(st).getTime();
    const now = new Date().getTime();
    if (now - lastedCheckedIn > lastCheckInTooOldThresholdMs) {
        return 'last checked in ' + diffDate(lastedCheckedIn, now) + ' ago';
    }
    return '';
}

/**
 * LastCheckInSpan returns a TemplateResult indicating the last check-in time of
 * the roller, if it checked in longer than lastCheckInTooOldThresholdMs ago.
 */
export function LastCheckInSpan(st: AutoRollMiniStatus | null | undefined): TemplateResult {
    const msg = LastCheckInMessage(st);
    if (msg) {
        return html`<span class="fg-failure">${msg}</span>`
    }
    return html``;
}