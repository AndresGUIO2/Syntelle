"""Bind intentional speaking to an unchanged qualified character package."""
import argparse
from hashlib import sha256
import json
from pathlib import Path
import shutil


def prepare(source: Path, output: Path):
    source=source.resolve();output=output.resolve()
    if output.exists():
        raise ValueError('Preserve existing preparation; select a new output')
    manifest=json.loads((source/'manifest.json').read_bytes())
    previous=manifest['package_hash']
    motion=manifest['packages']['motion']['conversational_motion']
    if motion['version']!='conversational-motion/1':
        raise ValueError('A qualified conversational motion package is required')
    original_version=motion.get('presence_version','')
    motion['presence_version']='conversational-presence/6'
    record={
        'version':'intentional-speaking/1',
        'source_package_hash':previous,
        'previous_presence_version':original_version,
        'presence_version':motion['presence_version'],
        'motion_direction':'semantic-anchors/1',
        'preparation_sha256':sha256(Path(__file__).read_bytes()).hexdigest(),
        'background_offset_bounds_degrees':[.6,.25,.08],
        'semantic_maximum_degrees':[6,2,.7],
        'emphasis_stroke_ms':[180,280],
        'ordinary_stroke_ms':[240,420],
        'release_ms':320,
        'provider_calls':0,
    }
    encoded=json.dumps(record,sort_keys=True,separators=(',',':')).encode()
    manifest['identity_files'].append({'path':'intentional-speaking/1','sha256':sha256(encoded).hexdigest(),'bytes':len(encoded)})
    manifest['package_hash']=sha256(json.dumps(manifest['identity_files'],separators=(',',':'),ensure_ascii=False).encode()).hexdigest()
    shutil.copytree(source,output)
    unchanged=[]
    for original in source.rglob('*'):
        if not original.is_file() or original.name=='manifest.json':continue
        relative=original.relative_to(source)
        content=original.read_bytes()
        if content!=(output/relative).read_bytes():
            raise ValueError('Qualified resource changed: '+relative.as_posix())
        unchanged.append({'file':relative.as_posix(),'sha256':sha256(content).hexdigest()})
    (output/'manifest.json').write_text(json.dumps(manifest,indent=2))
    report={**record,'package_hash':manifest['package_hash'],'unchanged_files':unchanged}
    (output.parent/'intentional-preparation.json').write_text(json.dumps(report,indent=2))
    return report


if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__)
    for name in ('source','output'):parser.add_argument('--'+name,type=Path,required=True)
    args=parser.parse_args();result=prepare(args.source,args.output)
    print(json.dumps({'package_hash':result['package_hash'],'unchanged_files':len(result['unchanged_files']),'provider_calls':0}))
