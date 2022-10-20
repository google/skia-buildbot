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
 * LastCheckInMessage returns a string indicating the last check-in time of the
 * roller, if it checked in longer than lastCheckInTooOldThresholdMs ago.
 */
export function LastCheckInMessage(st: AutoRollMiniStatus | null | undefined): String {
    if (!st || !st.timestamp) {
        return '';
    }
    const lastReported = new Date(st.timestamp).getTime();
    if (lastReported <= 0) {
        return '';
    }
    const now = new Date().getTime();
    if (now - lastReported > lastCheckInTooOldThresholdMs) {
        return 'last checked in ' + diffDate(lastReported, now) + ' ago';
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