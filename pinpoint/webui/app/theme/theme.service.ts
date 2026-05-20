import { Injectable, signal } from '@angular/core';

@Injectable({
  providedIn: 'root',
})
export class ThemeService {
  isDarkMode = signal<boolean>(false);

  constructor() {
    this.initializeTheme();
  }

  private initializeTheme() {
    const isDark = localStorage.getItem('theme') === 'dark';
    this.isDarkMode.set(isDark);
    this.applyTheme(isDark);
  }

  toggleTheme() {
    const newIsDark = !this.isDarkMode();
    this.isDarkMode.set(newIsDark);
    localStorage.setItem('theme', newIsDark ? 'dark' : 'light');
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
