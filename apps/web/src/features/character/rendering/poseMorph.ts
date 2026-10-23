import type { PoseImages, PoseMesh } from '../types/oralTypes'
import { dentalBounds, drawDentalRow } from './dentalOcclusion'

/** Warp lips and pose-specific soft tissue; composite rigid reference teeth last. */
export function createPoseMorph(
  from: PoseImages,
  to: PoseImages,
  mesh: PoseMesh,
  upper: HTMLImageElement,
  lower: HTMLImageElement,
  unavailable: () => void,
) {
  const gpu = document.createElement('canvas')
  gpu.width = mesh.width
  gpu.height = mesh.height
  const canvas = document.createElement('canvas')
  canvas.width = mesh.width
  canvas.height = mesh.height
  const dental = document.createElement('canvas')
  dental.width = mesh.width
  dental.height = mesh.height
  const context = canvas.getContext('2d'),
    teeth = dental.getContext('2d')
  const gl = gpu.getContext('webgl', {
    alpha: true,
    antialias: false,
    premultipliedAlpha: false,
    preserveDrawingBuffer: true,
    powerPreference: 'low-power',
  })
  if (!gl || !context || !teeth) return null
  const upperBounds = dentalBounds(upper),
    lowerBounds = dentalBounds(lower)
  const shaders: WebGLShader[] = [],
    textures: WebGLTexture[] = [],
    buffers: WebGLBuffer[] = []
  let program: WebGLProgram | null = null
  function onLoss(event: Event) {
    event.preventDefault()
    unavailable()
  }
  gpu.addEventListener('webglcontextlost', onLoss)
  function dispose() {
    gpu.removeEventListener('webglcontextlost', onLoss)
    shaders.forEach((s) => gl!.deleteShader(s))
    textures.forEach((t) => gl!.deleteTexture(t))
    buffers.forEach((b) => gl!.deleteBuffer(b))
    if (program) gl!.deleteProgram(program)
    gl!.getExtension('WEBGL_lose_context')?.loseContext()
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
      attribute vec2 aFrom; attribute vec2 aTo; attribute float aCavity;
      uniform vec2 uSize; uniform float uAmount;
      varying vec2 vFrom; varying vec2 vTo; varying float vCavity;
      void main() {
        vec2 point = mix(aFrom, aTo, uAmount) / uSize;
        gl_Position = vec4(point.x*2.0-1.0, 1.0-point.y*2.0, 0.0, 1.0);
        vFrom = aFrom/uSize; vTo = aTo/uSize; vCavity = aCavity;
      }`,
    )
    const precision = gl.getShaderPrecisionFormat(gl.FRAGMENT_SHADER, gl.HIGH_FLOAT)?.precision
      ? 'highp'
      : 'mediump'
    const fragment = compile(
      gl.FRAGMENT_SHADER,
      `
      precision ${precision} float;
      uniform sampler2D uFrom; uniform sampler2D uTo; uniform sampler2D uMaskFrom; uniform sampler2D uMaskTo;
      uniform vec2 uSize; uniform float uAmount; uniform float uCavityAmount; uniform vec2 uInner[20];
      varying vec2 vFrom; varying vec2 vTo; varying float vCavity;
      float edgeDistance(vec2 p, vec2 a, vec2 b) {
        vec2 d=b-a; return length(p-a-clamp(dot(p-a,d)/max(dot(d,d),0.0001),0.0,1.0)*d);
      }
      void main() {
        vec2 uv=vec2(gl_FragCoord.x,uSize.y-gl_FragCoord.y)/uSize;
        vec3 color=mix(texture2D(uFrom,vFrom).rgb,texture2D(uTo,vTo).rgb,uAmount);
        if(vCavity>0.5) {
          vec2 p=uv*uSize; float edge=edgeDistance(p,uInner[19],uInner[0]);
          for(int i=0;i<19;i++) edge=min(edge,edgeDistance(p,uInner[i],uInner[i+1]));
          vec3 interior=mix(texture2D(uFrom,vFrom).rgb,texture2D(uTo,vTo).rgb,uCavityAmount);
          color=mix(color,interior,smoothstep(0.0,0.8,edge));
        }
        float alpha=mix(texture2D(uMaskFrom,uv).r,texture2D(uMaskTo,uv).r,uAmount);
        gl_FragColor=vec4(color,alpha);
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
    const amountLocation = gl.getUniformLocation(program, 'uAmount'),
      cavityLocation = gl.getUniformLocation(program, 'uCavityAmount')
    const innerLocation = gl.getUniformLocation(program, 'uInner[0]'),
      inner = new Float32Array(40)
    const cavity = new Set(mesh.cavity_triangles)
    const vertices = new Float32Array(
      mesh.triangles.flatMap((tri, k) =>
        tri.flatMap((i) => [...mesh.rest[i], ...mesh.open[i], cavity.has(k) ? 1 : 0]),
      ),
    )
    const buffer = gl.createBuffer()
    if (!buffer) throw new Error('Buffer allocation failed')
    buffers.push(buffer)
    gl.bindBuffer(gl.ARRAY_BUFFER, buffer)
    gl.bufferData(gl.ARRAY_BUFFER, vertices, gl.STATIC_DRAW)
    for (const [name, length, offset] of [
      ['aFrom', 2, 0],
      ['aTo', 2, 8],
      ['aCavity', 1, 16],
    ] as const) {
      const attribute = gl.getAttribLocation(program, name)
      gl.enableVertexAttribArray(attribute)
      gl.vertexAttribPointer(attribute, length, gl.FLOAT, false, 20, offset)
    }
    for (const [unit, image, name] of [
      [0, from.surface, 'uFrom'],
      [1, to.surface, 'uTo'],
      [2, from.mask, 'uMaskFrom'],
      [3, to.mask, 'uMaskTo'],
    ] as const) {
      const texture = gl.createTexture()
      if (!texture) throw new Error('Texture allocation failed')
      textures.push(texture)
      gl.activeTexture(gl.TEXTURE0 + unit)
      gl.bindTexture(gl.TEXTURE_2D, texture)
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, unit > 1 ? gl.NEAREST : gl.LINEAR)
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, unit > 1 ? gl.NEAREST : gl.LINEAR)
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
      gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, gl.RGBA, gl.UNSIGNED_BYTE, image)
      gl.uniform1i(gl.getUniformLocation(program, name), unit)
    }
    return {
      canvas,
      dispose,
      draw(amount: number) {
        if (gl.isContextLost()) return
        const closed = (pose: PoseImages) => ['Rest', 'MBP'].includes(pose.data.id)
        gl.uniform1f(amountLocation, amount)
        gl.uniform1f(cavityLocation, closed(from) ? 1 : closed(to) ? 0 : amount)
        for (let i = 0; i < 20; i++)
          for (let axis = 0; axis < 2; axis++)
            inner[i * 2 + axis] =
              mesh.rest[20 + i][axis] * (1 - amount) + mesh.open[20 + i][axis] * amount
        gl.uniform2fv(innerLocation, inner)
        gl.clearColor(0, 0, 0, 0)
        gl.clear(gl.COLOR_BUFFER_BIT)
        gl.drawArrays(gl.TRIANGLES, 0, mesh.triangles.length * 3)
        context.clearRect(0, 0, mesh.width, mesh.height)
        context.drawImage(gpu, 0, 0)
        teeth.clearRect(0, 0, mesh.width, mesh.height)
        teeth.save()
        teeth.beginPath()
        teeth.moveTo(inner[0], inner[1])
        for (let i = 1; i < 20; i++) teeth.lineTo(inner[i * 2], inner[i * 2 + 1])
        teeth.closePath()
        teeth.clip()
        // Closed poses use the lip aperture; rounded poses additionally advance
        // an occluding edge. Exposure never controls enamel opacity.
        const exposure = (row: 'upper_visible' | 'lower_visible') => {
          const a = closed(from) ? to.data[row] : from.data[row]
          const b = closed(to) ? from.data[row] : to.data[row]
          return Number(a) * (1 - amount) + Number(b) * amount
        }
        drawDentalRow(teeth, upper, upperBounds, 'upper', exposure('upper_visible'))
        if (from.data.lower_visible || to.data.lower_visible) {
          const dx =
            from.data.lower_translation[0] * (1 - amount) + to.data.lower_translation[0] * amount
          const dy =
            from.data.lower_translation[1] * (1 - amount) + to.data.lower_translation[1] * amount
          drawDentalRow(teeth, lower, lowerBounds, 'lower', exposure('lower_visible'), dx, dy)
        }
        teeth.restore()
        const inset =
          (from.data.dental_inset_px ?? 0) * (1 - amount) + (to.data.dental_inset_px ?? 0) * amount
        if (inset > 0) {
          teeth.globalCompositeOperation = 'destination-out'
          teeth.strokeStyle = '#000'
          teeth.lineWidth = inset * 2
          teeth.lineJoin = 'round'
          teeth.stroke()
        }
        teeth.globalCompositeOperation = 'destination-in'
        teeth.drawImage(gpu, 0, 0)
        teeth.globalCompositeOperation = 'source-over'
        context.drawImage(dental, 0, 0)
      },
    }
  } catch {
    dispose()
    return null
  }
}
