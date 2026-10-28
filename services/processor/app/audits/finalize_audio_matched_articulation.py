"""Archive the offline articulation candidate and byte-verify PostgreSQL restoration."""
import json
import shutil
import subprocess
from hashlib import sha256
from pathlib import Path
from app.core.paths import ROOT
from app.preparation.archive import Archive, encoded


def main():
    folder=ROOT/'.tools/jaw-articulation-108';parent=ROOT/'.tools/emma-presence-107';db=Archive()
    profile=json.loads((folder/'profile/profile.json').read_bytes());old_profile=json.loads((parent/'profile-v3/profile.json').read_bytes())
    assert profile['laughter']==old_profile['laughter'] and profile['voice']==old_profile['voice']
    assert (folder/'profile/voice-capabilities.json').read_bytes()==(parent/'profile-v3/voice-capabilities.json').read_bytes()
    package=folder/'prepared-final/package';manifest=json.loads((package/'manifest.json').read_bytes());original=ROOT/'.tools/paired-parallax-100/prepared-v2/package';old=json.loads((original/'manifest.json').read_bytes())
    assert profile['package_hash']==manifest['package_hash']
    assert manifest['textures']==old['textures'] and manifest['geometry']==old['geometry']
    for name in ['laugh-en-US-review','laugh-es-CO-review','installed-en-US','installed-es-CO','installed-mobile-en-US','webkit-laughter']:
        assert json.loads((folder/(name+'.json')).read_bytes())['passed'],name
    curves={}
    for lang in ['en-US','es-CO']:
        curves[lang]=json.loads((folder/(lang+'-waveform.json')).read_bytes());assert curves[lang]['one_frame_peak_gate']
    parity=json.loads((folder/'cpu-cuda-release/packed720.json').read_bytes());assert len(parity)==19 and min(x['psnr_db'] for x in parity)>=45
    shutil.copytree(parent/'audible-review',folder/'original-audible-review',dirs_exist_ok=True)
    for name in ['owner-audible-confirmation.json','complete-operations.json']:
        shutil.copy2(parent/name,folder/('original-'+name))
    shutil.copy2(ROOT/'.tools/foreground-11/prepared/syntelle/base-landmarks.json',folder/'inputs/base-landmarks.json')
    source_image=Path('E:/Carpetas/Documentos/01-MisProyectos/Colors/Syntelle.jpg')
    assert source_image.exists();shutil.copy2(source_image,folder/'inputs/immutable-original.jpg')
    unchanged={}
    def record(path):
        before=parent/'final-source'/path;current=ROOT/path
        assert before.read_bytes()==current.read_bytes(),path
        unchanged[path.as_posix()]=sha256(current.read_bytes()).hexdigest()
    for path in (parent/'final-source/apps/web/src').rglob('*'):
        if path.is_file():record(path.relative_to(parent/'final-source'))
    record(Path('services/render-worker/internal/nativeanimation/laughter_bridge.go'))
    before=(parent/'final-source/services/render-worker/internal/nativeanimation/speech.go').read_text();current=(ROOT/'services/render-worker/internal/nativeanimation/speech.go').read_text()
    def function(text,name):
        start=text.index('func '+name+'(');end=text.find('\nfunc ',start+1)
        return text[start:] if end<0 else text[start:end]
    assert function(before,'MouthAt')==function(current,'MouthAt')
    source=folder/'final-source'
    for name in ['packages/render-contract','services/render-worker/internal','services/render-worker/cmd','services/render-worker/scripts','services/processor/app','services/processor/tests','specs/004-coordinated-performance']:
        shutil.copytree(ROOT/name,source/name,dirs_exist_ok=True,ignore=shutil.ignore_patterns('__pycache__','*.pyc','.venv','node_modules'))
    for name in ['AGENTS.md','compose.yaml','docs/README.md','docs/experiments/motion/audio-matched-articulation.md','services/render-worker/go.mod','services/render-worker/go.sum','services/processor/pyproject.toml']:
        target=source/name;target.parent.mkdir(parents=True,exist_ok=True);shutil.copy2(ROOT/name,target)
    binaries=folder/'binaries';binaries.mkdir(exist_ok=True);images={}
    for role,image in [('candidate','syntelle-render-native:jaw-articulation-108'),('phase107','syntelle-render-native:before-jaw-articulation-108')]:
        images[role]=subprocess.check_output(['docker','image','inspect',image,'--format','{{.Id}}'],text=True).strip()
        with (binaries/role).open('wb') as out:subprocess.run(['docker','run','--rm','--pull','never','--network','none','--entrypoint','cat',image,'/usr/local/bin/syntelle-render-worker'],stdout=out,check=True)
    status=json.loads(subprocess.check_output(['docker','inspect','syntelle-render-native-local'],text=True))[0]
    assert status['Config']['Image']=='syntelle-render-native:jaw-articulation-108' and status['State']['Running']
    report={'version':'audio-matched-articulation/1','parent_snapshot':766,'initial_candidate_snapshot':767,'base_resource_snapshot':730,'package_hash':manifest['package_hash'],'images':images,'installed':True,'provider_calls':0,'new_models':0,'audio_rate':1,'head_disabled_open_A_chin_travel_720_px':3.150,'geometry':manifest['packages']['motion']['mandible']['validation'],'peak_alignment_ms':{key:value['peak_offsets_ms'] for key,value in curves.items()},'parity_minimum_psnr_db':min(x['psnr_db'] for x in parity),'approved_audio_sha256':{key:value['audio_sha256'] for key,value in curves.items()},'audio_approval':'unchanged original accepted recordings','unchanged_sources':unchanged,'mouth_timing_function_unchanged':True,'performance_qualification':'pending uncontended capacity; unrelated GPU utilization 100 percent','owner_naturalness':'pending','public_upload_validation':False}
    (folder/'activation.json').write_bytes(encoded(report))
    result=db.archive_tree('audio-matched-articulation-108',folder,report)
    restored=ROOT/('.tools/jaw-articulation-108-restored-'+str(result['snapshot_id']));count=db.restore(result['snapshot_id'],restored)
    for path in restored.rglob('*'):
        if path.is_file():assert path.read_bytes()==(folder/path.relative_to(restored)).read_bytes()
    receipt={**result,'restored_files':count,'all_hashes_match':True,'parent_snapshot':766,'initial_candidate_snapshot':767,'base_resource_snapshot':730,'package_hash':manifest['package_hash']}
    db.event('audio-matched-articulation-108','restoration-verified',receipt)
    (ROOT/'.tools/jaw-articulation-108-archive.json').write_bytes(encoded(receipt));print(json.dumps(receipt))

if __name__=='__main__':main()
