"""Assemble source-bound speech motion, oral recovery and optional cached views.

This reusable pipeline never requests a provider. Finite reference acquisition
is a separate explicit command with its own PostgreSQL authorization.
"""
import argparse
import json
from pathlib import Path

from app.core.paths import ROOT
from app.preparation.faces import PreparationError
from app.preparation.conversational_parallax import write_json
from app.workflows.motion.prepare_conversational_parallax import prepare as prepare_motion
from app.workflows.motion.prepare_multiview_parallax import prepare as prepare_views
from app.workflows.rendering.export_native import export


def prepare(config, models, output, references=None):
    if output.exists():raise PreparationError('Preserve previous preparations; choose a fresh output directory.')
    specification=json.loads(config.read_bytes())
    if specification.get('version')!='conversational-parallax-input/1':raise PreparationError('Unsupported character performance input.')
    bundle=(ROOT/specification['bundle']).resolve()
    output.mkdir(parents=True)
    (output/'input.json').write_bytes(config.read_bytes())
    try:
        result=prepare_motion(bundle,output/'conversational')
        candidate=Path(result['bundle'])
        reference_result=None
        if references is not None:
            reference_result=prepare_views(candidate,config,references,models,output/'multiview')
            candidate=Path(reference_result['bundle'])
        exported=export(output/'package',bundle=candidate)
        report=dict(version='character-performance-preparation/1',source_sha256=specification['source_sha256'],
            resource_bundle=candidate.as_posix(),native_package=exported['directory'],
            reference_support=reference_result,paid_provider_calls=0,manual_character_overrides=False,
            owner_acceptance=False,arbitrary_uploads_qualified=False)
        write_json(output/'preparation.json',report)
        return report
    except Exception as error:
        write_json(output/'failure.json',dict(status='failed',error=str(error),provider_calls=0))
        raise


if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__)
    for name in ('config','models','output'):parser.add_argument('--'+name,type=Path,required=True)
    parser.add_argument('--references',type=Path)
    args=parser.parse_args()
    result=prepare(args.config.resolve(),args.models.resolve(),args.output.resolve(),args.references.resolve() if args.references else None)
    print(json.dumps(dict(package=result['native_package'],source_sha256=result['source_sha256'],provider_calls=0)))
