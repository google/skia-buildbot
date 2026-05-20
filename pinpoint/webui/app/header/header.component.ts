import { Component, OnInit, inject, signal } from '@angular/core';
import { MatToolbarModule } from '@angular/material/toolbar';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatMenuModule } from '@angular/material/menu';
import { MatTooltipModule } from '@angular/material/tooltip';
import { RouterLink } from '@angular/router';
import { GatewayService } from '../gateway/gateway.service';
import { ThemeService } from '../theme/theme.service';

// TODO(b/514573802): Update the link to show the new Pinpoint documentation.
export const DOCUMENTATION_URL =
  'https://chromium.googlesource.com/catapult/+/HEAD/dashboard/dashboard/pinpoint/README.md';
export const BUG_REPORT_URL =
  'https://b.corp.google.com/issues/new?component=1970595&template=2325183';

@Component({
  selector: 'app-header',
  standalone: true,
  imports: [
    MatToolbarModule,
    MatButtonModule,
    MatIconModule,
    MatMenuModule,
    RouterLink,
    MatTooltipModule,
  ],
  templateUrl: './header.component.html',
})
export class HeaderComponent implements OnInit {
  private gatewayService = inject(GatewayService);

  private themeService = inject(ThemeService);

  userEmail = signal<string>('Loading...');

  isDarkMode = this.themeService.isDarkMode;

  ngOnInit() {
    this.loadUserInfo();
  }

  toggleTheme() {
    this.themeService.toggleTheme();
  }

  private async loadUserInfo() {
    try {
      const res = await this.gatewayService.GetUserInfo({});
      this.userEmail.set(res.email || 'Unknown user');
    } catch (err) {
      console.error('Failed to load user info:', err);
      this.userEmail.set('Error loading user');
    }
  }

  onHelpClick() {
    window.open(DOCUMENTATION_URL, '_blank');
  }

  onBugClick() {
    window.open(BUG_REPORT_URL, '_blank');
  }

  onSignOutClick() {
    const redirectUrl = encodeURIComponent(window.location.origin);
    this.redirect(`/logout/?redirect=${redirectUrl}`);
  }

  redirect(url: string) {
    window.location.href = url;
  }
}
