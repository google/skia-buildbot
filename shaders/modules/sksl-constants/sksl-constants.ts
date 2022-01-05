// CodeMirror likes mode definitions as maps to bools, but a string of space
// separated words is easier to edit, so we convert our strings into a map here.
function words(str: string) {
  const obj: Record<string, boolean> = {};
  str.split(' ').forEach((word: string) => { obj[word] = true; });
  return obj;
}

// See the design doc for the list of keywords. http://go/shaders.skia.org
export const keywords = words(
    'const attribute uniform varying break continue '
  + 'discard return for while do if else struct in out inout uniform layout');

export const blockKeywords = words('case do else for if switch while struct enum union');

export const defKeywords = words('struct enum union');

export const atoms = words('sk_FragCoord true false');

export const builtins = words(
    'radians degrees '
  + 'sin cos tan asin acos atan '
  + 'pow exp log exp2 log2 '
  + 'sqrt inversesqrt '
  + 'abs sign floor ceil fract mod '
  + 'min max clamp saturate '
  + 'mix step smoothstep '
  + 'length distance dot cross normalize '
  + 'faceforward reflect refract '
  + 'matrixCompMult inverse '
  + 'lessThan lessThanEqual greaterThan greaterThanEqual equal notEqual '
  + 'any all not '
  + 'sample unpremul');

export const types = words(
    'int long char short double float unsigned '
  + 'signed void bool float float2 float3 float4 '
  + 'float2x2 float3x3 float4x4 '
  + 'half half2 half3 half4 '
  + 'half2x2 half3x3 half4x4 '
  + 'bool bool2 bool3 bool4 '
  + 'int int2 int3 int4 '
  + 'fragmentProcessor shader '
  + 'vec2 vec3 vec4 '
  + 'ivec2 ivec3 ivec4 '
  + 'bvec2 bvec3 bvec4 '
  + 'mat2 mat3 mat4');
