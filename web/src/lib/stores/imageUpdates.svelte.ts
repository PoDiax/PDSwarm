import { imageApi } from '../api';
import type { ImageUpdateStatus } from '../types';

export const IMAGE_UPDATE_INTERVALS = [
  { label: 'Disabled (Manual check only)', value: 0 },
  { label: 'Every 1 hour', value: 1 * 60 * 60 * 1000 },
  { label: 'Every 6 hours (Recommended)', value: 6 * 60 * 60 * 1000 },
  { label: 'Every 12 hours', value: 12 * 60 * 60 * 1000 },
  { label: 'Every 24 hours (Daily)', value: 24 * 60 * 60 * 1000 },
] as const;

class ImageUpdateStore {
  updates = $state<Record<string, ImageUpdateStatus>>({});
  checking = $state<Record<string, boolean>>({});
  lastCheckedTimestamp = $state<number>(0);
  checkIntervalMs = $state<number>(6 * 60 * 60 * 1000); // Default: 6 hours

  isCheckingAny = $derived(Object.values(this.checking).some(Boolean));
  totalUpdatesAvailable = $derived(
    Object.values(this.updates).filter((u) => u.status === 'update_available').length
  );

  constructor() {
    this.loadFromStorage();
  }

  private loadFromStorage() {
    if (typeof window === 'undefined') return;
    try {
      const storedInterval = localStorage.getItem('pdswarm_image_update_interval');
      if (storedInterval !== null) {
        const parsed = parseInt(storedInterval, 10);
        if (!isNaN(parsed)) {
          this.checkIntervalMs = parsed;
        }
      }

      const raw = localStorage.getItem('pdswarm_image_updates');
      if (raw) {
        const parsed = JSON.parse(raw);
        const timestamp = parsed.timestamp || 0;
        this.lastCheckedTimestamp = timestamp;

        // Determine cache TTL: match configured auto-check interval, or minimum 30 minutes
        const ttl = this.checkIntervalMs > 0 ? this.checkIntervalMs : 30 * 60 * 1000;
        if (Date.now() - timestamp < ttl) {
          this.updates = parsed.data || {};
        }
      }
    } catch {
      // ignore
    }
  }

  private saveToStorage() {
    if (typeof window === 'undefined') return;
    try {
      localStorage.setItem(
        'pdswarm_image_updates',
        JSON.stringify({
          timestamp: this.lastCheckedTimestamp || Date.now(),
          data: this.updates,
        })
      );
    } catch {
      // ignore
    }
  }

  setInterval(ms: number) {
    this.checkIntervalMs = ms;
    if (typeof window !== 'undefined') {
      localStorage.setItem('pdswarm_image_update_interval', String(ms));
    }
  }

  clearCache() {
    this.updates = {};
    this.lastCheckedTimestamp = 0;
    if (typeof window !== 'undefined') {
      localStorage.removeItem('pdswarm_image_updates');
    }
  }

  getStatus(imageRef?: string): ImageUpdateStatus | undefined {
    if (!imageRef) return undefined;
    return this.updates[imageRef];
  }

  isChecking(imageRef?: string): boolean {
    if (!imageRef) return false;
    return !!this.checking[imageRef];
  }

  hasUpdate(imageRef?: string): boolean {
    if (!imageRef) return false;
    const s = this.getStatus(imageRef);
    return s?.status === 'update_available';
  }

  isUpToDate(imageRef?: string): boolean {
    if (!imageRef) return false;
    const s = this.getStatus(imageRef);
    return s?.status === 'up_to_date';
  }

  hasAnyUpdate(imageRefs: string[]): boolean {
    return imageRefs.some((r) => this.hasUpdate(r));
  }

  isAnyChecking(imageRefs: string[]): boolean {
    return imageRefs.some((r) => this.isChecking(r));
  }

  async checkUpdate(imageRef: string, force = false): Promise<ImageUpdateStatus | null> {
    const trimmed = (imageRef || '').trim();
    if (!trimmed || trimmed.startsWith('<none>')) {
      return null;
    }

    if (!force && this.updates[trimmed]) {
      return this.updates[trimmed];
    }

    if (this.checking[trimmed]) {
      return this.updates[trimmed] || null;
    }

    this.checking = { ...this.checking, [trimmed]: true };
    try {
      const res = await imageApi.checkUpdate(trimmed);
      this.updates = { ...this.updates, [trimmed]: res };
      this.lastCheckedTimestamp = Date.now();
      this.saveToStorage();
      return res;
    } catch (err) {
      console.warn(`[ImageUpdateStore] Failed to check update for ${trimmed}:`, err);
      return null;
    } finally {
      const next = { ...this.checking };
      delete next[trimmed];
      this.checking = next;
    }
  }

  /**
   * Fast concurrent batch check for multiple images.
   * If force=false, only checks images that are missing or whose cache has expired.
   */
  async checkMultiple(imageRefs: string[], force = false): Promise<void> {
    const unique = [
      ...new Set(
        imageRefs
          .map((r) => (r || '').trim())
          .filter((r) => r && !r.startsWith('<none>'))
      ),
    ];
    if (unique.length === 0) return;

    const now = Date.now();
    const ttl = this.checkIntervalMs > 0 ? this.checkIntervalMs : 60 * 60 * 1000;
    const needsCheck = force
      ? unique
      : unique.filter(
          (r) =>
            !this.updates[r] ||
            this.lastCheckedTimestamp === 0 ||
            now - this.lastCheckedTimestamp > ttl
        );

    if (needsCheck.length === 0) {
      return;
    }

    // Set checking state for all images being fetched
    const newChecking = { ...this.checking };
    for (const r of needsCheck) {
      newChecking[r] = true;
    }
    this.checking = newChecking;

    try {
      // Send in fast batch requests of up to 30 images at once to the backend
      for (let i = 0; i < needsCheck.length; i += 30) {
        const chunk = needsCheck.slice(i, i + 30);
        const res = await imageApi.checkBatch(chunk);
        if (res && res.results) {
          this.updates = { ...this.updates, ...res.results };
        }
      }
      this.lastCheckedTimestamp = Date.now();
      this.saveToStorage();
    } catch (err) {
      console.warn('[ImageUpdateStore] Batch update check failed, falling back to individual checks:', err);
      // Fallback
      for (const r of needsCheck) {
        try {
          const res = await imageApi.checkUpdate(r);
          this.updates = { ...this.updates, [r]: res };
        } catch {
          // ignore
        }
      }
      this.lastCheckedTimestamp = Date.now();
      this.saveToStorage();
    } finally {
      const next = { ...this.checking };
      for (const r of needsCheck) {
        delete next[r];
      }
      this.checking = next;
    }
  }

  /**
   * Automatically runs in the background if the configured interval has elapsed
   * or if images in the current view have not yet been evaluated.
   */
  async checkIfNeeded(imageRefs: string[]): Promise<void> {
    if (this.checkIntervalMs <= 0) return; // Automatic checks are disabled

    const now = Date.now();
    const isIntervalElapsed =
      this.lastCheckedTimestamp === 0 ||
      now - this.lastCheckedTimestamp >= this.checkIntervalMs;

    const hasUnchecked = imageRefs.some(
      (r) => r && !r.startsWith('<none>') && !this.updates[r.trim()]
    );

    if (isIntervalElapsed || hasUnchecked) {
      await this.checkMultiple(imageRefs, false);
    }
  }
}

export const imageUpdateStore = new ImageUpdateStore();
