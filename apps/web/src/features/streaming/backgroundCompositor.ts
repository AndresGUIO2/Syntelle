// Public receiver only: decoded pixels/coverage plus a public background.
// No character geometry, animation, segmentation or private materials live here.
const vertex = `attribute vec2 position; varying vec2 uv;
void main(){ uv=position*.5+.5; gl_Position=vec4(position,0.,1.); }`
const fragment = `precision highp float;
varying vec2 uv;
uniform sampler2D pixels; uniform sampler2D background;
uniform int mode; uniform vec2 stepSize; uniform float sigma;
void main(){
 if(mode==0){
  vec2 c=vec2(clamp(uv.x*.5,.5/1440.,719.5/1440.),clamp(uv.y,.5/960.,959.5/960.));
  vec3 rgb=texture2D(pixels,c).rgb;
  float alpha=texture2D(pixels,c+vec2(.5,0.)).r;
  gl_FragColor=vec4(min(rgb,vec3(alpha))+texture2D(background,uv).rgb*(1.-alpha),1.);
 } else {
  vec3 color=vec3(0.); float weight=0.;
  for(int i=-60;i<=60;i++){
   float d=float(i); if(abs(d)<=ceil(sigma*3.)){
    float w=exp(-d*d/(2.*sigma*sigma));
    color+=texture2D(pixels,uv+stepSize*d).rgb*w; weight+=w;
   }
  }
  gl_FragColor=vec4(color/weight,1.);
 }
}`

export function createBackgroundCompositor(canvas: HTMLCanvasElement) {
  canvas.width = 720
  canvas.height = 960
  const gl = canvas.getContext('webgl', {
    alpha: false,
    antialias: false,
    depth: false,
    preserveDrawingBuffer: true,
  })
  if (!gl) throw Error('This browser cannot display the live video composition.')
  const shaders: WebGLShader[] = []
  const program = gl.createProgram()!
  const buffer = gl.createBuffer()!
  const textures: WebGLTexture[] = []
  const framebuffer = gl.createFramebuffer()!
  function shader(type: number, source: string) {
    const result = gl!.createShader(type)!
    shaders.push(result)
    gl!.shaderSource(result, source)
    gl!.compileShader(result)
    if (!gl!.getShaderParameter(result, gl!.COMPILE_STATUS))
      throw Error('Video composition could not start.')
    gl!.attachShader(program, result)
  }
  function texture() {
    const result = gl!.createTexture()!
    textures.push(result)
    gl!.bindTexture(gl!.TEXTURE_2D, result)
    gl!.texParameteri(gl!.TEXTURE_2D, gl!.TEXTURE_MIN_FILTER, gl!.LINEAR)
    gl!.texParameteri(gl!.TEXTURE_2D, gl!.TEXTURE_MAG_FILTER, gl!.LINEAR)
    gl!.texParameteri(gl!.TEXTURE_2D, gl!.TEXTURE_WRAP_S, gl!.CLAMP_TO_EDGE)
    gl!.texParameteri(gl!.TEXTURE_2D, gl!.TEXTURE_WRAP_T, gl!.CLAMP_TO_EDGE)
    gl!.texImage2D(gl!.TEXTURE_2D, 0, gl!.RGBA, 720, 960, 0, gl!.RGBA, gl!.UNSIGNED_BYTE, null)
    return result
  }
  function dispose() {
    textures.forEach((item) => gl!.deleteTexture(item))
    shaders.forEach((item) => gl!.deleteShader(item))
    gl!.deleteFramebuffer(framebuffer)
    gl!.deleteBuffer(buffer)
    gl!.deleteProgram(program)
    gl!.getExtension('WEBGL_lose_context')?.loseContext()
  }
  try {
    shader(gl.VERTEX_SHADER, vertex)
    shader(gl.FRAGMENT_SHADER, fragment)
    gl.linkProgram(program)
    if (!gl.getProgramParameter(program, gl.LINK_STATUS))
      throw Error('Video composition could not start.')
    gl.useProgram(program)
    gl.bindBuffer(gl.ARRAY_BUFFER, buffer)
    gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([-1, -1, 1, -1, -1, 1, 1, 1]), gl.STATIC_DRAW)
    const position = gl.getAttribLocation(program, 'position')
    gl.enableVertexAttribArray(position)
    gl.vertexAttribPointer(position, 2, gl.FLOAT, false, 0, 0)
    gl.uniform1i(gl.getUniformLocation(program, 'pixels'), 0)
    gl.uniform1i(gl.getUniformLocation(program, 'background'), 1)
    const input = texture(),
      original = texture(),
      horizontal = texture(),
      blurred = texture()
    let selected = original
    const mode = gl.getUniformLocation(program, 'mode'),
      sigma = gl.getUniformLocation(program, 'sigma'),
      step = gl.getUniformLocation(program, 'stepSize')
    const staging = document.createElement('canvas')
    staging.width = 720
    staging.height = 960
    const ctx = staging.getContext('2d')!
    function bind(unit: number, target: WebGLTexture) {
      gl!.activeTexture(gl!.TEXTURE0 + unit)
      gl!.bindTexture(gl!.TEXTURE_2D, target)
    }
    function upload(target: WebGLTexture, image: TexImageSource) {
      bind(0, target)
      gl!.pixelStorei(gl!.UNPACK_FLIP_Y_WEBGL, true)
      gl!.pixelStorei(gl!.UNPACK_PREMULTIPLY_ALPHA_WEBGL, false)
      gl!.texImage2D(gl!.TEXTURE_2D, 0, gl!.RGBA, gl!.RGBA, gl!.UNSIGNED_BYTE, image)
    }
    function setBackground(image: HTMLImageElement, amount: number) {
      const scale = Math.max(720 / image.naturalWidth, 960 / image.naturalHeight)
      ctx.drawImage(
        image,
        (720 - image.naturalWidth * scale) / 2,
        (960 - image.naturalHeight * scale) / 2,
        image.naturalWidth * scale,
        image.naturalHeight * scale,
      )
      upload(original, staging)
      selected = original
      const blur = Math.max(0, Math.min(20, amount))
      if (blur > 0) {
        gl!.bindFramebuffer(gl!.FRAMEBUFFER, framebuffer)
        gl!.viewport(0, 0, 720, 960)
        gl!.uniform1i(mode, 1)
        gl!.uniform1f(sigma, blur)
        for (const [source, target, x, y] of [
          [original, horizontal, 1 / 720, 0],
          [horizontal, blurred, 0, 1 / 960],
        ] as const) {
          bind(0, source)
          bind(1, original)
          // The unused background sampler must not reference the render target.
          gl!.framebufferTexture2D(
            gl!.FRAMEBUFFER,
            gl!.COLOR_ATTACHMENT0,
            gl!.TEXTURE_2D,
            target,
            0,
          )
          gl!.uniform2f(step, x, y)
          gl!.drawArrays(gl!.TRIANGLE_STRIP, 0, 4)
        }
        selected = blurred
      }
      gl!.bindFramebuffer(gl!.FRAMEBUFFER, null)
    }
    function draw(image: TexImageSource) {
      upload(input, image)
      bind(1, selected)
      gl!.bindFramebuffer(gl!.FRAMEBUFFER, null)
      gl!.viewport(0, 0, 720, 960)
      gl!.uniform1i(mode, 0)
      gl!.drawArrays(gl!.TRIANGLE_STRIP, 0, 4)
      if (gl!.isContextLost())
        throw Error('Video composition was interrupted. Reconnect to try again.')
    }
    return { setBackground, draw, dispose }
  } catch (error) {
    dispose()
    throw error
  }
}
