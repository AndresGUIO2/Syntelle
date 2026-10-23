"""Report corpus coverage without generating images or claiming missing evidence."""
import argparse
from hashlib import sha256
import json
from pathlib import Path

from app.workflows.oral.prepare_semantic_oral import algorithm_hash
from app.preparation.faces import PreparationError
from app.workflows.visemes.viseme_trial import write_report


def evaluate(corpus, output):
    spec=json.loads(corpus.read_text());current,_=algorithm_hash();cases=[];seen=set();issues=[]
    for item in spec['cases']:
        if item.get('input'):
            path=(corpus.parent/item['input']).resolve();source=json.loads(path.read_text())
            original=(path.parent/source['original']).resolve();expected=source['original_sha256']
        else:
            original=(corpus.parent/item['original']).resolve();expected=item['original_sha256']
        digest=sha256(original.read_bytes()).hexdigest()
        if digest!=expected:raise PreparationError('Corpus original bytes changed.')
        if digest in seen:raise PreparationError('The same portrait cannot count in both development and held-out sets.')
        seen.add(digest)
        package=(corpus.parent/item['package']).resolve() if item.get('package') else None
        result=json.loads(package.read_text()) if package and package.exists() else {}
        preflight=json.loads((corpus.parent/item['preflight']).read_text()) if item.get('preflight') else {}
        record={**item,'original_sha256':digest,'prepared':bool(result),
                'same_frozen_algorithm':result.get('algorithm_sha256')==current,
                'ready':result.get('ready',False), 'visual_acceptance':item.get('visual_acceptance',False),
                'preflight':preflight}
        cases.append(record)
    counts={split:sum(c['split']==split for c in cases) for split in ('development','held-out')}
    if counts!={'development':3,'held-out':2}:issues.append('Requires three development and two distinct held-out portraits.')
    if {c['kind'] for c in cases}!={'photograph','semirealistic-render'}:issues.append('Both photographs and semirealistic renders are required.')
    if not all(c['prepared'] and c['ready'] and c['same_frozen_algorithm'] and c['visual_acceptance'] for c in cases):
        issues.append('Every case must pass preparation, frozen-algorithm and independent visual acceptance checks.')
    report={'schema':'syntelle-oral-benchmark/1','algorithm_sha256':current,'cases':cases,'counts':counts,
            'passed':not issues,'issues':issues,'new_provider_requests':0,'manual_mask_repairs':False}
    write_report(output,report);print(json.dumps({'passed':report['passed'],'counts':counts,'issues':issues}))
    return report


if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--corpus',type=Path,required=True);parser.add_argument('--output',type=Path,required=True)
    args=parser.parse_args()
    try:evaluate(args.corpus.resolve(),args.output.resolve())
    except (ValueError,PreparationError) as error:raise SystemExit(str(error)) from None
