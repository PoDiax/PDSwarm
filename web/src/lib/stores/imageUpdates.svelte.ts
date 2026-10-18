import { imageApi } from '../api';
import type { ImageUpdateStatus } from '../types';

class ImageUpdateStore {
  updates = $state<Record<string, ImageUpdateStatus>>({});
  checking = $state<Record<string, boolean>>({});

  isCheckingAny = $derived(Object.values(this.checking).some(Boolean));

  constructor() {
    this.loadFromStorage();
  }

  private loadFromStorage() {
    if (typeof window === 'undefined') return;
    try {
      const raw = localStorage.getItem('pdswarm_image_updates');
      if (raw) {
        const parsed = JSON.parse(raw);
        // Expire cache after 15 minutes
        if (parsed.timestamp && Date.now() - parsed.timestamp < 15 * 60 * 1000) {
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
          timestamp: Date.now(),
          data: this.updates,
        })
      );
    } catch {
      // ignore
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

  async checkUpdate(imageRef: string): Promise<ImageUpdateStatus | null> {
    const trimmed = (imageRef || '').trim();
    if (!trimmed || trimmed.startsWith('<none>') || this.checking[trimmed]) {
      return this.updates[trimmed] || null;
    }

    this.checking = { ...this.checking, [trimmed]: true };
    try {
      const res = await imageApi.checkUpdate(trimmed);
      this.updates = { ...this.updates, [trimmed]: res };
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

  async checkMultiple(imageRefs: string[]): Promise<void> {
    const unique = [
      ...new Set(
        imageRefs
          .map((r) => (r || '').trim())
          .filter((r) => r && !r.startsWith('<none>'))
      ),
    ];

    // Batch checks in parallel groups of 3
    for (let i = 0; i < unique.length; i += 3) {
      const batch = unique.slice(i, i + 3);
      await Promise.all(batch.map((ref) => this.checkUpdate(ref)));
    }
  }
}

export const imageUpdateStore = new ImageUpdateStore();
