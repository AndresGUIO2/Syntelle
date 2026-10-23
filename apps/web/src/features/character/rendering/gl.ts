export function gpu(
  width: number,
  height: number,
  vertex: string,
  fragment: string,
  unavailable: () => void,
  premultiplied = false,
) {
  const canvas = document.createElement('canvas')
  canvas.width = width
  canvas.height = height
  const gl = canvas.getContext('webgl', {
    alpha: true,
    antialias: false,
    premultipliedAlpha: premultiplied,
    preserveDrawingBuffer: true,
  })
  if (!gl) throw Error('WebGL unavailable')
  const shaders: WebGLShader[] = [],
    textures: WebGLTexture[] = [],
    buffers: WebGLBuffer[] = []
  const program = gl.createProgram()!
  const lost = (event: Event) => {
    event.preventDefault()
    unavailable()
  }
  canvas.addEventListener('webglcontextlost', lost)
  function dispose() {
    canvas.removeEventListener('webglcontextlost', lost)
    textures.forEach((t) => gl!.deleteTexture(t))
    buffers.forEach((b) => gl!.deleteBuffer(b))
    shaders.forEach((s) => gl!.deleteShader(s))
    gl!.deleteProgram(program)
    gl!.getExtension('WEBGL_lose_context')?.loseContext()
  }
  try {
    for (const [type, code] of [
      [gl.VERTEX_SHADER, vertex],
      [gl.FRAGMENT_SHADER, fragment],
    ] as const) {
      const shader = gl.createShader(type)!
      shaders.push(shader)
      gl.shaderSource(shader, code)
      gl.compileShader(shader)
      if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS))
        throw Error(gl.getShaderInfoLog(shader) ?? 'Shader failed')
      gl.attachShader(program, shader)
    }
    gl.linkProgram(program)
    if (!gl.getProgramParameter(program, gl.LINK_STATUS))
      throw Error(gl.getProgramInfoLog(program) ?? 'Program failed')
    gl.useProgram(program)
    gl.viewport(0, 0, width, height)
    gl.disable(gl.BLEND)
    gl.disable(gl.DITHER)
  } catch (error) {
    dispose()
    throw error
  }
  function texture(
    unit: number,
    name: string,
    image: TexImageSource | { data: Uint8Array; width: number; height: number },
    nearest = false,
  ) {
    const tex = gl!.createTexture()!
    textures.push(tex)
    gl!.activeTexture(gl!.TEXTURE0 + unit)
    gl!.bindTexture(gl!.TEXTURE_2D, tex)
    const filter = nearest ? gl!.NEAREST : gl!.LINEAR
    gl!.texParameteri(gl!.TEXTURE_2D, gl!.TEXTURE_MIN_FILTER, filter)
    gl!.texParameteri(gl!.TEXTURE_2D, gl!.TEXTURE_MAG_FILTER, filter)
    gl!.texParameteri(gl!.TEXTURE_2D, gl!.TEXTURE_WRAP_S, gl!.CLAMP_TO_EDGE)
    gl!.texParameteri(gl!.TEXTURE_2D, gl!.TEXTURE_WRAP_T, gl!.CLAMP_TO_EDGE)
    gl!.pixelStorei(gl!.UNPACK_PREMULTIPLY_ALPHA_WEBGL, premultiplied)
    if ('data' in image)
      gl!.texImage2D(
        gl!.TEXTURE_2D,
        0,
        gl!.RGBA,
        image.width,
        image.height,
        0,
        gl!.RGBA,
        gl!.UNSIGNED_BYTE,
        image.data,
      )
    else gl!.texImage2D(gl!.TEXTURE_2D, 0, gl!.RGBA, gl!.RGBA, gl!.UNSIGNED_BYTE, image)
    gl!.uniform1i(gl!.getUniformLocation(program, name), unit)
    return tex
  }
  function attribute(name: string, values: Float32Array, components: number) {
    const buffer = gl!.createBuffer()!
    buffers.push(buffer)
    gl!.bindBuffer(gl!.ARRAY_BUFFER, buffer)
    gl!.bufferData(gl!.ARRAY_BUFFER, values, gl!.STATIC_DRAW)
    const index = gl!.getAttribLocation(program, name)
    gl!.enableVertexAttribArray(index)
    gl!.vertexAttribPointer(index, components, gl!.FLOAT, false, 0, 0)
  }
  function indices(values: Uint16Array, dynamic = false) {
    const buffer = gl!.createBuffer()!
    buffers.push(buffer)
    gl!.bindBuffer(gl!.ELEMENT_ARRAY_BUFFER, buffer)
    gl!.bufferData(gl!.ELEMENT_ARRAY_BUFFER, values, dynamic ? gl!.DYNAMIC_DRAW : gl!.STATIC_DRAW)
    return buffer
  }
  return { canvas, gl, program, texture, attribute, indices, dispose }
}
