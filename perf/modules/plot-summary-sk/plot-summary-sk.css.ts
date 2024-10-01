import { css } from 'lit';

export const style = css`
  :host {
    display: flex;
    position: relative;
    width: 100%;
    height: 100px;
    --sk-summary-highlight: var(--md-sys-color-primary-container, #ced0ce);
  }
  .load-btn {
    width: 20px;
    height: 85%;
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
  .loader {
    width: 20px;
    aspect-ratio: 1;
    --c: no-repeat linear-gradient(#000 0 0);
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
