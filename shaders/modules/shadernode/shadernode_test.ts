/* eslint-disable dot-notation */
import './index';
import { assert } from 'chai';
import { numPredefinedShaderUniforms, numPredefinedUniforms, ShaderNode } from './index';
import { CanvasKit } from '../../build/canvaskit/canvaskit';

// eslint-disable-next-line @typescript-eslint/no-var-requires
const CanvasKitInit = require('../../build/canvaskit/canvaskit.js');


let canvasKit: CanvasKit | null = null;

const getCanvasKit = async (): Promise<CanvasKit> => {
  if (canvasKit) {
    return canvasKit;
  }
  canvasKit = await CanvasKitInit({ locateFile: (file: string) => `https://particles.skia.org/dist/${file}` });
  return canvasKit!;
};

describe('ShaderNode', async () => {
  it('constructor throws when not passed in the correct number of image shaders', async () => {
    const ck = await getCanvasKit();
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    assert.throws(() => { const node = new ShaderNode(ck, []); });
  });

  it('constructor builds with a default shader', async () => {
    const ck = await getCanvasKit();
    const node = new ShaderNode(ck, new Array(2).fill(null));
    node.compile();

    // Confirm that all the post-compile pre-calculations are done correctly.
    assert.equal(node.getUniformCount(), numPredefinedUniforms, 'The default shader doesn\'t have any user uniforms.');
    assert.equal(node.getUniform(0).name, 'iResolution', 'Confirm the predefined shaders show up in the uniforms.');
    assert.equal(node.getUniformFloatCount(), node.numPredefinedUniformValues, 'These are equal because the default shader has 0 user uniforms.');
    assert.isNotNull(node['uniformsMallocObj'], "We Malloc'd");
    assert.equal(node.numPredefinedUniformValues, 11, 'The number of predefined uniform values is calculated after compile() is called. This value will change if predefinedUniforms is changed.');
    assert.deepEqual(node.errorLineNumbers, []);
    assert.equal(node.compileErrorMessage, '');
  });

  it('correctly indicates when run() needs to be called.', async () => {
    const ck = await getCanvasKit();
    const node = new ShaderNode(ck, new Array(2).fill(null));
    node.compile();

    assert.isFalse(node.needsRun());

    const originalCode = node.shaderCode;
    node.shaderCode += '\n';
    assert.isTrue(node.needsRun());
    node.shaderCode = originalCode;
    assert.isFalse(node.needsRun());
  });

  it('correctly reports user uniforms', async () => {
    const ck = await getCanvasKit();
    const node = new ShaderNode(ck, new Array(2).fill(null));
    node.compile();

    // Set code that has a user uniform, in this case with 4 floats, because
    // saving is not only indicated when the code changes, but when the user
    // uniforms change.
    node.setScrap({
      Type: 'sksl',
      Body: `uniform float4 iColorWithAlpha;

      half4 main(float2 fragCoord) {
        return half4(iColorWithAlpha);
      }
      `,
      SKSLMetaData: {
        Children: [],
        Uniforms: [1, 0, 1, 0],
      },
    });
    node.compile();

    assert.equal(node.getUniformCount() - numPredefinedUniforms, 1, 'There is just one user uniform.');
    assert.equal(node.getUniform(numPredefinedUniforms).name, 'iColorWithAlpha', 'Confirm the uniform name is extracted correctly.');
    assert.equal(node.getUniformFloatCount() - node.numPredefinedUniformValues, 4, 'The user uniform has 4 floats.');
  });

  it('correctly indicates when save() needs to be called.', async () => {
    const ck = await getCanvasKit();
    const node = new ShaderNode(ck, new Array(2).fill(null));
    node.compile();

    const startingUniformValues = [1, 0, 1, 0];
    const modifiedUniformValues = [1, 1, 1, 1];

    // Set code that has a user uniform, in this case with 4 floats, because
    // saving is not only indicated when the code changes, but when the user
    // uniforms change.
    node.setScrap({
      Type: 'sksl',
      Body: `uniform float4 iColorWithAlpha;

      half4 main(float2 fragCoord) {
        return half4(iColorWithAlpha);
      }
      `,
      SKSLMetaData: {
        Children: [],
        Uniforms: startingUniformValues,
      },
    });
    node.compile();

    // Changing the code means we need to save.
    const originalCode = node.shaderCode;
    assert.isFalse(node.needsSave());
    node.shaderCode += '\n';
    assert.isTrue(node.needsSave(), 'Needs save if code changed.');
    node.shaderCode = originalCode;
    assert.isFalse(node.needsSave(), "Doesn't need save when code restored.");

    // Also changing the user uniform values means we need to save.
    node.currentUserUniformValues = modifiedUniformValues;
    assert.isTrue(node.needsSave(), 'Needs save if uniform values changed.');
    node.currentUserUniformValues = startingUniformValues;
    assert.isFalse(node.needsSave(), "Doesn't need save if uniform values restored.");
  });

  it('reports compiler errors', async () => {
    const ck = await getCanvasKit();
    const node = new ShaderNode(ck, new Array(2).fill(null));
    node.compile();

    // Set code that has a user uniform, in this case with 4 floats, because
    // saving is not only indicated when the code changes, but when the user
    // uniforms change.
    node.setScrap({
      Type: 'sksl',
      Body: `uniform float4 iColorWithAlpha;

      half4 main(float2 fragCoord) {
        return half4(iColorWithAlpha) // Missing trailing semicolon.
      }
      `,
      SKSLMetaData: {
        Children: [],
        Uniforms: [1, 0, 1, 0],
      },
    });
    node.compile();

    assert.deepEqual(node.errorLineNumbers, [4]);
    node.compileErrorMessage.startsWith('error: 4:');
  });
});
