import { gpu } from './gl'
import type { GazeImages, GazeOffset, GazePackage } from '../types/gazeTypes'
import type { ReferenceLaughterPackage } from '../types/expressionTypes'

export function createEyeMorph(
  opened: HTMLImageElement,
  closed: HTMLImageElement,
  mask: HTMLImageElement,
  curves: number[][],
  unavailable: () => void,
  gaze?: { package: GazePackage; images: GazeImages },
  reference?: ReferenceLaughterPackage,
  referenceFrames?: readonly HTMLImageElement[],
) {
  const w = opened.width,
    h = opened.height
  const vertex =
    'attribute vec2 point;varying vec2 uv;void main(){uv=vec2((point.x+1.0)*0.5,(1.0-point.y)*0.5);gl_Position=vec4(point,0,1);}'
  const fragment = `precision highp float;
    uniform sampler2D original,closedEye,mask,curves;uniform float amount,height,width,lashBand;varying vec2 uv;
    uniform sampler2D sclera,ownership;uniform vec4 offsets;uniform float eyeSplit;
    uniform bool referenceMode;uniform sampler2D referenceA,referenceB;uniform float referenceFraction;uniform vec4 irisOffsets;
    uniform vec2 referenceRows,faceOffset,faceSize;
    vec4 irisTexel(vec2 p){
      vec3 own=texture2D(ownership,p).rgb;
      float a=own.g*own.b;
      // The photographed edge already contains sclera. Unmatte it before
      // filtering, otherwise the mask is applied twice and gaze starts with
      // a pale ring. Keep signed covered color until final composition.
      vec3 covered=(texture2D(original,p).rgb-(1.-own.g)*texture2D(sclera,p).rgb)*own.b;
      return vec4(covered,a);
    }
    vec4 irisSample(vec2 p){
      // Filter covered color, not the surrounding skin/eyelash photograph.
      // This is equivalent to bilinear filtering a premultiplied iris cutout,
      // using the already uploaded source and ownership textures.
      vec2 size=vec2(width,height),texel=p*size-.5,origin=floor(texel),f=fract(texel);
      vec2 q=(origin+.5)/size,pixelStep=1./size;
      return mix(mix(irisTexel(q),irisTexel(q+vec2(pixelStep.x,0.)),f.x),
                 mix(irisTexel(q+vec2(0.,pixelStep.y)),irisTexel(q+pixelStep),f.x),f.y);
    }
    vec3 gazed(vec3 base){
      vec2 offset=uv.x*width<eyeSplit?offsets.xy:offsets.zw;
      if(offset.x==0.0&&offset.y==0.0)return base;
      vec2 source=uv-offset/vec2(width,height);
      vec3 own=texture2D(ownership,uv).rgb,shifted=texture2D(ownership,source).rgb;
      vec4 iris=irisSample(source);
      // Unsupported original material is retained only where the translated
      // iris needs it. Vacated iris pixels must always reveal the sclera.
      // No threshold switches whole pixels between moved and stationary eyes.
      float missing=clamp(shifted.g-iris.a,0.,1.);
      vec3 clean=texture2D(sclera,uv).rgb;
      vec3 color=iris.rgb+base*missing+clean*max(0.,1.-iris.a-missing);
      return mix(base,color,own.r);
    }
    float decode(vec2 bytes){return dot(bytes*255.0,vec2(256.0,1.0))/65535.0*(height-1.0);}
    vec3 referenceInterior(vec2 p,vec2 gazeOffset){
      vec3 base=texture2D(original,p).rgb;
      if(gazeOffset.x==0.&&gazeOffset.y==0.)return base;
      vec2 shifted=p-gazeOffset/vec2(width,height);
      vec3 own=texture2D(ownership,shifted).rgb;vec4 iris=irisSample(shifted);
      float missing=clamp(own.g-iris.a,0.,1.);
      return iris.rgb+base*missing+texture2D(sclera,p).rgb*max(0.,1.-iris.a-missing);
    }
    float upperMap(float y,float edge,float sourceEdge,float band){return y>=edge-band?y+sourceEdge-edge:y*(sourceEdge-band)/max(edge-band,1.0);}
    float lowerMap(float y,float edge,float sourceEdge,float band){float bottom=height-1.0;return y<=edge+band?y+sourceEdge-edge:bottom+(y-bottom)*(sourceEdge+band-bottom)/min(edge+band-bottom,-1.0);}
    void main(){
      if(referenceMode){
        float x=(floor(uv.x*width)+.5)/width;
        vec4 a=texture2D(curves,vec2(x,referenceRows.x)),b=texture2D(curves,vec2(x,referenceRows.y));
        vec4 sa=texture2D(curves,vec2(x,referenceRows.x+1./52.)),sb=texture2D(curves,vec2(x,referenceRows.y+1./52.));
        if(sa.b<.5&&sb.b<.5){gl_FragColor=vec4(0.);return;}
        float upper=mix(decode(a.rg),decode(b.rg),referenceFraction),lower=mix(decode(a.ba),decode(b.ba),referenceFraction);
        float seam=mix(decode(sa.rg),decode(sb.rg),referenceFraction),y=uv.y*height-.5;
        float openCoverage=clamp(min(y+.5,lower)-max(y-.5,upper),0.,1.);
        float visible=clamp(min(y+.5,mix(lower,seam,amount))-max(y-.5,mix(upper,seam,amount)),0.,1.);
        vec2 shift=uv.x*width<eyeSplit?irisOffsets.xy:irisOffsets.zw;
        vec2 gazeOffset=uv.x*width<eyeSplit?offsets.xy:offsets.zw;
        vec2 source=uv-shift/vec2(width,height);
        vec3 interior=referenceInterior(source,gazeOffset);
        float cu=mix(upper,seam,amount),cl=mix(lower,seam,amount);
        float band=min(lashBand,min(min(upper,seam),min(height-1.-lower,height-1.-seam))*.8);
        float sourceY=y<cu?min(upperMap(y,cu,upper,band),upper-.5):max(lowerMap(y,cl,lower,band),lower+.5);
        float originalSeam=decode(texture2D(curves,vec2(x,1.5/52.)).rg);
        float closedY=y<cu?upperMap(y,cu,originalSeam,band):lowerMap(y,cl,originalSeam,band);
        vec2 faceUV=(vec2(uv.x*width-.5,sourceY)+faceOffset+.5)/faceSize;
        vec3 tissue=mix(texture2D(referenceA,faceUV).rgb,texture2D(referenceB,faceUV).rgb,referenceFraction);
        vec3 closed=texture2D(closedEye,vec2(source.x,(closedY+.5)/height)).rgb;
        vec3 skin=mix(tissue,closed,amount),color=mix(skin,interior,visible);
        float alpha=max(openCoverage,texture2D(mask,uv).r);
        gl_FragColor=vec4(color,alpha);return;
      }
      vec3 base=texture2D(original,uv).rgb;vec3 target=texture2D(closedEye,uv).rgb;
      float alpha=texture2D(mask,uv).r;
      if(amount==1.0){gl_FragColor=vec4(mix(base,target,alpha),1);return;}
      vec3 eyeColor=gazed(base);
      if(amount==0.0){gl_FragColor=vec4(eyeColor,1);return;}
      float x=(floor(uv.x*width)+0.5)/width;
      vec4 c=texture2D(curves,vec2(x,0.25)),s=texture2D(curves,vec2(x,0.75));
      vec3 color=mix(base,target,amount);
      if(s.b>0.5){
        float a=decode(c.rg),b=decode(c.ba),seam=decode(s.rg),upper=mix(a,seam,amount),lower=mix(b,seam,amount);
        float band=min(lashBand,min(min(a,seam),min(height-1.0-b,height-1.0-seam))*0.8);
        float y=uv.y*height-0.5;
        float sourceY=y<upper?min(upperMap(y,upper,a,band),a-0.5):max(lowerMap(y,lower,b,band),b+0.5);
        float closedY=y<upper?upperMap(y,upper,seam,band):lowerMap(y,lower,seam,band);
        vec3 skin=mix(texture2D(original,vec2(uv.x,(sourceY+0.5)/height)).rgb,texture2D(closedEye,vec2(uv.x,(closedY+0.5)/height)).rgb,amount);
        float coverage=clamp(min(y+0.5,lower)-max(y-0.5,upper),0.0,1.0);
        color=mix(skin,eyeColor,coverage);
      }
      gl_FragColor=vec4(mix(base,color,alpha),1);
    }`
  const renderer = gpu(w, h, vertex, fragment, unavailable)
  const { gl, program } = renderer
  renderer.attribute('point', new Float32Array([-1, -1, 1, -1, -1, 1, -1, 1, 1, -1, 1, 1]), 2)
  renderer.texture(0, 'original', opened)
  renderer.texture(1, 'closedEye', closed)
  renderer.texture(2, 'mask', mask)
  const data = new Uint8Array(w * 2 * 4)
  const pack = (value: number, offset: number) => {
    const n = Math.round(Math.max(0, Math.min(1, value / (h - 1))) * 65535)
    data[offset] = n >> 8
    data[offset + 1] = n & 255
  }
  let run = 0,
    longest = 0
  curves.forEach((c, x) => {
    pack(c[0]!, x * 4)
    pack(c[1]!, x * 4 + 2)
    pack(c[2]!, (w + x) * 4)
    data[(w + x) * 4 + 2] = c[3]! > 0 ? 255 : 0
    data[(w + x) * 4 + 3] = 255
    run = c[3]! > 0 ? run + 1 : 0
    longest = Math.max(longest, run)
  })
  const neutralCurves = renderer.texture(3, 'curves', { data, width: w, height: 2 }, true)
  renderer.texture(4, 'sclera', gaze?.images.sclera ?? opened)
  renderer.texture(
    5,
    'ownership',
    gaze?.images.ownership ?? { data: new Uint8Array([0, 0, 0, 255]), width: 1, height: 1 },
  )
  gl.uniform1f(
    gl.getUniformLocation(program, 'eyeSplit'),
    gaze ? (gaze.package.eyes[0]!.center[0]! + gaze.package.eyes[1]!.center[0]!) / 2 : w / 2,
  )
  const offsetLocation = gl.getUniformLocation(program, 'offsets')
  gl.uniform1f(gl.getUniformLocation(program, 'width'), w)
  gl.uniform1f(gl.getUniformLocation(program, 'height'), h)
  gl.uniform1f(gl.getUniformLocation(program, 'lashBand'), Math.max(2, longest * 0.12))
  const location = gl.getUniformLocation(program, 'amount')
  const modeLocation = gl.getUniformLocation(program, 'referenceMode')
  let referenceCurves: WebGLTexture | undefined
  if (reference && referenceFrames) {
    const bytes = new Uint8Array(w * 52 * 4)
    bytes.set(data)
    const store = (value: number, offset: number) => {
      const packed = Math.round(Math.max(0, Math.min(1, value / (h - 1))) * 65535)
      bytes[offset] = packed >> 8
      bytes[offset + 1] = packed & 255
    }
    reference.eyes.profiles.forEach((profile, index) =>
      profile.curves.forEach((curve, x) => {
        const first = ((index * 2 + 2) * w + x) * 4,
          second = first + w * 4
        store(curve[0]!, first)
        store(curve[1]!, first + 2)
        store(curve[2]!, second)
        bytes[second + 2] = curve[3]! > 0 ? 255 : 0
        bytes[second + 3] = 255
      }),
    )
    referenceCurves = renderer.texture(3, 'curves', { data: bytes, width: w, height: 52 }, true)
    gl.uniform2f(
      gl.getUniformLocation(program, 'faceOffset'),
      reference.eyes.crop.x - reference.crop.x,
      reference.eyes.crop.y - reference.crop.y,
    )
    gl.uniform2f(
      gl.getUniformLocation(program, 'faceSize'),
      reference.crop.width,
      reference.crop.height,
    )
  }
  const referenceTextures = referenceFrames?.map((image) =>
    renderer.texture(6, 'referenceA', image),
  )
  if (!referenceTextures) {
    const empty = { data: new Uint8Array([0, 0, 0, 0]), width: 1, height: 1 }
    renderer.texture(6, 'referenceA', empty, true)
    renderer.texture(7, 'referenceB', empty, true)
  }
  // Sampler units must differ even in old packages: WebGL validates all linked
  // sampler uniforms, including uniforms behind a disabled expression branch.
  gl.uniform1i(gl.getUniformLocation(program, 'referenceA'), 6)
  gl.uniform1i(gl.getUniformLocation(program, 'referenceB'), 7)
  return {
    canvas: renderer.canvas,
    dispose: renderer.dispose,
    draw(
      amount: number,
      offsets: readonly GazeOffset[] = [
        [0, 0],
        [0, 0],
      ],
    ) {
      gl.activeTexture(gl.TEXTURE3)
      gl.bindTexture(gl.TEXTURE_2D, neutralCurves)
      gl.uniform1i(modeLocation, 0)
      gl.uniform4f(offsetLocation, offsets[0]![0], offsets[0]![1], offsets[1]![0], offsets[1]![1])
      gl.uniform1f(location, Math.max(0, Math.min(1, amount)))
      gl.drawArrays(gl.TRIANGLES, 0, 6)
    },
    drawReference(amount: number, closure: number, offsets: readonly GazeOffset[]) {
      if (!reference || !referenceTextures || !referenceCurves) return
      const level = Math.max(0, Math.min(1, amount)) * 24
      const lo = Math.floor(level),
        hi = Math.min(24, lo + 1),
        fraction = level - lo
      for (const [unit, texture] of [
        [6, referenceTextures[lo]!],
        [7, referenceTextures[hi]!],
      ] as const) {
        gl.activeTexture(gl.TEXTURE0 + unit)
        gl.bindTexture(gl.TEXTURE_2D, texture)
      }
      gl.activeTexture(gl.TEXTURE3)
      gl.bindTexture(gl.TEXTURE_2D, referenceCurves)
      gl.uniform2f(
        gl.getUniformLocation(program, 'referenceRows'),
        (lo * 2 + 2.5) / 52,
        (hi * 2 + 2.5) / 52,
      )
      const first = reference.eyes.profiles[lo]!.iris_offsets.flat(),
        second = reference.eyes.profiles[hi]!.iris_offsets.flat()
      const shift = first.map((v, i) => v * (1 - fraction) + second[i]! * fraction)
      gl.uniform1i(modeLocation, 1)
      gl.uniform1f(gl.getUniformLocation(program, 'referenceFraction'), fraction)
      gl.uniform4f(
        gl.getUniformLocation(program, 'irisOffsets'),
        shift[0]!,
        shift[1]!,
        shift[2]!,
        shift[3]!,
      )
      gl.uniform4f(offsetLocation, offsets[0]![0], offsets[0]![1], offsets[1]![0], offsets[1]![1])
      gl.uniform1f(location, Math.max(0, Math.min(1, closure)))
      gl.drawArrays(gl.TRIANGLES, 0, 6)
    },
  }
}
