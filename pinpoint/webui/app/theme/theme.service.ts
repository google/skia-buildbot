import { Injectable, inject, signal } from '@angular/core';
import { SettingsService, Theme } from '../settings/settings.service';

@Injectable({
  providedIn: 'root',
})
export class ThemeService {
  private settingsService = inject(SettingsService);

  isDarkMode = signal<boolean>(false);

  constructor() {
    this.initializeTheme();
  }

  private initializeTheme() {
    const isDark = this.settingsService.getTheme() === Theme.Dark;
    this.isDarkMode.set(isDark);
    this.applyTheme(isDark);
  }

  toggleTheme() {
    const newIsDark = !this.isDarkMode();
    this.isDarkMode.set(newIsDark);
    this.settingsService.setTheme(newIsDark ? Theme.Dark : Theme.Light);
    this.applyTheme(newIsDark);
  }

  private applyTheme(isDark: boolean) {
    const root = document.documentElement;
    if (isDark) {
      root.classList.add('dark-theme');
    } else {
      root.classList.remove('dark-theme');
    }
  }
}
