/**
 * @module modules/common/keyboard-shortcuts
 * @description Shared keyboard shortcut logic for Skia Perf.
 */

export const SHORTCUTS = [
  { key: 'p', method: 'onTriagePositive', description: 'Mark as positive.', category: 'Triage' },
  { key: 'n', method: 'onTriageNegative', description: 'Mark as negative.', category: 'Triage' },
  { key: 'e', method: 'onTriageExisting', description: 'Mark as existing.', category: 'Triage' },
  { key: 'w', method: 'onZoomIn', description: 'Zoom in.', category: 'Navigation' },
  { key: ',', method: 'onZoomIn', description: 'Zoom in.', category: 'Navigation' },
  { key: 's', method: 'onZoomOut', description: 'Zoom out.', category: 'Navigation' },
  { key: 'o', method: 'onZoomOut', description: 'Zoom out.', category: 'Navigation' },
  { key: 'a', method: 'onPanLeft', description: 'Pan left.', category: 'Navigation' },
  { key: 'd', method: 'onPanRight', description: 'Pan right.', category: 'Navigation' },
  { key: 'g', method: 'onOpenReport', description: 'Open report.', category: 'Report' },
  { key: 'G', method: 'onOpenGroupReport', description: 'Open group report.', category: 'Report' },
  { key: '?', method: 'onOpenHelp', description: 'Show help.', category: 'General' },
] as const;

export interface KeyboardShortcutHandler {
  // Triage Actions
  onTriagePositive?(): void;
  onTriageNegative?(): void;
  onTriageExisting?(): void;

  // Navigation Actions
  onZoomIn?(): void;
  onZoomOut?(): void;
  onPanLeft?(): void;
  onPanRight?(): void;

  // Report Actions
  onOpenReport?(): void;
  onOpenGroupReport?(): void;

  // Help
  onOpenHelp?(): void;
}

export interface Shortcut {
  key: string;
  action: string;
  description: string;
  method?: string;
  callback?: () => void;
}

export class ShortcutRegistry {
  private static instance: ShortcutRegistry;

  private shortcuts: Map<string, Shortcut[]> = new Map();

  private constructor() {
    this.populateFromShortcuts();
  }

  static getInstance(): ShortcutRegistry {
    if (!ShortcutRegistry.instance) {
      ShortcutRegistry.instance = new ShortcutRegistry();
    }
    return ShortcutRegistry.instance;
  }

  private populateFromShortcuts(): void {
    SHORTCUTS.forEach((s) => {
      const category = s.category;
      if (!this.shortcuts.has(category)) {
        this.shortcuts.set(category, []);
      }
      this.shortcuts.get(category)!.push({
        key: s.key,
        action: s.description,
        description: s.description,
        method: s.method,
      });
    });
  }

  register(category: string, shortcuts: Shortcut[]): void {
    this.shortcuts.set(category, shortcuts);
  }

  getShortcuts(): Map<string, Shortcut[]> {
    return this.shortcuts;
  }

  reset(): void {
    this.shortcuts.clear();
    this.populateFromShortcuts();
  }
}

/**
 * Handles keyboard shortcuts by mapping keys to handler methods.
 * @param e The keyboard event.
 * @param handler The handler object implementing KeyboardShortcutHandler.
 */
export function handleKeyboardShortcut(e: KeyboardEvent, handler: KeyboardShortcutHandler): void {
  // Ignore if typing in input or textarea
  if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement) {
    return;
  }

  const shortcut = SHORTCUTS.find((s) => s.key === e.key);
  if (shortcut && typeof handler[shortcut.method as keyof KeyboardShortcutHandler] === 'function') {
    (handler[shortcut.method as keyof KeyboardShortcutHandler] as () => void)();
  }
}
