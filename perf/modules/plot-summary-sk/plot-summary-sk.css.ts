import { css } from 'lit';

export const style = css`
  :host {
    --md-elevation-level: 1;
    --md-outlined-icon-button-container-shape: 6px;
    --md-outlined-icon-button-icon-size: 24px;
    --sk-summary-highlight: var(--md-sys-color-primary-container, #ced0ce);
    display: flex;
    position: relative;
    width: 100%;
    height: 120px;

    --md-icon-button-state-layer-shape: 6px;
    --md-icon-button-icon-size: 24px;
    --md-icon-button-icon-color: var(--md-sys-color-on-surface-variant);

    align-items: center;
    gap: 6px;
  }
  .load-btn {
    width: 20px;
    height: 74px;
  }
  .overlay {
    position: absolute;
    top: 0;
    left: 0;
    width: 100%;
    height: 100%;
  }
  .plot {
    top: 0;
    left: 0;
    width: 100%;
    height: 100%;
  }
  .container {
    position: relative;
    width: 100%;
    height: 100%;
  }
  h-resizable-box-sk {
    position: absolute;
    top: 22px;
    height: 80px;
    width: 100%;
  }
  .loader {
    width: 20px;
    aspect-ratio: 1;
    --c: no-repeat linear-gradient(var(--md-sys-color-on-surface, black) 0 0);
    background:
      var(--c) 0% 50%,
      var(--c) 50% 50%,
      var(--c) 100% 50%;
    background-size: 20% 100%;
    animation: l1 1s infinite linear;
  }
  @keyframes l1 {
    0% {
      background-size:
        20% 100%,
        20% 100%,
        20% 100%;
    }
    33% {
      background-size:
        20% 10%,
        20% 100%,
        20% 100%;
    }
    50% {
      background-size:
        20% 100%,
        20% 10%,
        20% 100%;
    }
    66% {
      background-size:
        20% 100%,
        20% 100%,
        20% 10%;
    }
    100% {
      background-size:
        20% 100%,
        20% 100%,
        20% 100%;
    }
  }
`;
