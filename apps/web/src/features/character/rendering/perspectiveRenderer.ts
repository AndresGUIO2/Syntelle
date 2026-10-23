import { gpu } from './gl'
import { decodeProjection } from '../types/projectionTypes'
import type { MotionPackage } from '../types/motionTypes'
import type { Rect } from '../types/oralTypes'
import type { HeadPose } from '../animation/headMotion'
import { createBreathingTrack } from '../animation/breathing'
import { neckTransforms, type NeckMode } from './neckDeformation'
import type { ExpressionPackage, ExpressionState } from '../types/expressionTypes'

export function createPerspectiveRenderer(
  foreground: HTMLImageElement,
  data: MotionPackage,
  geometry: ArrayBuffer,
  unavailable: () => void,
  maximumSide = 1024,
  neckMode: NeckMode = 'full',
  expressions?: { package: ExpressionPackage; field: HTMLImageElement; smile?: HTMLImageElement },
) {
  const rig = data.projection!
  if (!rig?.validation.valid || rig.source_sha256 !== data.original_sha256)
    throw Error('Invalid RGB-D projection package')
  const { vertices, indices, fringe, articulation } = decodeProjection(geometry, rig)
  const neck = rig.articulation
  const articulated = Boolean(neck && neckMode !== 'previous')
  const volume = articulated && neckMode === 'full'
  const size = data.canvas,
    camera = rig.camera
  const scale = Math.min(1, maximumSide / Math.max(size.width, size.height))
  const breathTrack = data.breathing
    ? createBreathingTrack(data.animation_seed ?? 42, data.breathing)
    : null
  const renderer = gpu(
    Math.round(size.width * scale),
    Math.round(size.height * scale),
    `precision highp float;
    attribute vec3 point;attribute vec2 weights;attribute vec3 skinWeights,breathWeights;
    uniform mat4 neckBones[3];uniform bool articulated,volumeBreath;
    uniform vec2 chestCenter;uniform float forwardGain,lateralGain;
    uniform vec2 size,center,pivotPx;uniform vec3 pivot,angles;
    uniform float focal,distance,nearPlane,farPlane,breath;uniform bool referenceView;
    varying vec3 sourceProjection;varying float headWeight,torsoWeight;
    void main(){
      float sourceQ=distance-point.z;
      vec2 original=point.xy*size;
      vec3 world=vec3((original-center)*sourceQ/focal,point.z);
      vec3 p=world-pivot;
      float cy=cos(angles.x),sy=sin(angles.x),cp=cos(angles.y),sp=sin(angles.y),cr=cos(angles.z),sr=sin(angles.z);
      vec3 a=vec3(cy*p.x+sy*p.z,p.y,-sy*p.x+cy*p.z);
      vec3 b=vec3(a.x,cp*a.y-sp*a.z,sp*a.y+cp*a.z);
      vec3 rotated=vec3(cr*b.x-sr*b.y,sr*b.x+cr*b.y,b.z);
      world+=weights.x*(rotated-p);
      if(articulated){
        vec3 sourceWorld=vec3((original-center)*sourceQ/focal,point.z);
        world=sourceWorld+skinWeights.x*((neckBones[0]*vec4(sourceWorld,1.)).xyz-sourceWorld)
          +skinWeights.y*((neckBones[1]*vec4(sourceWorld,1.)).xyz-sourceWorld)
          +skinWeights.z*((neckBones[2]*vec4(sourceWorld,1.)).xyz-sourceWorld);
        if(volumeBreath){
          float forward=-breath*breathWeights.z*forwardGain;
          world.xy+=vec2(-breath*breathWeights.x*lateralGain,breath*breathWeights.y)*sourceQ/focal;
          world.xy-=forward*(chestCenter-center)/focal;
          world.z+=forward;
        }
      }
      float q=distance-world.z;
      vec2 pixel=world.xy*focal/q+center;
      if(!articulated&&(referenceView||weights.x==0.))pixel=original;
      if(!volumeBreath){
        if(referenceView)pixel=original;
        pixel.y+=weights.y*breath;
        pixel.x+=weights.y*(original.x-pivotPx.x)*breath/size.y*.25;
      }
      float clipZ=(farPlane+nearPlane)/(farPlane-nearPlane)*q-2.*farPlane*nearPlane/(farPlane-nearPlane);
      float targetW=articulated?q/distance:(referenceView?1.:mix(1.,q/distance,weights.x));
      float sourceW=articulated?sourceQ/distance:(referenceView?1.:mix(1.,sourceQ/distance,weights.x));
      gl_Position=vec4((pixel/size*vec2(2.,-2.)+vec2(-1.,1.))*targetW,clipZ/q*targetW,targetW);
      sourceProjection=vec3(point.xy*sourceW,sourceW);
      headWeight=articulated?dot(skinWeights,vec3(1.)):weights.x;
      torsoWeight=volumeBreath?breathWeights.z:weights.y;
    }`,
    `precision highp float;uniform sampler2D subject;uniform bool boundaryPass,referenceView;
    uniform sampler2D expressionField,smileMaterial;uniform vec4 expressionRect;uniform vec2 expressionAmount,sourceSize;uniform float expressionScale;
    uniform vec2 outputSize;uniform float breath;varying vec3 sourceProjection;varying float headWeight,torsoWeight;
    vec2 displacement(vec2 p){
      vec2 position=(p-expressionRect.xy+.5)/expressionRect.zw;
      if(position.x<0.||position.x>1.||position.y<0.||position.y>1.)return vec2(0.);
      vec3 delta=texture2D(expressionField,position).rgb*255.-128.;
      return vec2(delta.r*expressionAmount.x,delta.g*expressionAmount.x+delta.b*expressionAmount.y)*expressionScale;
    }
    void main(){
      vec2 uv=sourceProjection.xy/sourceProjection.z;
      if((referenceView&&(breath==0.||torsoWeight==0.))||(headWeight==0.&&breath==0.))uv=vec2(gl_FragCoord.x/outputSize.x,1.-gl_FragCoord.y/outputSize.y);
      if(expressionAmount.x>0.||expressionAmount.y>0.){
        vec2 target=uv*sourceSize-.5,p=target;
        for(int i=0;i<5;i++)p=target-displacement(p);
        uv=(p+.5)/sourceSize;
      }
      vec4 color=texture2D(subject,uv);
        if(expressionAmount.x>0.){
          vec2 local=(uv*sourceSize-expressionRect.xy)/expressionRect.zw;
          if(local.x>=0.&&local.x<=1.&&local.y>=0.&&local.y<=1.){
            vec4 reference=texture2D(smileMaterial,local);
            // Both materials live in the same neutral domain. Geometry moves
            // the full lip seam; the reference supplies coherent cheek detail.
            color.rgb=mix(color.rgb,reference.rgb/max(reference.a,.00001)*color.a,reference.a*expressionAmount.x);
          }
        }
      if(boundaryPass){if(color.a>=.995||color.a<=.001)discard;}
      else if(color.a<.995)discard;
      gl_FragColor=color;
    }`,
    unavailable,
    true,
  )
  const { gl, program } = renderer
  try {
    const points = new Float32Array(rig.geometry.vertices * 3),
      weights = new Float32Array(rig.geometry.vertices * 2)
    for (let i = 0; i < rig.geometry.vertices; i++) {
      points.set(vertices.subarray(i * 5, i * 5 + 3), i * 3)
      weights.set(vertices.subarray(i * 5 + 3, i * 5 + 5), i * 2)
    }
    renderer.attribute('point', points, 3)
    renderer.attribute('weights', weights, 2)
    if (articulation) {
      const skin = new Float32Array(rig.geometry.vertices * 3),
        breathing = new Float32Array(rig.geometry.vertices * 3)
      for (let i = 0; i < rig.geometry.vertices; i++) {
        skin.set(articulation.subarray(i * 6, i * 6 + 3), i * 3)
        breathing.set(articulation.subarray(i * 6 + 3, i * 6 + 6), i * 3)
      }
      renderer.attribute('skinWeights', skin, 3)
      renderer.attribute('breathWeights', breathing, 3)
    }
    const opaqueBuffer = renderer.indices(indices),
      boundaryBuffer = renderer.indices(fringe, true)
    const texture = renderer.texture(0, 'subject', foreground)
    renderer.texture(
      1,
      'expressionField',
      expressions?.field ?? { data: new Uint8Array([128, 128, 128, 255]), width: 1, height: 1 },
    )
    renderer.texture(
      2,
      'smileMaterial',
      expressions?.smile ?? { data: new Uint8Array([0, 0, 0, 0]), width: 1, height: 1 },
    )
    const uniform = (name: string) => gl.getUniformLocation(program, name)
    const expressionAmount = uniform('expressionAmount')
    const crop = expressions?.package.crop ?? { x: 0, y: 0, width: 1, height: 1 }
    gl.uniform4f(uniform('expressionRect'), crop.x, crop.y, crop.width, crop.height)
    gl.uniform2f(uniform('sourceSize'), size.width, size.height)
    gl.uniform1f(uniform('expressionScale'), expressions?.package.units_per_byte ?? 0)
    gl.uniform2f(uniform('size'), size.width, size.height)
    gl.uniform2f(uniform('outputSize'), renderer.canvas.width, renderer.canvas.height)
    gl.uniform2f(uniform('center'), camera.center[0]!, camera.center[1]!)
    gl.uniform2f(uniform('pivotPx'), camera.pivot_px[0]!, camera.pivot_px[1]!)
    gl.uniform3f(uniform('pivot'), rig.pivot[0]!, rig.pivot[1]!, rig.pivot[2]!)
    gl.uniform1i(uniform('articulated'), articulated ? 1 : 0)
    gl.uniform1i(uniform('volumeBreath'), volume ? 1 : 0)
    gl.uniform2f(
      uniform('chestCenter'),
      neck?.chest_center_px[0] ?? 0,
      neck?.chest_center_px[1] ?? 0,
    )
    gl.uniform1f(uniform('forwardGain'), neck?.breath_forward_gain ?? 0)
    gl.uniform1f(uniform('lateralGain'), neck?.breath_lateral_gain ?? 0)
    const boneUniform = uniform('neckBones[0]')
    for (const [key, value] of Object.entries({
      focal: camera.focal,
      distance: camera.distance,
      nearPlane: camera.near,
      farPlane: camera.far,
    }))
      gl.uniform1f(uniform(key), value)
    const angles = uniform('angles'),
      breath = uniform('breath'),
      pass = uniform('boundaryPass'),
      reference = uniform('referenceView')
    const stats = {
      fullUploads: 1,
      patchUploads: 0,
      patchPixels: 0,
      vertexUploads: 1,
      indexUploads: 0,
      expressionUploads: expressions ? 1 : 0,
    }
    // Only the small transparent boundary needs ordering; vertex geometry stays immutable.
    const boundary = Array.from({ length: fringe.length / 3 }, (_, triangle) => {
      const ids = Array.from(fringe.subarray(triangle * 3, triangle * 3 + 3))
      const weighted = [0, 0, 0]
      let z = 0
      const moments = new Float64Array(9)
      let breathDepth = 0
      for (const id of ids) {
        const i = id * 5,
          q = camera.distance - vertices[i + 2]!
        const world = [
          ((vertices[i]! * size.width - camera.center[0]!) * q) / camera.focal,
          ((vertices[i + 1]! * size.height - camera.center[1]!) * q) / camera.focal,
          vertices[i + 2]!,
        ]
        z += world[2]! / 3
        for (let axis = 0; axis < 3; axis++)
          weighted[axis]! += ((world[axis]! - rig.pivot[axis]!) * vertices[i + 3]!) / 3
        if (neck && articulation) {
          for (let j = 0; j < 3; j++)
            for (let axis = 0; axis < 3; axis++)
              moments[j * 3 + axis]! +=
                ((world[axis]! - neck.pivots[j]![axis]!) * articulation[id * 6 + j]!) / 3
          breathDepth += articulation[id * 6 + 5]! / 3
        }
      }
      return { ids, weighted, z, depth: 0, moments, breathDepth }
    })
    const sorted = new Uint16Array(fringe.length)
    let lastSort = ''
    return {
      canvas: renderer.canvas,
      dispose: renderer.dispose,
      stats,
      setClip(rect: Rect) {
        // Keep two raster pixels around the visible rectangle for the final
        // bilinear canvas sampling. Coordinates remain in the original view.
        const sx = renderer.canvas.width / size.width,
          sy = renderer.canvas.height / size.height
        const left = Math.max(0, Math.floor(rect.x * sx) - 2),
          top = Math.max(0, Math.floor(rect.y * sy) - 2),
          right = Math.min(renderer.canvas.width, Math.ceil((rect.x + rect.width) * sx) + 2),
          bottom = Math.min(renderer.canvas.height, Math.ceil((rect.y + rect.height) * sy) + 2)
        gl.enable(gl.SCISSOR_TEST)
        gl.scissor(left, renderer.canvas.height - bottom, right - left, bottom - top)
      },
      setExpression(value: ExpressionState) {
        const clamp = (v: number) => (Number.isFinite(v) ? Math.max(0, Math.min(1, v)) : 0)
        gl.uniform2f(
          expressionAmount,
          expressions?.smile ? clamp(value.smile) : 0,
          expressions ? clamp(value.brows) : 0,
        )
      },
      update(rect: Rect, image: HTMLCanvasElement) {
        gl.activeTexture(gl.TEXTURE0)
        gl.bindTexture(gl.TEXTURE_2D, texture)
        gl.pixelStorei(gl.UNPACK_PREMULTIPLY_ALPHA_WEBGL, true)
        gl.texSubImage2D(gl.TEXTURE_2D, 0, rect.x, rect.y, gl.RGBA, gl.UNSIGNED_BYTE, image)
        stats.patchUploads++
        stats.patchPixels += rect.width * rect.height
      },
      draw(milliseconds: number, moving = true, pose: HeadPose = { yaw: 0, pitch: 0, roll: 0 }) {
        const limits = rig.bounds
        const degrees = moving
          ? [
              Math.max(-limits.yaw, Math.min(limits.yaw, pose.yaw)),
              Math.max(-limits.pitch, Math.min(limits.pitch, pose.pitch)),
              Math.max(-limits.roll, Math.min(limits.roll, pose.roll)),
            ]
          : [0, 0, 0]
        const [yaw, pitch, roll] = degrees.map((v) => (v * Math.PI) / 180)
        gl.uniform3f(angles, yaw!, pitch!, roll!)
        const amount = moving
          ? (breathTrack?.sample(milliseconds) ??
              Math.sin((milliseconds / data.motion.breath_period_ms) * Math.PI * 2)) *
            data.anatomy.face_height *
            data.motion.breath_face_height_fraction
          : 0
        gl.uniform1f(breath, amount)
        const transforms = articulated ? neckTransforms(neck!, degrees) : null
        if (transforms) gl.uniformMatrix4fv(boneUniform, false, transforms)
        gl.uniform1i(reference, degrees.every((v) => v === 0) ? 1 : 0)
        const key =
          degrees.map((v) => v.toFixed(1)).join(',') + (volume ? `:${amount.toFixed(1)}` : '')
        if (key !== lastSort) {
          const sy = Math.sin(yaw!),
            cy = Math.cos(yaw!),
            sp = Math.sin(pitch!),
            cp = Math.cos(pitch!)
          for (const t of boundary)
            if (transforms) {
              let z = t.z
              for (let j = 0; j < 3; j++) {
                const offset = j * 16,
                  m = j * 3
                z +=
                  transforms[offset + 2]! * t.moments[m]! +
                  transforms[offset + 6]! * t.moments[m + 1]! +
                  transforms[offset + 10]! * t.moments[m + 2]! -
                  t.moments[m + 2]!
              }
              if (volume) z -= amount * t.breathDepth * neck!.breath_forward_gain
              t.depth = camera.distance - z
            } else
              t.depth =
                camera.distance -
                (t.z +
                  sp * t.weighted[1]! +
                  cp * (-sy * t.weighted[0]! + cy * t.weighted[2]!) -
                  t.weighted[2]!)
          boundary.sort((a, b) => b.depth - a.depth)
          boundary.forEach((t, i) => sorted.set(t.ids, i * 3))
          gl.bindBuffer(gl.ELEMENT_ARRAY_BUFFER, boundaryBuffer)
          gl.bufferSubData(gl.ELEMENT_ARRAY_BUFFER, 0, sorted)
          stats.indexUploads++
          lastSort = key
        }
        gl.enable(gl.DEPTH_TEST)
        gl.depthFunc(gl.LEQUAL)
        gl.depthMask(true)
        gl.disable(gl.BLEND)
        gl.clearColor(0, 0, 0, 0)
        gl.clearDepth(1)
        gl.clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)
        gl.uniform1i(pass, 0)
        gl.bindBuffer(gl.ELEMENT_ARRAY_BUFFER, opaqueBuffer)
        gl.drawElements(gl.TRIANGLES, indices.length, gl.UNSIGNED_SHORT, 0)
        gl.uniform1i(pass, 1)
        gl.depthMask(false)
        gl.enable(gl.BLEND)
        gl.blendFunc(gl.ONE, gl.ONE_MINUS_SRC_ALPHA)
        gl.bindBuffer(gl.ELEMENT_ARRAY_BUFFER, boundaryBuffer)
        gl.drawElements(gl.TRIANGLES, fringe.length, gl.UNSIGNED_SHORT, 0)
        gl.depthMask(true)
        gl.disable(gl.BLEND)
      },
    }
  } catch (error) {
    renderer.dispose()
    throw error
  }
}
