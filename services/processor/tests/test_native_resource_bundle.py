import hashlib
import json
from pathlib import Path

import pytest
from PIL import Image

from app.workflows.rendering.export_native import export


def fixture_bundle(root: Path, source: str = 'a' * 64) -> Path:
    folders = {name: root / name for name in ('oral', 'motion', 'motion_assets', 'gaze', 'expressions', 'backgrounds')}
    for folder in folders.values():
        folder.mkdir(parents=True)
    def write(folder, name, value):
        (folders[folder] / name).write_text(json.dumps(value), encoding='utf-8')
    for folder, names in {'oral': ['upper.png', 'lower.png', 'surface.png', 'mask.png'], 'motion_assets': ['foreground.png', 'open.png', 'closed.png', 'mask.png'], 'gaze': ['sclera.png', 'ownership.png'], 'expressions': ['field.png', 'face.png', 'cavity.png'], 'backgrounds': ['background.png']}.items():
        for name in names:
            Image.new('RGBA', (2, 2), (140, 150, 160, 255)).save(folders[folder] / name)
    geometry = b'immutable-fixture-geometry'
    (folders['motion'] / 'geometry.bin').write_bytes(geometry)
    write('oral', 'package.json', {'source_sha256': source, 'build_id': 'oral-fixture', 'reference': {'upper': 'upper.png', 'lower': 'lower.png'}, 'poses': {'Rest': {'surface': 'surface.png', 'mask': 'mask.png'}}})
    write('motion', 'package.json', {'original_sha256': source, 'build_id': 'motion-fixture', 'oral_build_id': 'oral-fixture', 'foreground': 'foreground.png', 'eyes': {'open': 'open.png', 'closed': 'closed.png', 'mask': 'mask.png'}, 'projection': {'geometry': {'file': 'geometry.bin', 'sha256': hashlib.sha256(geometry).hexdigest()}}})
    write('gaze', 'gaze.json', {'source_sha256': source, 'motion_build_id': 'motion-fixture'})
    asset = lambda name: {'file': name, 'sha256': hashlib.sha256((folders['expressions'] / name).read_bytes()).hexdigest()}
    write('expressions', 'expressions.json', {'source_sha256': source, 'motion_build_id': 'motion-fixture', 'oral_build_id': 'oral-fixture', 'field': asset('field.png'), 'reference_laughter': {'version': 'reference-laughter/1', 'source_sha256': source, 'motion_build_id': 'motion-fixture', 'oral_build_id': 'oral-fixture', 'frames': [asset('face.png')] * 25, 'oral': {'cavity': asset('cavity.png')}}})
    write('backgrounds', 'catalog.json', {'items': [{'id': 'fixture', 'image': 'background.png'}]})
    bundle = root / 'bundle.json'
    bundle.write_text(json.dumps({'version': 'character-resource-bundle/1', 'source_sha256': source, **{key: str(path) for key, path in folders.items()}}), encoding='utf-8')
    return bundle


def test_explicit_bundle_exports_independent_character_and_reference_textures(tmp_path: Path):
    bundle = fixture_bundle(tmp_path / 'character')
    export(tmp_path / 'native', bundle=bundle)
    result = json.loads((tmp_path / 'native/manifest.json').read_text(encoding='utf-8'))
    assert result['packages']['oral']['source_sha256'] == 'a' * 64
    assert 'expressions/face.png' in result['textures']
    assert 'expressions/cavity.png' in result['textures']
    assert result['paid_provider_calls'] == 0
    assert not any('syntelle' in item['path'] for item in result['identity_files'])
    for texture in result['textures'].values():
        assert hashlib.sha256((tmp_path / 'native' / texture['file']).read_bytes()).hexdigest() == texture['sha256']


def test_bundle_rejects_mixed_portraits_and_changed_reference(tmp_path: Path):
    bundle = fixture_bundle(tmp_path / 'character')
    oral = tmp_path / 'character/oral/package.json'
    source = json.loads(oral.read_text(encoding='utf-8'))
    source['source_sha256'] = 'b' * 64
    oral.write_text(json.dumps(source), encoding='utf-8')
    with pytest.raises(ValueError, match='different portraits'):
        export(tmp_path / 'mixed', bundle=bundle)
    source['source_sha256'] = 'a' * 64
    oral.write_text(json.dumps(source), encoding='utf-8')
    Image.new('RGBA', (2, 2), 'red').save(tmp_path / 'character/expressions/face.png')
    with pytest.raises(ValueError, match='Reference laughter asset checksum'):
        export(tmp_path / 'changed', bundle=bundle)


def test_bundle_rejects_cross_build_packages(tmp_path: Path):
    bundle = fixture_bundle(tmp_path / 'character')
    gaze = tmp_path / 'character/gaze/gaze.json'
    source = json.loads(gaze.read_text(encoding='utf-8'))
    source['motion_build_id'] = 'different-motion'
    gaze.write_text(json.dumps(source), encoding='utf-8')
    with pytest.raises(ValueError, match='motion build binding'):
        export(tmp_path / 'crossed', bundle=bundle)
