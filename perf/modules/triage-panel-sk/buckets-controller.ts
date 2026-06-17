/**
 * @module modules/triage-panel-sk/buckets-controller
 * @description Reactive controller encapsulating triage bucket state management and localStorage persistence.
 */
import { ReactiveController, ReactiveControllerHost } from 'lit';
import { Anomaly } from '../json';
import { errorMessage } from '../../../elements-sk/modules/errorMessage';

export interface TriageBucket {
  name: string;
  anomalies: Anomaly[];
  triageState: 'NEW' | 'APPLIED' | 'DIRTY';
  actionType?: 'IGNORE' | 'EXISTING_BUG' | 'NEW_BUG';
  bugId?: number;
  lastAppliedIds?: string[];
}

const STORAGE_KEY = 'perf-user-triage-buckets-v3';

export function updateBucketDirtyState(bucket: TriageBucket): TriageBucket {
  if (bucket.triageState === 'NEW') {
    return bucket;
  }
  const lastApplied = new Set(bucket.lastAppliedIds || []);
  const hasUnapplied = bucket.anomalies.some((a) => !lastApplied.has(a.id));
  return {
    ...bucket,
    triageState: hasUnapplied ? 'DIRTY' : 'APPLIED',
  };
}

export class TriageBucketsController implements ReactiveController {
  private host: ReactiveControllerHost & EventTarget;

  buckets: TriageBucket[] = [];

  constructor(host: ReactiveControllerHost & EventTarget) {
    this.host = host;
    host.addController(this);
  }

  hostConnected() {
    this.loadBuckets();
  }

  private loadBuckets() {
    try {
      const saved = localStorage.getItem(STORAGE_KEY);
      if (saved) {
        const loaded: TriageBucket[] = JSON.parse(saved);
        this.buckets = loaded.map((b) => ({
          ...b,
          triageState: b.triageState || 'NEW',
          anomalies: b.anomalies || [],
        }));
        this.host.requestUpdate();
      } else {
        this.buckets = [];
        this.host.requestUpdate();
      }
    } catch (e) {
      console.error('Failed to load triage buckets from localStorage', e);
    }
  }

  private saveBuckets() {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(this.buckets));
    } catch (e) {
      console.error('Failed to save triage buckets to localStorage', e);
    }
    this.host.requestUpdate();
    this.emitBucketsChanged();
  }

  private emitBucketsChanged() {
    this.host.dispatchEvent(
      new CustomEvent('buckets-changed', {
        detail: { buckets: this.buckets },
        bubbles: true,
        composed: true,
      })
    );
  }

  addNewBucket(name: string): boolean {
    const trimmed = name.trim();
    if (!trimmed) return false;
    if (this.buckets.some((b) => b.name === trimmed)) {
      errorMessage(`Bucket "${trimmed}" already exists.`);
      return false;
    }
    this.buckets = [...this.buckets, { name: trimmed, anomalies: [], triageState: 'NEW' }];
    this.saveBuckets();
    return true;
  }

  updateBucket(updated: TriageBucket) {
    this.buckets = this.buckets.map((b) => (b.name === updated.name ? updated : b));
    this.saveBuckets();
  }

  deleteBucket(name: string) {
    this.buckets = this.buckets.filter((b) => b.name !== name);
    this.saveBuckets();
  }

  addToBucket(name: string, anomaliesToAdd: Anomaly[]) {
    this.buckets = this.buckets.map((b) => {
      if (b.name === name) {
        const existingIds = new Set(b.anomalies.map((a) => a.id));
        const newAnomalies = [...b.anomalies];
        anomaliesToAdd.forEach((a) => {
          if (!existingIds.has(a.id)) {
            newAnomalies.push(a);
          }
        });
        const updated = { ...b, anomalies: newAnomalies };
        return updateBucketDirtyState(updated);
      }
      return b;
    });
    this.saveBuckets();
  }

  getStagedAnomalyIds(): Set<string> {
    const ids = new Set<string>();
    this.buckets.forEach((b) => b.anomalies.forEach((a) => ids.add(a.id)));
    return ids;
  }

  copyAll(): string {
    const lines: string[] = [];
    for (const b of this.buckets) {
      lines.push(`[${b.name}]`);
      for (const a of b.anomalies) {
        lines.push(`[${a.start_revision}-${a.end_revision}]:${a.test_path}:${a.id}`);
      }
      lines.push('');
    }
    const text = lines.join('\n').trim();
    navigator.clipboard.writeText(text);
    return text;
  }

  async pasteAll(anomalies: Anomaly[]): Promise<boolean> {
    try {
      const text = await navigator.clipboard.readText();
      if (!text) return false;

      const lines = text.split(/\r?\n/);
      let currentBucketName = 'Imported Bucket';
      const parsedBuckets = new Map<string, string[]>();

      for (const line of lines) {
        const trimmed = line.trim();
        if (!trimmed) continue;

        if (trimmed.startsWith('[') && trimmed.endsWith(']')) {
          currentBucketName = trimmed.slice(1, -1).trim();
          if (!parsedBuckets.has(currentBucketName)) {
            parsedBuckets.set(currentBucketName, []);
          }
        } else {
          const parts = trimmed.split(':');
          const id = parts.pop()?.trim();
          if (id) {
            if (!parsedBuckets.has(currentBucketName)) {
              parsedBuckets.set(currentBucketName, []);
            }
            parsedBuckets.get(currentBucketName)!.push(id);
          }
        }
      }

      if (parsedBuckets.size === 0) return false;

      const newBuckets = [...this.buckets];

      parsedBuckets.forEach((pastedIds, bucketName) => {
        const matching = anomalies.filter((a) => pastedIds.includes(a.id));
        const existingIndex = newBuckets.findIndex((b) => b.name === bucketName);

        if (existingIndex >= 0) {
          const existingBucket = newBuckets[existingIndex];
          const existingIds = new Set(existingBucket.anomalies.map((a) => a.id));
          const mergedAnomalies = [...existingBucket.anomalies];
          matching.forEach((m) => {
            if (!existingIds.has(m.id)) {
              mergedAnomalies.push(m);
            }
          });
          newBuckets[existingIndex] = updateBucketDirtyState({
            ...existingBucket,
            anomalies: mergedAnomalies,
          });
        } else {
          newBuckets.push({
            name: bucketName,
            anomalies: matching,
            triageState: 'NEW',
          });
        }
      });

      this.buckets = newBuckets;
      this.saveBuckets();
      return true;
    } catch (_e) {
      errorMessage('Failed to read from clipboard. Please check browser clipboard permissions.');
      return false;
    }
  }
}
