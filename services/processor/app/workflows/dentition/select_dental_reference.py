"""Inspect original cached visemes and rank a single dental reference, offline."""
import argparse
from datetime import datetime,timezone
from hashlib import sha256
import json
from pathlib import Path
from PIL import Image
from app.preparation.dental_selection import load_originals,select_reference,save_selection_evidence
from app.workflows.visemes.viseme_trial import ROOT


def inspect(reference,batch,settings_path,output):
    inventory=json.loads((ROOT/'docs/characters/syntelle/viseme-review.json').read_text(encoding='utf-8'))['poses']
    canvas=json.loads((reference/'run.json').read_text(encoding='utf-8'))['canvas']; settings=json.loads(settings_path.read_text(encoding='utf-8'))
    folders=[reference if pose=='A' else batch/pose.lower() for pose in inventory if pose!='Rest']
    boxes=[Image.open(folder/'mouth-mask.png').getbbox() for folder in folders]
    boxes.append(Image.open(reference/'base-mouth-mask.png').getbbox())
    bounds=(min(b[0] for b in boxes),min(b[1] for b in boxes),max(b[2] for b in boxes),max(b[3] for b in boxes))
    crop={'x':bounds[0],'y':bounds[1],'width':bounds[2]-bounds[0],'height':bounds[3]-bounds[1]}
    images,points,provenance=load_originals(reference,batch,(canvas['width'],canvas['height']),bounds,inventory)
    report,masks=select_reference(images,points,settings)
    report.update({'settings':settings,'settings_sha256':sha256(settings_path.read_bytes()).hexdigest(),
                   'inputs':provenance,'crop':crop,'created_at':datetime.now(timezone.utc).isoformat()})
    save_selection_evidence(output,images,masks,report)
    print(json.dumps({'selected_pose':report['selected_pose'],'ranking':report['ranking'],'candidates':report['candidates']},indent=2))
    return report


if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--reference',type=Path,default=ROOT/'.tools/viseme-a-05')
    parser.add_argument('--batch',type=Path,default=ROOT/'.tools/visemes-01')
    parser.add_argument('--settings',type=Path,default=ROOT/'docs/characters/syntelle/dental-selection.json')
    parser.add_argument('--output',type=Path,default=ROOT/'.tools/dental-selection-01')
    args=parser.parse_args();inspect(args.reference,args.batch,args.settings,args.output)
