import { Component, OnInit, inject, signal } from '@angular/core';
import { MatToolbarModule } from '@angular/material/toolbar';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatMenuModule } from '@angular/material/menu';
import { RouterLink } from '@angular/router';
import { GatewayService } from '../gateway/gateway.service';

@Component({
  selector: 'app-header',
  standalone: true,
  imports: [MatToolbarModule, MatButtonModule, MatIconModule, MatMenuModule, RouterLink],
  templateUrl: './header.component.html',
})
export class HeaderComponent implements OnInit {
  private gatewayService = inject(GatewayService);

  userEmail = signal<string>('Loading...');

  ngOnInit() {
    this.loadUserInfo();
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
    alert('Help button clicked');
  }

  onBugClick() {
    alert('File a bug button clicked');
  }

  onSignOutClick() {
    alert('Not implemented: Sign out');
  }
}
