"""Material opacity, request bounds, and archived single-character evidence."""
import json
from pathlib import Path
import httpx
import numpy as np
import pytest
from PIL import Image
from app import complete_dentition as completion
from app.preparation.faces import PreparationError
from app.preparation.mesh import make_mesh
from app.preparation.dental_seams import reconcile_completion
from app.prepare_completed_oral import soft_tissue


def test_generated_seam_matches_selected_color_without_replacing_original_pixels():
    raw=np.zeros((50,80,4),np.uint8);raw[10:35,10:70]=[210,195,170,255]
    selected=np.zeros_like(raw);selected[30:35,10:70]=[95,83,72,255]
    before=np.abs(raw[29,30,:3].astype(int)-selected[30,30,:3]).max()
    result,record=reconcile_completion(Image.fromarray(raw),Image.fromarray(selected))
    fixed=np.asarray(result);protected=selected[...,3]>0
    assert before>100
    assert np.abs(fixed[29,30,:3].astype(int)-fixed[30,30,:3]).max()<=1
    assert np.array_equal(fixed[protected,:3],selected[protected,:3])
    assert np.all(fixed[protected,3]==255)
    assert record['generated_seam_pixels']>0


def test_rectangular_tooth_removal_does_not_repaint_lip_outside_aperture():
    yy,xx=np.mgrid[:70,:90];raw=np.stack((180+xx%7,95+yy%5,100+xx%3),axis=-1).astype(np.uint8)
    raw[25:40,20:70]=[40,12,10];raw[27:30,30:60]=[145,125,105]
    raw[35:38,42:48]=[110,45,52]
    opening=np.zeros((70,90),np.uint8);opening[25:40,20:70]=255
    removal=np.zeros_like(opening);removal[27:50,28:62]=255
    tongue=np.zeros_like(opening);tongue[35:38,42:48]=255
    lips,cavity,_=soft_tissue(Image.fromarray(raw),Image.fromarray(removal),Image.fromarray(tongue),Image.fromarray(opening))
    assert np.array_equal(np.asarray(lips)[46:49,30:60],raw[46:49,30:60])
    assert np.array_equal(np.asarray(cavity)[tongue>0],raw[tongue>0])


def test_chroma_matte_preserves_dark_enamel_and_removes_enclosed_key():
    pixels=np.zeros((100,100,3),np.uint8);pixels[:]=[0,255,0]
    pixels[15:85,15:85]=[12,9,8]
    pixels[40:45,40:45]=[0,255,0]
    matte=completion.chroma_matte(Image.fromarray(pixels))
    assert matte[25,25,3]==255
    assert matte[42,42,3]==0
    assert (matte[0,0]==0).all()


def test_rgba_registration_cannot_introduce_green_fringe():
    raw=np.zeros((20,20,4),np.uint8);raw[:,:,:3]=[0,255,0]
    raw[5:15,5:15]=[80,60,50,255]
    warped=completion.warp_rgba(raw,np.array([[1,0,.5],[0,1,.5]],float),(20,20))
    assert np.all(warped[warped[...,3]>0,1]<=60)


@pytest.fixture
def experiment(tmp_path,monkeypatch):
    root=tmp_path/'workspace';root.mkdir();monkeypatch.setattr(completion,'ROOT',root)
    colors=tmp_path/'Colors';colors.mkdir();Image.new('RGB',(64,64),'tan').save(colors/'Syntelle.jpg')
    prompt=root/'prompt.txt';prompt.write_text('Complete the supplied fragments only.');monkeypatch.setattr(completion,'PROMPT',prompt)
    source=root/'source';source.mkdir()
    for name,y in [('upper',15),('lower',40)]:
        raw=np.zeros((64,64,4),np.uint8);raw[y:y+5,20:44]=[100,85,75,255]
        Image.fromarray(raw).save(source/f'{name}-teeth.png')
    Image.new('RGB',(64,64),'tan').save(source/'dental-source.png')
    output=root/'completion';completion.prepare_canvas(source,output)
    monkeypatch.setenv('GEMINI_API_KEY','unit-test-secret')
    return source,output


def test_preparation_reuses_exact_sources_and_rejects_changed_cache(experiment):
    source,output=experiment
    first=(output/'canvas.png').read_bytes();completion.prepare_canvas(source,output)
    assert first==(output/'canvas.png').read_bytes()
    (output/'canvas.png').write_bytes(b'changed')
    with pytest.raises(PreparationError,match='evidence changed'):completion.prepare_canvas(source,output)


def test_failed_request_is_recorded_and_cannot_retry(experiment,monkeypatch):
    _,output=experiment;calls=[]
    class Client:
        def __init__(self,**kwargs):assert kwargs['follow_redirects'] is False
        def __enter__(self):return self
        def __exit__(self,*args):pass
        def post(self,url,**kwargs):
            calls.append(kwargs);return httpx.Response(503,json={'error':'unit-test-secret'})
    monkeypatch.setattr(completion.httpx,'Client',Client)
    with pytest.raises(PreparationError,match='503'):completion.request(output)
    with pytest.raises(PreparationError,match='already attempted'):completion.request(output)
    assert len(calls)==1
    assert 'unit-test-secret' not in (output/'initial/response.json').read_text()
    assert json.loads((output/'initial/run.json').read_text())['retries']==0
    # Both attempts, including a provider error, consume the bounded ledger.
    (output/'correction').mkdir();(output/'correction/run.json').write_text('{}')
    with pytest.raises(PreparationError):completion.request(output,True)


def test_current_completion_restores_selected_material_and_all_pairs_cover_crop():
    directory=completion.OUTPUT
    if not (directory/'package-v2/package.json').exists():pytest.skip('Local character experiment not present')
    record=json.loads((directory/'correction/replay/replay.json').read_text())
    for row in ('upper','lower'):
        source=np.asarray(Image.open(completion.SOURCE/f'{row}-teeth.png'))
        final=np.asarray(Image.open(directory/f'correction/replay/{row}.png'))
        selected=source[...,3]>=128
        assert np.array_equal(source[selected,:3],final[selected,:3])
        assert np.all(final[selected,3]==255)
        assert record['rows'][row]['completed_pixels']>record['rows'][row]['protected_pixels']
    package=json.loads((directory/'package-v2/package.json').read_text())
    assert len(package['pairs'])==81
    assert all(pair['safe'] and pair['mesh']['minimum_signed_double_area']>0 for pair in package['pairs'].values())
    assert package['poses']['FV']['points'][14][1]==pytest.approx(84.5)
    assert len(list(directory.glob('*/run.json')))==2
