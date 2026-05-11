export class TraceDatabase {
  private dbName = 'TraceCache';

  private storeName = 'frames';

  private version = 1;

  private async openDB(): Promise<IDBDatabase> {
    return await new Promise((resolve, reject) => {
      const request = indexedDB.open(this.dbName, this.version);
      request.onerror = () => reject(request.error);
      request.onsuccess = () => resolve(request.result);
      request.onupgradeneeded = () => {
        const db = request.result;
        if (!db.objectStoreNames.contains(this.storeName)) {
          db.createObjectStore(this.storeName, { keyPath: 'id' });
        }
      };
    });
  }

  async get(id: string): Promise<any | null> {
    const db = await this.openDB();
    return await new Promise((resolve, reject) => {
      const transaction = db.transaction([this.storeName], 'readonly');
      const store = transaction.objectStore(this.storeName);
      const request = store.get(id);
      request.onerror = () => {
        db.close();
        reject(request.error);
      };
      request.onsuccess = () => {
        db.close();
        resolve(request.result ? request.result.data : null);
      };
    });
  }

  async set(id: string, data: any): Promise<void> {
    const db = await this.openDB();
    return await new Promise((resolve, reject) => {
      const transaction = db.transaction([this.storeName], 'readwrite');
      const store = transaction.objectStore(this.storeName);
      const request = store.put({ id, data, timestamp: Date.now() });
      request.onerror = () => {
        db.close();
        reject(request.error);
      };
      request.onsuccess = () => {
        db.close();
        resolve();
      };
    });
  }

  async evictOlderThan(days: number): Promise<void> {
    const db = await this.openDB();
    const cutoff = Date.now() - days * 24 * 3600 * 1000;
    return await new Promise((resolve, reject) => {
      const transaction = db.transaction([this.storeName], 'readwrite');
      const store = transaction.objectStore(this.storeName);
      const request = store.openCursor();
      request.onerror = () => {
        db.close();
        reject(request.error);
      };
      request.onsuccess = (e: any) => {
        const cursor = e.target.result;
        if (cursor) {
          if (cursor.value.timestamp < cutoff) {
            cursor.delete();
          }
          cursor.continue();
        } else {
          db.close();
          resolve();
        }
      };
    });
  }
}

export async function hashRequest(req: any): Promise<string> {
  const msgBuffer = new TextEncoder().encode(JSON.stringify(req));
  const hashBuffer = await crypto.subtle.digest('SHA-256', msgBuffer);
  const hashArray = Array.from(new Uint8Array(hashBuffer));
  return hashArray.map((b) => b.toString(16).padStart(2, '0')).join('');
}
