"""Add measured lip-contact lighting to cached reference laughter, offline."""
import argparse
from hashlib import sha256
import json
from pathlib import Path
import shutil

from app.preparation.archive import encoded
from app.preparation.faces import PreparationError
from app.preparation.reference_laughter import contact_shadow
from app.workflows.rendering.export_native import resource_paths, validate_bindings


def prepare(bundle: Path, output: Path):
    if output.exists():
        raise PreparationError('Preserve previous evidence; select a fresh output directory.')
    folders,specification=resource_paths(bundle,None)
    filenames={'oral':'package.json','motion':'package.json','gaze':'gaze.json','expressions':'expressions.json'}
    packages={key:json.loads((folders[key]/file).read_bytes()) for key,file in filenames.items()}
    validate_bindings(packages,specification)
    expression=packages['expressions']
    reference=expression.get('reference_laughter')
    if not reference or reference.get('version')!='reference-laughter/1' or not reference['validation']['valid']:
        raise PreparationError('A validated full-face reference is required.')
    shadow=contact_shadow(packages['oral'],reference['oral']['inner'])
    images={p.relative_to(folders['expressions']).as_posix():sha256(p.read_bytes()).hexdigest()
            for p in folders['expressions'].rglob('*') if p.suffix.lower() in ('.png','.jpg','.webp')}
    shutil.copytree(folders['expressions'],output/'expressions')
    reference['oral']['contact_shadow']=shadow
    (output/'expressions/expressions.json').write_bytes(encoded(expression))
    for name,checksum in images.items():
        if sha256((output/'expressions'/name).read_bytes()).hexdigest()!=checksum:
            raise PreparationError('Image bytes changed while adding lighting metadata.')
    result=dict(specification);result['expressions']=(output/'expressions').resolve().as_posix()
    (output/'input-bundle.json').write_bytes(bundle.read_bytes())
    (output/'resource-bundle.json').write_bytes(encoded(result))
    report={'version':'reference-contact-lighting-preparation/1','source_sha256':specification['source_sha256'],
            'input_bundle_sha256':sha256(bundle.read_bytes()).hexdigest(),'image_hashes':images,
            'contact_shadow':shadow,'paid_provider_calls':0,'owner_acceptance':False}
    (output/'preparation.json').write_bytes(encoded(report))
    return report


if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--bundle',type=Path,required=True)
    parser.add_argument('--output',type=Path,required=True)
    args=parser.parse_args()
    report=prepare(args.bundle.resolve(),args.output.resolve())
    print(json.dumps({'images_unchanged':len(report['image_hashes']),'contact_shadow':report['contact_shadow'],'paid_provider_calls':0}))
