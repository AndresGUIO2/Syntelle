import { gpu } from './gl'
import type { Rect } from '../types/oralTypes'
import type { MotionPackage } from '../types/motionTypes'
import type { HeadPose } from '../animation/headMotion'
import { createBreathingTrack } from '../animation/breathing'

export function createCharacterRenderer(
  foreground: HTMLImageElement,
  depth: HTMLImageElement,
  data: MotionPackage,
  unavailable: () => void,
  maximumSide = 1024,
) {
  const size = data.canvas,
    scale = Math.min(1, maximumSide / Math.max(size.width, size.height))
  const vertex = `precision highp float;attribute vec3 point;attribute vec2 weights;uniform bool anatomical;uniform vec2 size,pivot;uniform vec3 angles;uniform vec2 anatomy;uniform float breath;varying vec2 uv;
    void main(){
      uv=point.xy;vec2 pixel=uv*size;vec2 original=pixel;
      float head=anatomical?weights.x:1.0-smoothstep(pivot.y,pivot.y+anatomy.y*0.55,pixel.y);
      vec3 p=vec3(pixel-pivot,anatomical?point.z:(point.z-0.5)*anatomy.x*0.35);
      float cy=cos(angles.x),sy=sin(angles.x),cp=cos(angles.y),sp=sin(angles.y),cr=cos(angles.z),sr=sin(angles.z);
      vec3 a=vec3(cy*p.x+sy*p.z,p.y,-sy*p.x+cy*p.z);
      vec3 b=vec3(a.x,cp*a.y-sp*a.z,sp*a.y+cp*a.z);
      vec2 rotated=vec2(cr*b.x-sr*b.y,sr*b.x+cr*b.y);
      pixel+=head*(rotated-p.xy);
      float torso=anatomical?weights.y:smoothstep(pivot.y+anatomy.y*0.2,pivot.y+anatomy.y*0.65,pixel.y)*(1.0-smoothstep(size.y-anatomy.y*0.15,size.y,pixel.y));
      pixel.y+=torso*breath;pixel.x+=torso*((anatomical?original.x:pixel.x)-pivot.x)*breath/max(anatomical?size.y:anatomy.y,1.0)*0.25;
      gl_Position=vec4(pixel/size*vec2(2,-2)+vec2(-1,1),0,1);
    }`
  const fragment =
    'precision highp float;uniform sampler2D subject;varying vec2 uv;void main(){gl_FragColor=texture2D(subject,uv);}'
  const renderer = gpu(
    Math.round(size.width * scale),
    Math.round(size.height * scale),
    vertex,
    fragment,
    unavailable,
    true,
  )
  const { gl, program } = renderer
  const n = data.rig?.grid ?? 64,
    c = document.createElement('canvas')
  c.width = c.height = n + 1
  const ctx = c.getContext('2d')!
  ctx.drawImage(depth, 0, 0, n + 1, n + 1)
  const samples = ctx.getImageData(0, 0, n + 1, n + 1).data
  const vertices: number[] = [],
    weights: number[] = []
  if (
    data.rig &&
    (!data.rig.geometry_validation.valid || data.rig.source_sha256 !== data.original_sha256)
  )
    throw Error('Invalid anatomical motion package')
  const point = (x: number, y: number) => {
    const v = data.rig?.vertices[y * (n + 1) + x]
    vertices.push(x / n, y / n, v?.[2] ?? samples[(y * (n + 1) + x) * 4]! / 255)
    weights.push(v?.[3] ?? 0, v?.[4] ?? 0)
  }
  for (let y = 0; y < n; y++)
    for (let x = 0; x < n; x++) {
      point(x, y)
      point(x + 1, y)
      point(x, y + 1)
      point(x, y + 1)
      point(x + 1, y)
      point(x + 1, y + 1)
    }
  renderer.attribute('point', new Float32Array(vertices), 3)
  renderer.attribute('weights', new Float32Array(weights), 2)
  gl.uniform1i(gl.getUniformLocation(program, 'anatomical'), data.rig ? 1 : 0)
  const texture = renderer.texture(0, 'subject', foreground)
  gl.uniform2f(gl.getUniformLocation(program, 'size'), size.width, size.height)
  gl.uniform2f(
    gl.getUniformLocation(program, 'pivot'),
    data.rig?.pivot[0] ?? data.anatomy.head_pivot[0]!,
    data.rig?.pivot[1] ?? data.anatomy.chin_y,
  )
  gl.uniform2f(
    gl.getUniformLocation(program, 'anatomy'),
    data.anatomy.face_width,
    data.anatomy.face_height,
  )
  const angles = gl.getUniformLocation(program, 'angles'),
    breathing = gl.getUniformLocation(program, 'breath')
  const stats = { fullUploads: 1, patchUploads: 0, patchPixels: 0 }
  const breathTrack = data.breathing
    ? createBreathingTrack(
        data.animation_seed ?? parseInt(data.original_sha256.slice(0, 8), 16),
        data.breathing,
      )
    : null
  return {
    canvas: renderer.canvas,
    dispose: renderer.dispose,
    stats,
    update(rect: Rect, image: HTMLCanvasElement) {
      gl.activeTexture(gl.TEXTURE0)
      gl.bindTexture(gl.TEXTURE_2D, texture)
      gl.pixelStorei(gl.UNPACK_PREMULTIPLY_ALPHA_WEBGL, true)
      gl.texSubImage2D(gl.TEXTURE_2D, 0, rect.x, rect.y, gl.RGBA, gl.UNSIGNED_BYTE, image)
      stats.patchUploads++
      stats.patchPixels += rect.width * rect.height
    },
    draw(milliseconds: number, moving = true, pose?: HeadPose) {
      const m = data.motion,
        radians = Math.PI / 180
      gl.uniform3f(
        angles,
        moving
          ? (pose?.yaw ?? Math.sin((milliseconds / 8500) * Math.PI * 2) * m.yaw_degrees) * radians
          : 0,
        moving
          ? (pose?.pitch ?? Math.sin((milliseconds / 7100) * Math.PI * 2) * m.pitch_degrees) *
              radians
          : 0,
        moving
          ? (pose?.roll ?? Math.sin((milliseconds / 11500) * Math.PI * 2) * m.roll_degrees) *
              radians
          : 0,
      )
      gl.uniform1f(
        breathing,
        moving
          ? (breathTrack?.sample(milliseconds) ??
              Math.sin((milliseconds / m.breath_period_ms) * Math.PI * 2)) *
              data.anatomy.face_height *
              m.breath_face_height_fraction
          : 0,
      )
      gl.clearColor(0, 0, 0, 0)
      gl.clear(gl.COLOR_BUFFER_BIT)
      gl.drawArrays(gl.TRIANGLES, 0, vertices.length / 3)
    },
  }
}
