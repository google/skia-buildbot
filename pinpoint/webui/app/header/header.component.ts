import { Component } from '@angular/core';
import { MatToolbarModule } from '@angular/material/toolbar';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

@Component({
  selector: 'app-header',
  standalone: true,
  imports: [MatToolbarModule, MatButtonModule, MatIconModule],
  templateUrl: './header.component.html',
})
export class HeaderComponent {
  onHelpClick() {
    alert('Help button clicked');
  }

  onBugClick() {
    alert('File a bug button clicked');
  }

  onAvatarClick() {
    alert('User avatar clicked');
  }
}
