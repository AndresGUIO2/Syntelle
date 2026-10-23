import type { OralLighting, PoseImages, PoseMesh } from '../types/oralTypes'
import type { Rect } from '../types/oralTypes'
import type { ReferenceLaughterImages, ReferenceLaughterPackage } from '../types/expressionTypes'
import { createReferenceLaughterPass } from './referenceLaughterPass'

/** Warp lips and pose-specific soft tissue; composite rigid reference teeth last. */
export function createCompletedMorph(
  from: PoseImages,
  to: PoseImages,
  mesh: PoseMesh,
  upper: HTMLImageElement,
  lower: HTMLImageElement,
  unavailable: () => void,
  lighting?: OralLighting,
  reference?: {
    package: ReferenceLaughterPackage
    images: ReferenceLaughterImages
    oralCrop: Rect
    chinY: number
  },
) {
  const semantic = Boolean(from.data.aperture_sdf && to.data.aperture_sdf)
  const gpu = document.createElement('canvas')
  gpu.width = mesh.width
  gpu.height = mesh.height
  const canvas = document.createElement('canvas')
  canvas.width = mesh.width
  canvas.height = mesh.height
  const context = canvas.getContext('2d')
  const gl = gpu.getContext('webgl', {
    alpha: true,
    antialias: false,
    premultipliedAlpha: false,
    preserveDrawingBuffer: true,
    powerPreference: 'low-power',
  })
  if (!gl || !context) return null
  const shaders: WebGLShader[] = [],
    textures: WebGLTexture[] = [],
    buffers: WebGLBuffer[] = []
  let program: WebGLProgram | null = null
  let referencePass: ReturnType<typeof createReferenceLaughterPass> | undefined
  function onLoss(event: Event) {
    event.preventDefault()
    unavailable()
  }
  gpu.addEventListener('webglcontextlost', onLoss)
  function dispose() {
    referencePass?.dispose()
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
      attribute vec2 aFrom; attribute vec2 aTo;
      uniform vec2 uSize; uniform float uAmount;
      varying vec2 vFrom; varying vec2 vTo;
      void main() {
        vec2 point = mix(aFrom, aTo, uAmount) / uSize;
        gl_Position = vec4(point.x*2.0-1.0, 1.0-point.y*2.0, 0.0, 1.0);
        vFrom = aFrom/uSize; vTo = aTo/uSize;
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
      uniform sampler2D uUpper; uniform sampler2D uLower;
      uniform sampler2D uCavityFrom; uniform sampler2D uCavityTo;
      uniform vec2 uJaw;
      uniform vec2 uUpperShift;
      uniform bool uSemantic;
      uniform vec4 uLighting;
      uniform vec4 uVisibleAperture;
      uniform vec4 uApertureBounds;
      uniform vec2 uSize; uniform float uAmount; uniform float uCavityAmount; uniform vec2 uInner[20];
      varying vec2 vFrom; varying vec2 vTo;
      float edgeDistance(vec2 p, vec2 a, vec2 b) {
        vec2 d=b-a; return length(p-a-clamp(dot(p-a,d)/max(dot(d,d),0.0001),0.0,1.0)*d);
      }
      float edgeSquared(vec2 p, vec2 a, vec2 b) {
        vec2 d=b-a;
        vec2 q=p-a-clamp(dot(p-a,d)/max(dot(d,d),0.0001),0.0,1.0)*d;
        return dot(q,q);
      }
      bool crosses(vec2 p,vec2 a,vec2 b) {
        if((a.y>p.y)==(b.y>p.y)) return false;
        return p.x<(b.x-a.x)*(p.y-a.y)/(b.y-a.y)+a.x;
      }
      void main() {
        vec2 uv=vec2(gl_FragCoord.x,uSize.y-gl_FragCoord.y)/uSize;
        vec2 fromUV=uAmount==0.0?uv:vFrom;
        vec2 toUV=uAmount==1.0?uv:vTo;
        vec3 fromColor=texture2D(uFrom,fromUV).rgb,toColor=texture2D(uTo,toUV).rgb;
        vec3 color=uAmount==0.0?fromColor:uAmount==1.0?toColor:mix(fromColor,toColor,uAmount);
        vec2 p=uv*uSize;
        if(uSemantic || (p.x>=uApertureBounds.x-1.0 && p.y>=uApertureBounds.y-1.0 && p.x<=uApertureBounds.z+1.0 && p.y<=uApertureBounds.w+1.0 && uInner[15].y-uInner[5].y>uLighting.w)) {
          float coverage;
          float apertureDistance=0.0,openingRatio=0.0;
          if(uSemantic) {
            float a=(texture2D(uMaskFrom,fromUV).g*255.0-128.0)/8.0;
            float b=(texture2D(uMaskTo,toUV).g*255.0-128.0)/8.0;
            float distance=uAmount==0.0?a:uAmount==1.0?b:mix(a,b,uAmount);
            coverage=smoothstep(-0.5,0.5,distance);
            apertureDistance=max(0.0,distance);
            if(uVisibleAperture.x>0.0) openingRatio=mix(texture2D(uMaskFrom,fromUV).b,texture2D(uMaskTo,toUV).b,uAmount);
          } else {
            float edge=edgeDistance(p,uInner[19],uInner[0]);
            bool inside=crosses(p,uInner[19],uInner[0]);
            for(int i=0;i<19;i++) {
              edge=min(edge,edgeDistance(p,uInner[i],uInner[i+1]));
              if(crosses(p,uInner[i],uInner[i+1])) inside=!inside;
            }
            // Keep the existing analytic boundary for non-semantic packages.
            coverage=smoothstep(-0.5,0.65,inside?edge:-edge);
            apertureDistance=inside?edge:0.0;
            if(uVisibleAperture.x>0.0) openingRatio=max(0.0,uInner[15].y-uInner[5].y)/uVisibleAperture.x;
          }
          vec3 lip=color;
          float gap=uInner[15].y-uInner[5].y;
          // Zero aperture coverage contributes no oral color. Avoid dental
          // texture/shadow work there, especially under CPU software rendering.
          if(coverage>0.0 && (uSemantic || gap>uLighting.w)) {
            vec3 cavityFrom=texture2D(uCavityFrom,fromUV).rgb,cavityTo=texture2D(uCavityTo,toUV).rgb;
            vec3 interior=uCavityAmount==0.0?cavityFrom:uCavityAmount==1.0?cavityTo:mix(cavityFrom,cavityTo,uCavityAmount);
            vec4 upper=texture2D(uUpper,(p-uUpperShift)/uSize);
            vec4 lower=texture2D(uLower,(p-uJaw)/uSize);
            // Interpolate covered color, then unpremultiply. Transparent black
            // texels must not create dark seams around enamel silhouettes.
            upper.rgb/=max(upper.a,0.00001);
            lower.rgb/=max(lower.a,0.00001);
            if(upper.a>0.0 || lower.a>0.0) {
              // min(sqrt(d)) == sqrt(min(d)); only enamel needs its shadow.
              float edge=min(100000000.0,edgeSquared(p,uInner[19],uInner[0]));
              for(int i=0;i<19;i++) edge=min(edge,edgeSquared(p,uInner[i],uInner[i+1]));
              float depth=1.0-uLighting.z*pow(abs(p.x-(uInner[0].x+uInner[10].x)*0.5)/max(1.0,(uInner[10].x-uInner[0].x)*0.5),2.0);
              float illumination=clamp(depth-uLighting.x*exp(-sqrt(edge)/max(.1,uLighting.y)),uSemantic?0.22:0.68,1.0);
              if(uVisibleAperture.x>0.0) {
                float ambient=mix(uVisibleAperture.y,1.0,smoothstep(0.0,uVisibleAperture.z,openingRatio));
                illumination=clamp(depth-uLighting.x*exp(-apertureDistance/uVisibleAperture.w),.12,1.0)*ambient;
              }
              // The same color-only lighting, algebraically factored to one
              // scalar power rather than twelve channel powers per fragment.
              float exposure=pow(illumination,1.0/2.2);
              lower.rgb*=exposure;
              upper.rgb*=exposure;
            }
            vec3 oral=mix(interior,lower.rgb,lower.a);
            oral=mix(oral,upper.rgb,upper.a);
            color=mix(lip,oral,coverage);
          }
        }
        float fromAlpha=texture2D(uMaskFrom,uv).r,toAlpha=texture2D(uMaskTo,uv).r;
        float alpha=uAmount==0.0?fromAlpha:uAmount==1.0?toAlpha:mix(fromAlpha,toAlpha,uAmount);
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
    gl.uniform1i(gl.getUniformLocation(program, 'uSemantic'), semantic ? 1 : 0)
    gl.uniform4f(
      gl.getUniformLocation(program, 'uVisibleAperture'),
      lighting?.visible_aperture?.reference_gap_px ?? 0,
      lighting?.visible_aperture?.minimum_irradiance ?? 1,
      lighting?.visible_aperture?.full_light_ratio ?? 1,
      lighting?.visible_aperture?.contact_width_px ?? 1,
    )
    gl.uniform4f(
      gl.getUniformLocation(program, 'uLighting'),
      lighting?.contact_shadow_strength ?? 0.16,
      lighting?.contact_shadow_width_px ?? 3,
      lighting?.depth_strength ?? 0.08,
      lighting?.closed_gap_px ?? 0.8,
    )
    const amountLocation = gl.getUniformLocation(program, 'uAmount'),
      cavityLocation = gl.getUniformLocation(program, 'uCavityAmount'),
      jawLocation = gl.getUniformLocation(program, 'uJaw')
    const upperLocation = gl.getUniformLocation(program, 'uUpperShift')
    const innerLocation = gl.getUniformLocation(program, 'uInner[0]'),
      inner = new Float32Array(40)
    const boundsLocation = gl.getUniformLocation(program, 'uApertureBounds')
    const vertices = new Float32Array(
      mesh.triangles.flatMap((tri) => tri.flatMap((i) => [...mesh.rest[i], ...mesh.open[i]])),
    )
    const buffer = gl.createBuffer()
    if (!buffer) throw new Error('Buffer allocation failed')
    buffers.push(buffer)
    gl.bindBuffer(gl.ARRAY_BUFFER, buffer)
    gl.bufferData(gl.ARRAY_BUFFER, vertices, gl.STATIC_DRAW)
    for (const [name, length, offset] of [
      ['aFrom', 2, 0],
      ['aTo', 2, 8],
    ] as const) {
      const attribute = gl.getAttribLocation(program, name)
      gl.enableVertexAttribArray(attribute)
      gl.vertexAttribPointer(attribute, length, gl.FLOAT, false, 16, offset)
    }
    for (const [unit, image, name] of [
      [0, from.surface, 'uFrom'],
      [1, to.surface, 'uTo'],
      [2, from.mask, 'uMaskFrom'],
      [3, to.mask, 'uMaskTo'],
      [4, upper, 'uUpper'],
      [5, lower, 'uLower'],
      [6, from.cavity ?? from.surface, 'uCavityFrom'],
      [7, to.cavity ?? to.surface, 'uCavityTo'],
    ] as const) {
      const texture = gl.createTexture()
      if (!texture) throw new Error('Texture allocation failed')
      textures.push(texture)
      gl.activeTexture(gl.TEXTURE0 + unit)
      gl.bindTexture(gl.TEXTURE_2D, texture)
      const filter = (unit === 2 || unit === 3) && !semantic ? gl.NEAREST : gl.LINEAR
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, filter)
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, filter)
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
      gl.pixelStorei(gl.UNPACK_PREMULTIPLY_ALPHA_WEBGL, unit === 4 || unit === 5)
      gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, gl.RGBA, gl.UNSIGNED_BYTE, image)
      gl.uniform1i(gl.getUniformLocation(program, name), unit)
    }
    return {
      canvas,
      gpuCanvas: gpu,
      dispose,
      setPair(nextFrom: PoseImages, nextTo: PoseImages, nextMesh: PoseMesh) {
        if (
          nextMesh.width !== mesh.width ||
          nextMesh.height !== mesh.height ||
          Boolean(nextFrom.data.aperture_sdf && nextTo.data.aperture_sdf) !== semantic
        )
          throw Error('Incompatible oral resource update')
        from = nextFrom
        to = nextTo
        mesh = nextMesh
        gl.useProgram(program)
        gl.bindBuffer(gl.ARRAY_BUFFER, buffer)
        gl.bufferData(
          gl.ARRAY_BUFFER,
          new Float32Array(
            mesh.triangles.flatMap((tri) => tri.flatMap((i) => [...mesh.rest[i], ...mesh.open[i]])),
          ),
          gl.STATIC_DRAW,
        )
        for (const [unit, image] of [
          [0, from.surface],
          [1, to.surface],
          [2, from.mask],
          [3, to.mask],
          [6, from.cavity ?? from.surface],
          [7, to.cavity ?? to.surface],
        ] as const) {
          gl.activeTexture(gl.TEXTURE0 + unit)
          gl.bindTexture(gl.TEXTURE_2D, textures[unit]!)
          gl.pixelStorei(gl.UNPACK_PREMULTIPLY_ALPHA_WEBGL, false)
          gl.texSubImage2D(gl.TEXTURE_2D, 0, 0, 0, gl.RGBA, gl.UNSIGNED_BYTE, image)
        }
      },
      draw(amount: number, copyToCanvas = true) {
        if (gl.isContextLost()) return
        if (gpu.width !== mesh.width || gpu.height !== mesh.height) {
          gpu.width = mesh.width
          gpu.height = mesh.height
        }
        gl.viewport(0, 0, mesh.width, mesh.height)
        gl.useProgram(program)
        gl.bindBuffer(gl.ARRAY_BUFFER, buffer)
        for (const [name, offset] of [
          ['aFrom', 0],
          ['aTo', 8],
        ] as const) {
          const attribute = gl.getAttribLocation(program!, name)
          gl.enableVertexAttribArray(attribute)
          gl.vertexAttribPointer(attribute, 2, gl.FLOAT, false, 16, offset)
        }
        textures.forEach((texture, unit) => {
          gl.activeTexture(gl.TEXTURE0 + unit)
          gl.bindTexture(gl.TEXTURE_2D, texture)
        })
        const closed = (pose: PoseImages) => ['Rest', 'MBP'].includes(pose.data.id)
        gl.uniform1f(amountLocation, amount)
        gl.uniform1f(cavityLocation, closed(from) ? 1 : closed(to) ? 0 : amount)
        for (let i = 0; i < 20; i++)
          for (let axis = 0; axis < 2; axis++)
            inner[i * 2 + axis] =
              mesh.rest[20 + i][axis] * (1 - amount) + mesh.open[20 + i][axis] * amount
        let minX = Infinity,
          minY = Infinity,
          maxX = -Infinity,
          maxY = -Infinity
        for (let i = 0; i < 20; i++) {
          minX = Math.min(minX, inner[i * 2])
          maxX = Math.max(maxX, inner[i * 2])
          minY = Math.min(minY, inner[i * 2 + 1])
          maxY = Math.max(maxY, inner[i * 2 + 1])
        }
        gl.uniform4f(boundsLocation, minX, minY, maxX, maxY)
        gl.uniform2f(
          jawLocation,
          from.data.lower_translation[0] * (1 - amount) + to.data.lower_translation[0] * amount,
          from.data.lower_translation[1] * (1 - amount) + to.data.lower_translation[1] * amount,
        )
        const upperFrom = from.data.upper_translation ?? [0, 0],
          upperTo = to.data.upper_translation ?? [0, 0]
        gl.uniform2f(
          upperLocation,
          upperFrom[0] * (1 - amount) + upperTo[0] * amount,
          upperFrom[1] * (1 - amount) + upperTo[1] * amount,
        )
        gl.uniform2fv(innerLocation, inner)
        gl.clearColor(0, 0, 0, 0)
        gl.clear(gl.COLOR_BUFFER_BIT)
        gl.drawArrays(gl.TRIANGLES, 0, mesh.triangles.length * 3)
        if (copyToCanvas) {
          context.clearRect(0, 0, mesh.width, mesh.height)
          context.drawImage(gpu, 0, 0)
        }
        // The very same path paints exact endpoints and every intermediate frame.
      },
      drawReference(amount: number, energy: number) {
        if (!reference || gl.isContextLost()) return
        referencePass ??= createReferenceLaughterPass(
          gl,
          gpu,
          reference.package,
          reference.images,
          reference.oralCrop,
          reference.chinY,
          textures[4]!,
          textures[5]!,
        )
        referencePass.draw(amount, energy)
      },
    }
  } catch {
    dispose()
    return null
  }
}
