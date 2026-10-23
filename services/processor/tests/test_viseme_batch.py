import argparse
import json
from hashlib import sha256
from io import BytesIO
from pathlib import Path

import pytest
from PIL import Image

np=pytest.importorskip("numpy")
pytest.importorskip("mediapipe")
from app.preparation.faces import PreparationError
from app.preparation.gemini import VISEMES, build_prompt
from app.preparation.mouth import compose_mouth
from app.preparation.oral import polygon_mask, remove_teeth, reference_row, add_dentition
from app.viseme_batch import batch
from app import viseme_trial
from test_preparation import face

ROOT=Path(__file__).resolve().parents[3]


@pytest.mark.parametrize("viseme,phrase",[("EI","Strong readable E/I"),("O","rounded “O”"),("U","“U” vowel"),("MBP","Closed pressed lips"),("SZ","S/Z sibilant"),("FV","F/V consonant"),("CHSHR","CH, SH, J")])
def test_inventory_preserves_original_descriptions(viseme,phrase):
    source=ROOT/"docs/prompts/nano-banana-source.txt"; before=source.read_bytes()
    prompt=build_prompt(source,viseme)
    assert phrase in prompt and "[VISEME_DESCRIPTION]" not in prompt
    assert source.read_bytes()==before


def test_closed_mbp_and_narrow_fv_do_not_use_a_opening_gate():
    image=Image.new("RGB",(320,400))
    assert compose_mouth(image,image,face(),face(),"MBP").metrics["variant_opening_ratio"]<0.035
    compose_mouth(image,image,face(),face(),"FV")
    with pytest.raises(PreparationError,match="A viseme"):
        compose_mouth(image,image,face(),face(),"A")
    with pytest.raises(PreparationError,match="close the lips"):
        compose_mouth(image,image,face(),face(True),"MBP")


def test_inpainting_preserves_tongue_and_unmasked_pixels():
    texture=Image.new("RGB",(80,60),"#301011")
    from PIL import ImageDraw
    draw=ImageDraw.Draw(texture); draw.rectangle((25,20,55,30),fill="#EEE0D1"); draw.rectangle((30,30,50,45),fill="#A64761")
    removal=polygon_mask(texture.size,[[[20,15],[60,15],[60,47],[20,47]]])
    tongue=polygon_mask(texture.size,[[[30,30],[50,30],[50,45],[30,45]]])
    corrected,mask=remove_teeth(texture,removal,tongue)
    original,result=np.asarray(texture),np.asarray(corrected)
    np.testing.assert_array_equal(original[np.asarray(tongue)>0],result[np.asarray(tongue)>0])
    np.testing.assert_array_equal(original[np.asarray(mask)==0],result[np.asarray(mask)==0])
    assert np.any(original[np.asarray(mask)>0]!=result[np.asarray(mask)>0])


def test_closed_pose_never_exposes_reference_teeth():
    texture=Image.new("RGB",(320,400),"#553322")
    row=reference_row(Image.new("RGB",texture.size,"white"),Image.new("L",texture.size,255))
    actual=add_dentition(texture,row,row,face().points,{"upper_visible":False,"lower_visible":False,"lower_translation":[0,0]})
    np.testing.assert_array_equal(np.asarray(actual),np.asarray(texture))


def test_batch_has_exactly_seven_entries_and_stops_on_provider_failure(tmp_path,monkeypatch):
    from app import viseme_batch
    source=tmp_path/"base.png"; source.write_bytes(b"same-base")
    reference=tmp_path/"reference"; reference.mkdir()
    (reference/"run.json").write_text(json.dumps({"source_sha256":sha256(source.read_bytes()).hexdigest()}))
    calls=[]
    def run(args):
        calls.append((args.viseme,args.input.read_bytes()))
        args.output.mkdir(); failed=args.viseme=="U"
        (args.output/"run.json").write_text(json.dumps({"status":"provider_request_failed" if failed else "ready_for_visual_review"}))
        if failed: raise PreparationError("synthetic provider failure")
    monkeypatch.setattr(viseme_batch,"run",run)
    args=argparse.Namespace(input=source,reference=reference,output=tmp_path/"batch",generate=True)
    with pytest.raises(PreparationError,match="Batch stopped at U"):batch(args)
    assert calls==[("EI",b"same-base"),("O",b"same-base"),("U",b"same-base")]
    report=json.loads((args.output/"batch.json").read_text())
    assert report["request_limit"]==7 and report["order"]==list(VISEMES[1:])


def test_resume_reuses_raw_and_refuses_missing_paid_response(tmp_path,monkeypatch):
    source=tmp_path/"base.png"; Image.new("RGB",(320,400),"#C7B7A8").save(source)
    variant=BytesIO(); Image.new("RGB",(320,400),"#BAA798").save(variant,format="PNG")
    calls=[]
    class Detector:
        def __init__(self,*_):self.count=0
        def __enter__(self):return self
        def __exit__(self,*_):pass
        def detect(self,_):self.count+=1;return face(self.count%2==0)
    def generate(data,*_):
        calls.append(data);return variant.getvalue(),{"request_settings":{},"model":"test"}
    monkeypatch.setattr(viseme_trial,"FaceDetector",Detector)
    monkeypatch.setattr(viseme_trial,"generate_a",generate)
    monkeypatch.setattr(viseme_trial,"dotenv_values",lambda _: {"GEMINI_API_KEY":"private-test-key"})
    args=argparse.Namespace(input=source,output=tmp_path/"output",viseme="EI",generate=True,variant=None,model=None,face_model=tmp_path/"model")
    viseme_trial.run(args);viseme_trial.run(args)
    assert calls==[source.read_bytes()]
    report=json.loads((args.output/"run.json").read_text())
    assert report["generation"]["request_settings"]["input_sha256"]==sha256(source.read_bytes()).hexdigest()
    (args.output/"generated-ei.png").unlink()
    with pytest.raises(PreparationError,match="previous request"):viseme_trial.run(args)
    assert len(calls)==1
