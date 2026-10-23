"""Export cached, reviewed local poses. Never contacts Gemini."""
import argparse
import json
import shutil
from importlib.metadata import version
from hashlib import sha256
from pathlib import Path
import cv2
import numpy as np
from PIL import Image, ImageDraw, ImageFilter
from app.preparation.faces import PreparationError
from app.preparation.mesh import make_mesh, INNER
from app.preparation.mouth import export_lossless_pair, make_mouth_mask, transform_points
from app.preparation.oral import polygon_mask, remove_teeth, reference_row, add_dentition, cavity_mask, reviewed_dental_masks, dental_bite_stop
from app.preparation.dentition import prepare_rows
from app.preparation.dental_selection import load_originals,select_reference,save_selection_evidence,mask_preview
from app.preparation.argon_masks import load_argon_masks
from app.preparation.jaw import calibrate_jaw
from app.workflows.visemes.viseme_trial import ROOT, write_report


def prepare(reference: Path, batch: Path, review_path: Path, output: Path):
    review=json.loads(review_path.read_text(encoding="utf-8"))
    if review.get("owner_review",{}).get("status")=="rejected":
        raise PreparationError("The owner rejected this dental composition. Inspect its cached evidence; it cannot be installed as the current trial.")
    history=output.parent/"preparation-history"
    history.mkdir(parents=True,exist_ok=True)
    if (output/"review.json").exists():
        previous=(output/"review.json").read_bytes()
        (history/f"review-{sha256(previous).hexdigest()}.json").write_bytes(previous)
    a_report=json.loads((reference/"run.json").read_text(encoding='utf-8'))
    if review["source_sha256"]!=a_report["source_sha256"]: raise PreparationError("Review masks belong to another character.")
    base=Image.open(reference/"base.png").convert("RGB")
    base_points=np.array(json.loads((reference/"base-landmarks.json").read_text(encoding='utf-8'))["landmarks_px"])
    runs={"A":reference,**{p:batch/p.lower() for p in review["poses"] if p not in {"Rest","A"}}}
    reports={p:json.loads((path/"run.json").read_text(encoding='utf-8')) for p,path in runs.items()}
    for report in reports.values():
        if report["source_sha256"]!=review["source_sha256"]: raise PreparationError("Mixed source images in package.")
        if "overlay" not in report: raise PreparationError("A pose has no valid raw composition; inspect its run record.")
    masks={p:Image.open(runs[p]/"mouth-mask.png").convert("L") for p in runs}
    masks["Rest"]=make_mouth_mask(base.size,base_points,base_points)
    boxes=[m.getbbox() for m in masks.values()]
    left,top=min(b[0] for b in boxes),min(b[1] for b in boxes)
    right,bottom=max(b[2] for b in boxes),max(b[3] for b in boxes)
    bounds=(left,top,right,bottom); size=(right-left,bottom-top)
    origin=review["coordinates"]["annotation_origin"]
    offset=(origin[0]-left,origin[1]-top)
    textures={"Rest":base.crop(bounds)}; points={"Rest":base_points[:,:2]-[left,top]}
    for pose,path in runs.items():
        r=reports[pose]; raw=Image.open(path/r["overlay"]["file"]).convert("RGB")
        full=base.copy(); full.paste(raw,(r["overlay"]["x"],r["overlay"]["y"]))
        textures[pose]=full.crop(bounds)
        landmarks=np.array(json.loads((path/"variant-landmarks.json").read_text(encoding='utf-8'))["landmarks_px"])
        points[pose]=transform_points(landmarks,np.array(r["metrics"]["transform"]))-[left,top]
    reference_cfg=review["reference"]
    for pose in review['poses'].values():
        pose['dental_inset_px']=reference_cfg.get('dental_inset_px',0)
    dental_provenance=None
    selection=None
    argon_masks=None
    if reference_cfg.get("automatic_original"):
        settings_path=ROOT/reference_cfg["automatic_original"]
        settings=json.loads(settings_path.read_text(encoding='utf-8'))
        original_images,original_points,original_provenance=load_originals(reference,batch,base.size,bounds,review['poses'])
        if any(p['source_sha256']!=review['source_sha256'] for p in original_provenance.values()):
            raise PreparationError('Original dental candidates do not belong to this character.')
        selection,proposals=select_reference(original_images,original_points,settings)
        selection.update({'settings':settings,'settings_sha256':sha256(settings_path.read_bytes()).hexdigest(),
                          'inputs':original_provenance,'crop':{'x':left,'y':top,'width':size[0],'height':size[1]}})
        selected=selection['selected_pose']
        if selected is None:
            save_selection_evidence(output/'selection',original_images,proposals,selection)
            raise PreparationError('No unambiguous dental reference passed the gates. Inspect selection/selection.json; no automatic fallback or provider request.')
        upper_mask,lower_mask=proposals[selected]
        automatic_masks=(upper_mask.copy(),lower_mask.copy())
        if reference_cfg.get('argon_original_masks'):
            argon_settings_path=ROOT/reference_cfg['argon_original_masks']
            argon_masks,argon_provenance=load_argon_masks(ROOT,json.loads(argon_settings_path.read_text(encoding='utf-8')),
                selected,review['source_sha256'],original_provenance[selected]['raw_sha256'],selection['crop'],output/'argon-masks')
            upper_mask,lower_mask=argon_masks['upper_teeth'],argon_masks['lower_teeth']
            selection['mask_review']={key:argon_provenance[key] for key in ('version','reviewer','reviewed_at','status','notes','method')}
            selection['mask_review']['contours_sha256']=argon_provenance['contours_sha256']
            selection['mask_review']['config_sha256']=sha256(argon_settings_path.read_bytes()).hexdigest()
        elif reference_cfg.get('reviewed_original_masks'):
            mask_review_path=ROOT/reference_cfg['reviewed_original_masks']
            mask_review=json.loads(mask_review_path.read_text(encoding='utf-8'))
            upper_mask,lower_mask=reviewed_dental_masks(mask_review,selected,review['source_sha256'],original_provenance[selected]['raw_sha256'],selection['crop'])
            selection['mask_review']={key:mask_review[key] for key in ('version','reviewer','reviewed_at','status','notes')}
            selection['mask_review'].update({'method':'reviewed contours','config_sha256':sha256(mask_review_path.read_bytes()).hexdigest()})
        feather=reference_cfg.get('mask_feather_px',0.35)
        # Argon sees enamel beyond MediaPipe's approximate inner lip ring.
        # Retain that extraction; each target lip aperture still occludes it.
        clip=np.full((size[1],size[0]),255,dtype=np.uint8) if argon_masks else np.asarray(cavity_mask(size,original_points[selected]))
        upper_mask=Image.fromarray(np.minimum(np.asarray(upper_mask.filter(ImageFilter.GaussianBlur(feather))),clip))
        lower_mask=Image.fromarray(np.minimum(np.asarray(lower_mask.filter(ImageFilter.GaussianBlur(feather))),clip))
        upper=reference_row(original_images[selected],upper_mask)
        lower=reference_row(original_images[selected],lower_mask)
        # Closing-lip landmarks can pass above the fixed upper enamel. Derive a
        # bite stop from the extracted rows so lower teeth never cross the uppers.
        minimum_gap=settings['minimum_dental_gap_ratio']*selection['candidates'][selected]['mouth_width_px']
        bite_stop,gap_evidence=dental_bite_stop(upper_mask,lower_mask,minimum_gap)
        selection['jaw_motion']={'method':'lower-inner-lip offset with per-column extracted-row bite stop',
                                 'minimum_translation_y':bite_stop,'minimum_gap_px':round(minimum_gap,4),
                                 'column_profile':gap_evidence,'poses':{}}
        for pose in review['poses']:
            proposed=int(round(points[pose][14,1]-points[selected][14,1]))
            applied=max(bite_stop,proposed)
            review['poses'][pose]['lower_translation']=[0,applied]
            selection['jaw_motion']['poses'][pose]={'landmark_offset_y':proposed,'applied_offset_y':applied,'bite_stop_applied':applied!=proposed}
        if reference_cfg.get('jaw_calibration'):
            jaw_path=ROOT/reference_cfg['jaw_calibration']
            jaw_settings=json.loads(jaw_path.read_text(encoding='utf-8'))
            translations,jaw_evidence=calibrate_jaw(points,selected,upper_mask,lower_mask,
                selection['candidates'][selected]['mouth_width_px'],jaw_settings)
            jaw_evidence['settings']=jaw_settings
            jaw_evidence['settings_sha256']=sha256(jaw_path.read_bytes()).hexdigest()
            selection['jaw_motion']=jaw_evidence
            for pose,translation in translations.items():review['poses'][pose]['lower_translation']=translation
        save_selection_evidence(output/'selection',original_images,proposals,selection)
        dental_provenance={'kind':'automatically-selected-original','source_pose':selected,
                           'raw_sha256':original_provenance[selected]['raw_sha256'],
                           'settings_sha256':selection['settings_sha256'],'method':selection['method'],
                           'mask_status':'reviewed contours; owner approval pending' if selection.get('mask_review') else 'automatic proposal; assistant inspection only; owner approval pending'}
        if argon_masks:
            dental_provenance['mask_status']='owner-selected Argon contour candidate; composition review pending'
    elif reference_cfg.get("dental_plate"):
        calibration_path=ROOT/reference_cfg["dental_plate"]
        calibration=json.loads(calibration_path.read_text(encoding='utf-8'))
        raw_path=ROOT/calibration["raw"]; request_path=ROOT/calibration["request_record"]
        request=json.loads(request_path.read_text(encoding='utf-8'))
        raw_hash=sha256(raw_path.read_bytes()).hexdigest()
        if request["source_sha256"]!=review["source_sha256"] or request["raw_sha256"]!=raw_hash:
            raise PreparationError("Dental plate provenance does not match this character.")
        upper,lower,dental_evidence=prepare_rows(Image.open(raw_path),calibration,size,offset)
        upper_mask,lower_mask=upper.getchannel("A"),lower.getchannel("A")
        dental_provenance={"kind":"complete-synthetic-plate", "raw_sha256":raw_hash,
                           "request_sha256":sha256(request_path.read_bytes()).hexdigest(),
                           "calibration_sha256":sha256(calibration_path.read_bytes()).hexdigest(),
                           "prompt_sha256":request["prompt_sha256"],"model":request["model"]}
    else:
        upper_mask=polygon_mask(size,[reference_cfg["upper"]],offset,reference_cfg["feather_px"])
        lower_mask=polygon_mask(size,[reference_cfg["lower"]],offset,reference_cfg["feather_px"])
        refinement=reference_cfg.get("color_refinement")
        if refinement:
            red,green,blue=np.asarray(textures["A"],dtype=float).transpose(2,0,1)
            weight=np.clip(np.minimum(np.minimum((green/(red+1)-refinement["minimum_g_over_r"])/refinement["ratio_ramp"],
                                                (blue/(red+1)-refinement["minimum_b_over_r"])/refinement["ratio_ramp"]),
                                     (red-refinement["minimum_red"])/refinement["red_ramp"]),0,1)
            lower_mask=Image.fromarray(np.rint(np.asarray(lower_mask)*weight).astype(np.uint8))
        upper=reference_row(textures["A"],upper_mask); lower=reference_row(textures["A"],lower_mask)
    output.mkdir(parents=True,exist_ok=True)
    export_lossless_pair(base,output,"base")
    export_lossless_pair(upper,output,"upper-teeth"); export_lossless_pair(lower,output,"lower-teeth")
    upper_mask.save(output/"upper-mask.png"); lower_mask.save(output/"lower-mask.png")
    if dental_provenance and selection is None:
        write_report(output/"dental-calibration.json",{"provenance":dental_provenance,"config":calibration,"rows":dental_evidence})
    if selection is not None:
        export_lossless_pair(original_images[selected],output,'dental-source')
        preview=mask_preview(original_images[selected],upper_mask,lower_mask)
        if argon_masks:
            amount=np.asarray(argon_masks['tongue_visible'],dtype=float)[...,None]/255*0.45
            preview=Image.fromarray(np.rint(np.asarray(preview)*(1-amount)+np.array([245,165,50])*amount).astype(np.uint8))
        export_lossless_pair(preview,output,'dental-mask')
        export_lossless_pair(mask_preview(original_images[selected],*automatic_masks),output,'dental-proposal')
        if selection.get('mask_review') and not argon_masks:
            write_report(output/'dental-masks-reviewed.json',mask_review)
    manifest={"schema":"syntelle-viseme-package/1","pipeline_version":"oral-layers/1","renderer_version":"pose-morph/3",
              "source_sha256":review["source_sha256"],"review_sha256":sha256(review_path.read_bytes()).hexdigest(),
              "canvas":{"width":base.width,"height":base.height},"crop":{"x":left,"y":top,"width":size[0],"height":size[1]},
              "detail":{"x":max(0,left-120),"y":max(0,top-138),"width":size[0]+240,"height":size[1]+270},
              "reference":{"upper":"upper-teeth.webp","lower":"lower-teeth.webp"},"poses":{},"pairs":{}}
    if dental_provenance:
        manifest["pipeline_version"]="oral-layers/5" if selection else "oral-layers/2"
        if argon_masks:
            manifest['pipeline_version']='oral-layers/6-argon'
            manifest['reference']['argon_occlusion']=argon_provenance['occlusion']
        if reference_cfg.get('jaw_calibration'):manifest['pipeline_version']='oral-layers/7-jaw'
        manifest["reference"]["provenance"]=dental_provenance
    if selection:
        manifest['reference']['selection']={k:selection[k] for k in ('method','selected_pose','status','ranking','score_margin','candidates','mask_method','new_provider_requests','jaw_motion')}
        if selection.get('mask_review'): manifest['reference']['selection']['mask_review']=selection['mask_review']
        manifest['reference']['selection'].update({'original_preview':'dental-source.webp','mask_preview':'dental-mask.webp',
                                                  'review_status':('Reviewed contours for this character; owner approval pending. Automatic proposal was incomplete.' if selection.get('mask_review') else 'Automatic proposal; internally inspected for this character. Owner approval pending.')})
        if argon_masks:
            manifest['reference']['selection']['review_status']='Argon contour trial. Teeth occlude tongue; full tongue evidence retained. Existing pose-specific interiors remain. Composition approval pending.'
    manifest["export_settings"]={"png":"lossless", "webp":{"lossless":True,"exact":True,"method":6},
                                 "runtime_teeth":"fixed canonical rows; lower row rigid translation only"}
    manifest["dependencies"]={name:version(name) for name in ("Pillow","numpy","mediapipe")}
    manifest["dependencies"]["opencv"]=cv2.__version__
    manifest["algorithm_config"]={
        "alignment":{"anchor_ids":[33,133,263,362,168,6,197,195,10,70,300,127,356],"maximum_rmse_eye_ratio":0.035,"scale_range":[0.9,1.1],"maximum_rotation_degrees":5},
        "mask":{"dilation_mouth_width_ratio":0.12,"feather_mouth_width_ratio":0.025,"nose_margin_ratio":0.07,"chin_margin_ratio":0.08},
        "mesh":{"minimum_closed_inner_gap_px":0.75,"fixed_boundary_anchors":8,"diagonal_samples":101,"quadratic_area_check":True},
        "render":{"inner_edge_feather_px":0.8,"teeth_scale":1,"upper_translation":[0,0],"lower_motion":"rigid translation only","interior":"pose-specific soft tissue; dentition excluded from mesh",
                  "dental_inset_px":reference_cfg.get('dental_inset_px',0),
                  "dental_visibility":"opaque enamel with geometric shutter exposure between tooth-visible and toothless open poses; closed poses use lip occlusion",
                  "dental_shutter_margin_px":2},
        "inpainting":review["inpainting"]}
    manifest["source_run_hashes"]={pose:sha256((path/"run.json").read_bytes()).hexdigest() for pose,path in runs.items()}
    source_files=["services/processor/app/workflows/visemes/prepare_viseme_package.py","services/processor/app/preparation/oral.py",
                  "services/processor/app/preparation/mesh.py","services/processor/app/preparation/mouth.py",
                  "services/processor/app/preparation/gemini.py","services/processor/app/preparation/faces.py",
                  "services/processor/app/preparation/dentition.py","services/processor/app/workflows/dentition/dentition_trial.py",
                  "services/processor/app/preparation/dental_selection.py","services/processor/app/workflows/dentition/select_dental_reference.py",
                  "services/processor/app/workflows/visemes/viseme_trial.py","services/processor/app/workflows/visemes/viseme_batch.py",
                  "apps/web/src/experiments/viseme-a/poseMorph.ts","apps/web/src/experiments/viseme-a/VisemeStage.vue",
                  "apps/web/src/experiments/viseme-a/dentalOcclusion.ts",
                  "apps/web/src/experiments/viseme-a/VisemeTrial.vue","apps/web/src/experiments/viseme-a/usePosePackage.ts",
                  "apps/web/src/experiments/viseme-a/useVisemePlayback.ts",
                  "docs/prompts/nano-banana-source.txt"]
    if selection:
        source_files.extend([reference_cfg['automatic_original'],
                             'apps/web/src/experiments/viseme-a/DentalReferenceReview.vue',
                             'apps/web/src/experiments/viseme-a/poseTypes.ts'])
        if reference_cfg.get('reviewed_original_masks'): source_files.append(reference_cfg['reviewed_original_masks'])
        if argon_masks: source_files.extend([reference_cfg['argon_original_masks'],'services/processor/app/preparation/argon_masks.py'])
        if reference_cfg.get('jaw_calibration'):source_files.extend([reference_cfg['jaw_calibration'],'services/processor/app/preparation/jaw.py'])
    elif dental_provenance:
        source_files.extend([reference_cfg["dental_plate"],"docs/prompts/dentition-v1.txt"])
    manifest["pipeline_source_hashes"]={name:sha256((ROOT/name).read_bytes()).hexdigest() for name in source_files}
    if selection: manifest['selection_sha256']=sha256(json.dumps(selection,sort_keys=True).encode()).hexdigest()
    manifest["build_id"]=sha256((manifest["review_sha256"]+json.dumps(manifest["pipeline_source_hashes"],sort_keys=True)
                                +json.dumps(manifest['source_run_hashes'],sort_keys=True)+manifest.get('selection_sha256','')).encode()).hexdigest()
    sheet=Image.new("RGB",(1200,3*310),"#FAFAF9"); drawing=ImageDraw.Draw(sheet)
    for index,(pose,settings) in enumerate(review["poses"].items()):
        slug=pose.lower(); directory=output/slug; directory.mkdir(exist_ok=True)
        removal=polygon_mask(size,settings["remove"],offset)
        tongue=polygon_mask(size,settings["tongue"],offset)
        cavity=cavity_mask(size,points[pose])
        if argon_masks and pose==selected:
            removal=Image.fromarray(np.maximum(np.asarray(argon_masks['upper_teeth']),np.asarray(argon_masks['lower_teeth'])))
            tongue=argon_masks['tongue_visible']
        removal=Image.fromarray(np.minimum(np.asarray(removal),np.asarray(cavity)))
        cfg=review["inpainting"]
        surface,actual_mask=remove_teeth(textures[pose],removal,tongue,cfg["radius_px"],cfg["dilate_px"],cavity)
        # Narrow all-teeth poses provide no dark neighboring pixels for Telea.
        # Use the recorded A cavity sample only inside the reviewed removal,
        # never in protected tongue pixels or on the lips.
        if pose in cfg.get("dark_fill_poses",[]):
            sample_x,sample_y=cfg["cavity_reference_point"]
            color=textures["A"].getpixel((sample_x+offset[0],sample_y+offset[1]))
            fill=Image.fromarray(np.where((np.asarray(actual_mask)>0)&(np.asarray(tongue)==0),255,0).astype(np.uint8))
            surface.paste(Image.new("RGB",size,color),(0,0),fill)
        actual_mask.save(directory/"teeth-removal.png"); tongue.save(directory/"tongue-protection.png")
        protected=np.asarray(tongue)>0
        tongue_identical=bool(np.array_equal(np.asarray(surface)[protected],np.asarray(textures[pose])[protected]))
        if not tongue_identical: raise PreparationError("Inpainting altered protected tongue tissue.")
        corrected=add_dentition(surface,upper,lower,points[pose],settings)
        if pose=="Rest" or (pose=="A" and reference_cfg.get("preserve_a_endpoint",True)):
            corrected=textures[pose].copy()
        alpha=masks[pose].crop(bounds)
        endpoint=corrected.convert("RGBA"); endpoint.putalpha(alpha)
        export_lossless_pair(endpoint,directory,"endpoint")
        export_lossless_pair(surface,directory,"surface")
        alpha.save(directory/"alpha.png")
        full=base.copy(); full.paste(corrected,bounds[:2],alpha)
        original=np.asarray(base); composed=np.asarray(full); outside=np.asarray(masks[pose])==0
        changed=int(np.count_nonzero(np.any(original[outside]!=composed[outside],axis=1)))
        if changed: raise PreparationError("Changed pixels outside mouth mask.")
        full.save(directory/"composed.png")
        entry={"id":pose,"status":settings["status"],"notes":settings["notes"],"upper_visible":settings["upper_visible"],
               "lower_visible":settings["lower_visible"],"lower_translation":settings["lower_translation"],
               "dental_inset_px":settings['dental_inset_px'],
               "endpoint":f"{slug}/endpoint.webp","surface":f"{slug}/surface.webp","mask":f"{slug}/alpha.png",
               "points":points[pose].round(6).tolist(),"inner":points[pose][INNER].round(6).tolist(),
               "changed_pixels_outside_mask":changed,"tongue_pixels_preserved":tongue_identical,"tongue_protected_pixel_count":int(protected.sum()),
               "raw_sha256":reports.get(pose,{}).get("generation",{}).get("raw_sha256"),"opencv_version":cv2.__version__}
        manifest["poses"][pose]=entry
        x=(index%3)*400; y=(index//3)*310
        frame=full.crop(manifest_rect(manifest["detail"])).resize((400,280))
        sheet.paste(frame,(x,y+30)); drawing.text((x+8,y+4),f"{pose} - {settings['status']}",fill="#24212B",font_size=17)
    manifest['playback_policy']=review.get('playback_policy','accepted-poses-only')
    if manifest['playback_policy'] not in {'accepted-poses-only','geometry-only-experiment'}:
        raise PreparationError('Unknown playback policy.')
    # Owner-operated experiments may compare phonetic candidates in motion.
    # Geometry remains a separate mandatory gate, not an approval label.
    for source in points:
        for target in points:
            if source==target: continue
            key=f"{source}-{target}"
            try:
                mesh=make_mesh(points[source],points[target],(0,0,*size))
                accepted=manifest['playback_policy']=='geometry-only-experiment' or all(review["poses"][p]["status"]=="accepted" for p in (source,target))
                manifest["pairs"][key]={"safe":accepted,"reason":None if accepted else "One pose requires visual review.","mesh":mesh}
            except PreparationError as exc:
                manifest["pairs"][key]={"safe":False,"reason":str(exc),"mesh":None}
    (output/"review.json").write_bytes(review_path.read_bytes())
    write_report(output/'resolved-review.json',review)
    sheet.save(output/"contact-sheet.png")
    manifest["artifact_hashes"]={str(path.relative_to(output)).replace("\\","/"):sha256(path.read_bytes()).hexdigest()
                                for path in output.rglob("*") if path.is_file() and path.name!="package.json"}
    write_report(output/"package.json",manifest)
    snapshot=history/manifest["build_id"]
    snapshot.mkdir(exist_ok=True)
    shutil.copyfile(output/"package.json",snapshot/"package.json")
    shutil.copyfile(review_path,snapshot/"review.json")
    for name in source_files:
        saved=snapshot/"source"/name; saved.parent.mkdir(parents=True,exist_ok=True); shutil.copyfile(ROOT/name,saved)
        if sha256(saved.read_bytes()).hexdigest()!=manifest["pipeline_source_hashes"][name]:
            raise PreparationError("Pipeline source changed during preparation; inspect the build before publishing it.")
    print(f"Prepared {len(points)} local poses, {sum(p['safe'] for p in manifest['pairs'].values())} safe directed transitions. No provider request.")
    return manifest


def manifest_rect(rect):
    return (rect["x"],rect["y"],rect["x"]+rect["width"],rect["y"]+rect["height"])


def install_trial(output, manifest):
    """Copy only explicit runtime resources; never raw provider inputs or secrets."""
    target=ROOT/"apps/web/src/experiments/viseme-a/poses"
    names={"base.webp","upper-teeth.webp","lower-teeth.webp","package.json"}
    selection=manifest['reference'].get('selection')
    if selection: names.update(selection[k] for k in ('original_preview','mask_preview'))
    for pose in manifest["poses"].values():
        names.update(pose[key] for key in ("endpoint","surface","mask"))
    for name in sorted(names):
        path=Path(name)
        if path.is_absolute() or ".." in path.parts: raise PreparationError("Invalid runtime resource path.")
        saved=target/path; saved.parent.mkdir(parents=True,exist_ok=True)
        shutil.copyfile(output/path,saved)
    print("Development trial assets installed; no public generation endpoint.")


if __name__=="__main__":
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--reference",type=Path,default=ROOT/".tools/viseme-a-05")
    parser.add_argument("--batch",type=Path,default=ROOT/".tools/visemes-01")
    parser.add_argument("--review",type=Path,default=ROOT/"docs/characters/syntelle/viseme-auto-review.json")
    parser.add_argument("--output",type=Path,default=ROOT/".tools/dental-selection-01/package-current")
    parser.add_argument("--install-trial",action="store_true")
    args=parser.parse_args(); manifest=prepare(args.reference,args.batch,args.review,args.output)
    if args.install_trial: install_trial(args.output,manifest)
