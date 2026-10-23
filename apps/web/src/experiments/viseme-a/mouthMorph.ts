import mesh from './mouth-mesh.json'
import { sample } from './sample'

/** One tiny GPU patch shared by the portrait and detail views. No per-frame inference. */
export function createMouthMorph(
  base: HTMLImageElement,
  texture: HTMLImageElement,
  mask: HTMLImageElement,
  unavailable: () => void,
) {
  const canvas = document.createElement('canvas')
  canvas.width = mesh.width
  canvas.height = mesh.height
  const gl = canvas.getContext('webgl', {
    alpha: true,
    antialias: false,
    premultipliedAlpha: false,
    preserveDrawingBuffer: true,
    powerPreference: 'low-power',
  })
  if (!gl) return null
  const shaders: WebGLShader[] = []
  const textures: WebGLTexture[] = []
  const buffers: WebGLBuffer[] = []
  let program: WebGLProgram | null = null
  function onLoss(event: Event) {
    event.preventDefault()
    unavailable()
  }
  canvas.addEventListener('webglcontextlost', onLoss)
  function dispose() {
    canvas.removeEventListener('webglcontextlost', onLoss)
    shaders.forEach((shader) => gl!.deleteShader(shader))
    textures.forEach((texture) => gl!.deleteTexture(texture))
    buffers.forEach((buffer) => gl!.deleteBuffer(buffer))
    if (program) gl!.deleteProgram(program)
  }
  function compile(type: number, source: string) {
    const shader = gl!.createShader(type)
    if (!shader) throw new Error('Shader allocation failed')
    shaders.push(shader)
    gl!.shaderSource(shader, source)
    gl!.compileShader(shader)
    if (!gl!.getShaderParameter(shader, gl!.COMPILE_STATUS))
      throw new Error('Shader compilation failed')
    return shader
  }
  try {
    const vertex = compile(
      gl.VERTEX_SHADER,
      `
      attribute vec2 aRest;
      attribute vec2 aOpen;
      attribute float aCavity;
      uniform vec2 uSize;
      uniform float uAmount;
      varying vec2 vRest;
      varying vec2 vOpen;
      varying float vCavity;
      void main() {
        vec2 position = mix(aRest, aOpen, uAmount) / uSize;
        gl_Position = vec4(position.x * 2.0 - 1.0, 1.0 - position.y * 2.0, 0.0, 1.0);
        vRest = aRest / uSize;
        vOpen = aOpen / uSize;
        vCavity = aCavity;
      }`,
    )
    const precision = gl.getShaderPrecisionFormat(gl.FRAGMENT_SHADER, gl.HIGH_FLOAT)?.precision
      ? 'highp'
      : 'mediump'
    const fragment = compile(
      gl.FRAGMENT_SHADER,
      `
      precision ${precision} float;
      uniform sampler2D uRest;
      uniform sampler2D uOpen;
      uniform sampler2D uMask;
      uniform vec2 uSize;
      uniform float uAmount;
      varying vec2 vRest;
      varying vec2 vOpen;
      varying float vCavity;
      uniform vec2 uInner[20];
      float edgeDistance(vec2 point, vec2 start, vec2 end) {
        vec2 edge = end - start;
        float along = clamp(dot(point - start, edge) / max(dot(edge, edge), 0.0001), 0.0, 1.0);
        return length(point - start - along * edge);
      }
      void main() {
        vec2 maskUv = vec2(gl_FragCoord.x, uSize.y - gl_FragCoord.y) / uSize;
        vec3 color = mix(texture2D(uRest, vRest).rgb, texture2D(uOpen, vOpen).rgb, uAmount);
        // Teeth are attached to the reference, not to the deforming lip mesh.
        // The moving aperture reveals a fixed interior instead of stretching it
        // or blending it with the closed-mouth lip texture.
        if (vCavity > 0.5) {
          vec2 point = maskUv * uSize;
          float distance = edgeDistance(point, uInner[19], uInner[0]);
          for (int i = 0; i < 19; i++) distance = min(distance, edgeDistance(point, uInner[i], uInner[i + 1]));
          color = mix(color, texture2D(uOpen, maskUv).rgb, smoothstep(0.0, 0.8, distance));
        }
        gl_FragColor = vec4(color, texture2D(uMask, maskUv).a);
      }`,
    )
    program = gl.createProgram()
    if (!program) throw new Error('Program allocation failed')
    gl.attachShader(program, vertex)
    gl.attachShader(program, fragment)
    gl.linkProgram(program)
    if (!gl.getProgramParameter(program, gl.LINK_STATUS)) throw new Error('Program linking failed')
    gl.useProgram(program)
    gl.viewport(0, 0, mesh.width, mesh.height)
    gl.disable(gl.BLEND)
    gl.disable(gl.DITHER)
    gl.uniform2f(gl.getUniformLocation(program, 'uSize'), mesh.width, mesh.height)
    const amountLocation = gl.getUniformLocation(program, 'uAmount')
    const innerLocation = gl.getUniformLocation(program, 'uInner[0]')
    const inner = new Float32Array(40)
    const cavity = new Set(mesh.cavity_triangles)
    const data = new Float32Array(
      mesh.triangles.flatMap((triangle, triangleIndex) =>
        triangle.flatMap((i) => [
          ...mesh.rest[i],
          ...mesh.open[i],
          cavity.has(triangleIndex) ? 1 : 0,
        ]),
      ),
    )
    const buffer = gl.createBuffer()
    if (!buffer) throw new Error('Buffer allocation failed')
    buffers.push(buffer)
    gl.bindBuffer(gl.ARRAY_BUFFER, buffer)
    gl.bufferData(gl.ARRAY_BUFFER, data, gl.STATIC_DRAW)
    for (const [name, offset] of [
      ['aRest', 0],
      ['aOpen', 8],
    ] as const) {
      const attribute = gl.getAttribLocation(program, name)
      gl.enableVertexAttribArray(attribute)
      gl.vertexAttribPointer(attribute, 2, gl.FLOAT, false, 20, offset)
    }
    const cavityAttribute = gl.getAttribLocation(program, 'aCavity')
    gl.enableVertexAttribArray(cavityAttribute)
    gl.vertexAttribPointer(cavityAttribute, 1, gl.FLOAT, false, 20, 16)
    const rest = document.createElement('canvas')
    rest.width = mesh.width
    rest.height = mesh.height
    const context = rest.getContext('2d')
    if (!context) throw new Error('Canvas unavailable')
    context.drawImage(base, -sample.mouth.x, -sample.mouth.y)
    for (const [unit, image, name] of [
      [0, rest, 'uRest'],
      [1, texture, 'uOpen'],
      [2, mask, 'uMask'],
    ] as const) {
      const texture = gl.createTexture()
      if (!texture) throw new Error('Texture allocation failed')
      textures.push(texture)
      gl.activeTexture(gl.TEXTURE0 + unit)
      gl.bindTexture(gl.TEXTURE_2D, texture)
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, unit === 2 ? gl.NEAREST : gl.LINEAR)
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, unit === 2 ? gl.NEAREST : gl.LINEAR)
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
      gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, gl.RGBA, gl.UNSIGNED_BYTE, image)
      gl.uniform1i(gl.getUniformLocation(program, name), unit)
    }
    return {
      canvas,
      draw(amount: number) {
        if (gl.isContextLost()) return
        gl.uniform1f(amountLocation, amount)
        for (let i = 0; i < 20; i++)
          for (let axis = 0; axis < 2; axis++) {
            inner[i * 2 + axis] =
              mesh.rest[20 + i][axis] * (1 - amount) + mesh.open[20 + i][axis] * amount
          }
        gl.uniform2fv(innerLocation, inner)
        gl.clearColor(0, 0, 0, 0)
        gl.clear(gl.COLOR_BUFFER_BIT)
        gl.drawArrays(gl.TRIANGLES, 0, mesh.triangles.length * 3)
      },
      dispose,
    }
  } catch {
    dispose()
    return null
  }
}
