import { css } from 'lit';

export const style = css`
  :host {
    position: relative;
    display: inline-block;
    background-color: var(--md-sys-color-background, 'white');
  }

  md-outlined-button {
    --md-outlined-button-container-shape: 4px;
  }
`;
