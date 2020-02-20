export const spiral = {
  "Bindings": [],
  "Code": [
    "uniform float slider_speed;",
    "void spawn(inout Particle p) {",
    "  p.lifetime = 2 + rand;",
    "  p.vel = p.dir * mix(50, 60, rand) * mix(0.5, 4, slider_speed);",
    "}",
    "",
    "void update(inout Particle p) {",
    "  p.scale = 0.5 + 1.5 * p.age;",
    "  float3 a0 = float3(0.098, 0.141, 0.784);",
    "  float3 a1 = float3(0.525, 0.886, 0.980);",
    "  float3 b0 = float3(0.376, 0.121, 0.705);",
    "  float3 b1 = float3(0.933, 0.227, 0.953);",
    "  p.color.rgb = mix(mix(a0, a1, p.age), mix(b0, b1, p.age), rand);",
    "}"
],
  "Drawable": {
    "Radius": 2,
    "Type": "SkCircleDrawable"
},
  "EffectCode": [
    "void effectSpawn(inout Effect effect) {",
    "  effect.lifetime = 4;",
    "  effect.rate = 120;",
    "  effect.spin = 6;",
    "}",
    "",
    "void effectUpdate(inout Effect effect) {",
    "}",
    ""
  ],
  "MaxCount": 800
};