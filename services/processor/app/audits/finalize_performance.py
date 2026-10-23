"""Archive contextual-performance evidence and verify byte-exact restoration."""
from hashlib import sha256
import json

from app.core.paths import ROOT
from app.preparation.archive import Archive, encoded
from app.preparation.faces import PreparationError

IDENTITY='authorized-character-performance-2026-10-20'


def run():
    evidence=ROOT/'.tools/character-performance-34'
    source=evidence/'sources'
    paths=[
        'services/api/cmd/api/main.go','services/api/cmd/performance-probe/main.go',
        'services/api/internal/conversation/performance.go','services/api/internal/conversation/performance_test.go',
        'services/api/internal/conversation/gemini.go','services/api/internal/conversation/provider.go',
        'services/api/internal/conversation/service.go','services/api/internal/speech/speech.go',
        'services/processor/app/workflows/motion/probe_performance.py',
        'services/processor/app/workflows/motion/prepare_laughter_reference.py',
        'services/processor/app/workflows/motion/bind_laughter_reference.py',
        'services/processor/app/workflows/motion/prepare_smile_reference.py',
        'services/processor/app/audits/finalize_performance.py',
        'apps/web/src/features/character/animation/performance.ts',
        'apps/web/src/features/character/animation/attention.ts',
        'apps/web/src/features/character/animation/expressions.ts',
        'apps/web/src/features/character/animation/speechBehavior.ts',
        'apps/web/src/features/character/types/performanceTypes.ts',
        'apps/web/src/features/character/types/laughterTypes.ts',
        'apps/web/src/features/character/types/packageContext.ts',
        'apps/web/src/features/character/speech/speechTimeline.ts',
        'apps/web/src/features/character/composables/useSpeechPlayback.ts',
        'apps/web/src/features/character/composables/useCharacterMotion.ts',
        'apps/web/src/features/character/composables/useMotionAssets.ts',
        'apps/web/src/features/character/composables/useMotionRenderer.ts',
        'apps/web/src/features/character/composables/useExpressions.ts',
        'apps/web/src/features/demo/catalog.ts','apps/web/playwright.config.ts',
        'apps/web/tests/e2e/performance.spec.ts','apps/web/tests/fixtures/performanceHarness.ts',
        'docs/characters/performance-experiment.json','docs/characters/laughter-reference-proposal.json',
        'docs/characters/laughter-reference-prompt.txt','docs/experiments/motion/contextual-performance.md',
        'docs/development/user-demo.md','docs/development/conversation-workspace.md','AGENTS.md',
    ]
    for name in paths:
        target=source/name;target.parent.mkdir(parents=True,exist_ok=True)
        data=(ROOT/name).read_bytes()
        if target.exists() and target.read_bytes()!=data:
            raise PreparationError('Frozen final source differs; preserve it before another audit.')
        target.write_bytes(data)
    db=Archive()
    with db.connect() as conn:
        probes=conn.execute('SELECT request_key,binding,status,request,result,usage,diagnostics,audio FROM syntelle_conversation.operations WHERE kind=%s ORDER BY id',(IDENTITY,)).fetchall()
        images=conn.execute('SELECT request_key,kind,status,binding,metadata,usage FROM requests WHERE budget_id=%s ORDER BY id',(IDENTITY,)).fetchall()
    if len(probes)!=4 or len(images)!=2 or any(row['status']!='received' for row in probes+images):
        raise PreparationError('Actual paid-work records do not match the exhausted bounded experiment.')
    for probe in probes:
        audio=probe.pop('audio')
        probe['audio_sha256']=sha256(bytes(audio)).hexdigest() if audio else None
    usage={'identity':IDENTITY,'limits':{'image':2,'tts':2,'gemini':2,'analysis':0},
           'actual':{'image':len(images),'tts':sum(p['request_key'].startswith('tts-') for p in probes),
                     'gemini':sum(p['request_key'].startswith('gemini-') for p in probes),'analysis':0,'retries':0},
           'probes':probes,'images':images,'facial_laughter_enabled':False,
           'reason':'Actual composition review rejected the dental material seam. No further paid requests.'}
    (evidence/'provider-evidence.json').write_bytes(encoded(usage))
    result=db.archive_tree('character-performance-34/final',evidence,{'owner_visual_acceptance':False,'facial_laughter_enabled':False})
    destination=ROOT/'.tools/character-performance-34-restored'
    restored=db.restore(result['snapshot_id'],destination)
    report={**result,'restored_files':restored,'verified':restored==result['files'],'destination':str(destination)}
    (ROOT/'.tools/character-performance-34-archive.json').write_bytes(encoded(report))
    print(json.dumps(report))


if __name__=='__main__':run()
