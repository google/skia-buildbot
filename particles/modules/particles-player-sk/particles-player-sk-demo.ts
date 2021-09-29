import './index.ts';
import './particles-player-sk-demo.css';
import { $$ } from 'common-sk/modules/dom';
import { ParticlesPlayerSk } from './particles-player-sk';

const player = $$<ParticlesPlayerSk>('particles-player-sk')!;

const nouniforms = {
  width: 256,
  height: 256,
  body: {
    Bindings: [],
    Code: `
void spawn(inout Particle p) {
  p.lifetime = 2 + rand(p.seed);
  p.vel = p.dir * mix(50, 60, rand(p.seed));
}

void update(inout Particle p) {
  p.scale = 0.5 + 1.5 * p.age;
  float3 a0 = float3(0.098, 0.141, 0.784);
  float3 a1 = float3(0.525, 0.886, 0.980);
  float3 b0 = float3(0.376, 0.121, 0.705);
  float3 b1 = float3(0.933, 0.227, 0.953);
  p.color.rgb = mix(mix(a0, a1, p.age), mix(b0, b1, p.age), rand(p.seed));
}

`.split('\n'),
    Drawable: {
      Radius: 2,
      Type: 'SkCircleDrawable',
    },
    EffectCode: `
void effectSpawn(inout Effect effect) {
  effect.lifetime = 4;
  effect.rate = 120;
  effect.spin = 6;
}
`.split('\n'),
    MaxCount: 800,
  },
};

const uniforms = {
  width: 256,
  height: 256,
  body: {
    MaxCount: 800,
    Drawable: {
      Type: 'SkCircleDrawable',
      Radius: 2,
    },
    EffectCode: `
uniform float slider_rate;
uniform float slider_spin;

void effectSpawn(inout Effect effect) {
      effect.lifetime = 4;
}

void effectUpdate(inout Effect effect) {
      effect.rate = 100 * slider_rate;
      effect.spin = 10 * slider_spin;
}

`.split('\n'),
    Code: `
uniform float3 slider_color;

void spawn(inout Particle p) {
  p.lifetime = 2 + rand(p.seed);
  p.vel = p.dir * mix(50, 60, rand(p.seed));
}

void update(inout Particle p) {
  p.color.rgb = slider_color;
}
`.split('\n'),
    Bindings: [],
  },
};

player.initialize(uniforms).then(() => {
  player.pause();
  $$('#results')!.innerHTML = '<div id=loaded>Loaded.</div>';
});

$$('#pause')!.addEventListener('click', () => player.pause());
$$('#play')!.addEventListener('click', () => player.play());
$$('#reset')!.addEventListener('click', () => player.resetView());
$$('#restart')!.addEventListener('click', () => player.restartAnimation());

$$('#nouniforms')!.addEventListener('click', () => { player.initialize(nouniforms); });
$$('#uniforms')!.addEventListener('click', () => { player.initialize(uniforms); });
