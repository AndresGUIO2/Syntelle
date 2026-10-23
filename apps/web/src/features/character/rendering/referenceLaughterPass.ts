import type { ReferenceLaughterImages, ReferenceLaughterPackage } from '../types/expressionTypes'
import type { Rect } from '../types/oralTypes'

/** Independent facial pass sharing the ordinary oral renderer's WebGL context. */
export function createReferenceLaughterPass(
  gl: WebGLRenderingContext,
  canvas: HTMLCanvasElement,
  data: ReferenceLaughterPackage,
  images: ReferenceLaughterImages,
  oralCrop: Rect,
  chinY: number,
  upper: WebGLTexture,
  lower: WebGLTexture,
) {
  const program = gl.createProgram()!
  const shaders: WebGLShader[] = []
  const textures: WebGLTexture[] = []
  const buffer = gl.createBuffer()!
  const compile = (type: number, source: string) => {
    const shader = gl.createShader(type)!
    shaders.push(shader)
    gl.shaderSource(shader, source)
    gl.compileShader(shader)
    if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) throw Error('Reference shader failed')
    gl.attachShader(program, shader)
  }
  function dispose() {
    shaders.forEach((value) => gl.deleteShader(value))
    textures.forEach((value) => gl.deleteTexture(value))
    gl.deleteBuffer(buffer)
    gl.deleteProgram(program)
  }
  try {
    compile(gl.VERTEX_SHADER, `attribute vec2 point;void main(){gl_Position=vec4(point,0.,1.);}`)
    compile(
      gl.FRAGMENT_SHADER,
      `precision highp float;
      uniform sampler2D faceA,faceB,cavity,upperRow,lowerRow;
      uniform vec2 size,origin,lowerShift;uniform vec4 oralRect;
      uniform float fraction,jaw,chin;uniform vec2 inner[20],outer[20];
      uniform vec3 contactShadow;
      float jawWeight(vec2 p){
        float left=outer[0].x+oralRect.x,right=outer[10].x+oralRect.x;
        float width=max(1.,right-left),x=(p.x-(left+right)*.5)/(width*.85);
        float h=max(0.,1.-x*x);h*=h;
        return h*smoothstep(outer[5].y+oralRect.y,outer[15].y+oralRect.y,p.y)*(1.-smoothstep(chin,chin+width*.3,p.y));
      }
      float distanceTo(vec2 p,vec2 a,vec2 b){vec2 d=b-a;return length(p-a-clamp(dot(p-a,d)/max(dot(d,d),.0001),0.,1.)*d);}
      bool crosses(vec2 p,vec2 a,vec2 b){if((a.y>p.y)==(b.y>p.y))return false;return p.x<(b.x-a.x)*(p.y-a.y)/(b.y-a.y)+a.x;}
      void main(){
        vec2 p=vec2(gl_FragCoord.x,size.y-gl_FragCoord.y)-.5;
        vec2 world=p+origin,q=world;
        for(int n=0;n<3;n++)q.y=world.y-jaw*jawWeight(q);
        vec2 uv=(q-origin+.5)/size;
        vec4 base=mix(texture2D(faceA,uv),texture2D(faceB,uv),fraction);
        vec2 local=world-oralRect.xy,point=local+.5;
        if(local.x>=0.&&local.y>=0.&&local.x<oralRect.z&&local.y<oralRect.w){
          bool inside=crosses(point,inner[19],inner[0]);float edge=distanceTo(point,inner[19],inner[0]);
          for(int i=0;i<19;i++){edge=min(edge,distanceTo(point,inner[i],inner[i+1]));if(crosses(point,inner[i],inner[i+1]))inside=!inside;}
          float coverage=smoothstep(-.5,.65,inside?edge:-edge);
          vec3 color=texture2D(cavity,(local+vec2(.5,.5-jaw*.5))/oralRect.zw).rgb;
          vec4 lower=texture2D(lowerRow,(local-lowerShift+.5)/oralRect.zw);
          vec4 upper=texture2D(upperRow,(local+.5)/oralRect.zw);
          if(contactShadow.x>0.&&contactShadow.z>0.){
            float closure=clamp(1.-(inner[15].y-inner[5].y)/contactShadow.z,0.,1.);
            float light=pow(max(.55,1.-contactShadow.x*closure*exp(-max(0.,inside?edge:-edge)/contactShadow.y)),1./2.2);
            lower.rgb*=light;upper.rgb*=light;
          }
          color=lower.rgb+color*(1.-lower.a);color=upper.rgb+color*(1.-upper.a);
          base.rgb=mix(base.rgb,color,coverage);
        }
        gl_FragColor=base;
      }`,
    )
    gl.linkProgram(program)
    if (!gl.getProgramParameter(program, gl.LINK_STATUS))
      throw Error('Reference shader link failed')
    gl.useProgram(program)
    gl.bindBuffer(gl.ARRAY_BUFFER, buffer)
    gl.bufferData(
      gl.ARRAY_BUFFER,
      new Float32Array([-1, -1, 1, -1, -1, 1, -1, 1, 1, -1, 1, 1]),
      gl.STATIC_DRAW,
    )
    function texture(image: HTMLImageElement) {
      const value = gl.createTexture()!
      textures.push(value)
      gl.bindTexture(gl.TEXTURE_2D, value)
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
      gl.pixelStorei(gl.UNPACK_PREMULTIPLY_ALPHA_WEBGL, false)
      gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, gl.RGBA, gl.UNSIGNED_BYTE, image)
      return value
    }
    const faces = images.frames.map(texture),
      cavity = texture(images.cavity)
    const referenceUpper = images.upper ? texture(images.upper) : upper
    const referenceLower = images.lower ? texture(images.lower) : lower
    return {
      dispose,
      draw(amount: number, energy: number) {
        const level = Math.max(0, Math.min(1, amount)) * 24
        const lo = Math.floor(level),
          hi = Math.min(24, lo + 1),
          f = level - lo
        const jaw = data.oral.jaw_limit_px * Math.max(0, Math.min(1, energy)) * amount
        const sample = (frames: readonly number[][][]) =>
          frames[lo]!.flatMap((point, i) =>
            point.map((v, axis) => v * (1 - f) + frames[hi]![i]![axis]! * f),
          )
        const inner = sample(data.oral.inner),
          outer = sample(data.oral.outer)
        for (let i = 11; i < 20; i++) inner[i * 2 + 1]! += jaw * Math.sin((Math.PI * (20 - i)) / 10)
        if (canvas.width !== data.crop.width || canvas.height !== data.crop.height) {
          canvas.width = data.crop.width
          canvas.height = data.crop.height
        }
        gl.viewport(0, 0, canvas.width, canvas.height)
        gl.useProgram(program)
        gl.bindBuffer(gl.ARRAY_BUFFER, buffer)
        const point = gl.getAttribLocation(program, 'point')
        gl.enableVertexAttribArray(point)
        gl.vertexAttribPointer(point, 2, gl.FLOAT, false, 0, 0)
        for (const [unit, name, image] of [
          [0, 'faceA', faces[lo]!],
          [1, 'faceB', faces[hi]!],
          [2, 'cavity', cavity],
          [3, 'upperRow', referenceUpper],
          [4, 'lowerRow', referenceLower],
        ] as const) {
          gl.activeTexture(gl.TEXTURE0 + unit)
          gl.bindTexture(gl.TEXTURE_2D, image)
          gl.uniform1i(gl.getUniformLocation(program, name), unit)
        }
        gl.uniform2f(gl.getUniformLocation(program, 'size'), canvas.width, canvas.height)
        gl.uniform2f(gl.getUniformLocation(program, 'origin'), data.crop.x, data.crop.y)
        gl.uniform4f(
          gl.getUniformLocation(program, 'oralRect'),
          oralCrop.x,
          oralCrop.y,
          oralCrop.width,
          oralCrop.height,
        )
        gl.uniform1f(gl.getUniformLocation(program, 'fraction'), f)
        const shadow = data.oral.contact_shadow
        const full = data.oral.inner[24]!
        gl.uniform3f(
          gl.getUniformLocation(program, 'contactShadow'),
          shadow?.strength ?? 0,
          shadow?.width_px ?? 1,
          full[15]![1]! - full[5]![1]!,
        )
        gl.uniform1f(gl.getUniformLocation(program, 'jaw'), jaw)
        gl.uniform1f(gl.getUniformLocation(program, 'chin'), chinY)
        gl.uniform2fv(gl.getUniformLocation(program, 'inner[0]'), inner)
        gl.uniform2fv(gl.getUniformLocation(program, 'outer[0]'), outer)
        const shiftA = data.oral.lower_translation[lo]!,
          shiftB = data.oral.lower_translation[hi]!
        gl.uniform2f(
          gl.getUniformLocation(program, 'lowerShift'),
          shiftA[0]! * (1 - f) + shiftB[0]! * f,
          shiftA[1]! * (1 - f) + shiftB[1]! * f + jaw,
        )
        gl.clearColor(0, 0, 0, 0)
        gl.clear(gl.COLOR_BUFFER_BIT)
        gl.drawArrays(gl.TRIANGLES, 0, 6)
      },
    }
  } catch (error) {
    dispose()
    throw error
  }
}
