import json
from hashlib import sha256

import pytest

from app.oral_benchmark import evaluate
from app.preparation.faces import PreparationError


def test_corpus_does_not_claim_preparation_from_original_images_alone(tmp_path):
    original=tmp_path/'original.png';original.write_bytes(b'owned-test-fixture')
    corpus=tmp_path/'corpus.json';corpus.write_text(json.dumps({'cases':[{'id':'test','split':'development','kind':'photograph',
        'original':'original.png','original_sha256':sha256(original.read_bytes()).hexdigest()}]}))
    report=evaluate(corpus,tmp_path/'report.json')
    assert not report['passed'] and not report['cases'][0]['prepared']
    assert report['counts']=={'development':1,'held-out':0}
    assert report['new_provider_requests']==0


def test_corpus_prevents_a_development_image_counting_as_held_out(tmp_path):
    original=tmp_path/'original.png';original.write_bytes(b'owned-test-fixture')
    case={'id':'test','split':'development','kind':'photograph','original':'original.png','original_sha256':sha256(original.read_bytes()).hexdigest()}
    corpus=tmp_path/'corpus.json';corpus.write_text(json.dumps({'cases':[case,{**case,'id':'duplicate','split':'held-out'}]}))
    with pytest.raises(PreparationError,match='same portrait'):evaluate(corpus,tmp_path/'report.json')
