"""Archive the source-layout cleanup and verify exact restoration, without providers."""
import json
from pathlib import Path
import shutil
import xml.etree.ElementTree as ET

from app.audits.finalize_refinement import digest, require, source_snapshot
from app.core.paths import ROOT
from app.preparation.archive import Archive
from app.workflows.visemes.viseme_trial import write_report


def main():
    root = ROOT/'.tools/organization-16'
    relocation = json.loads((root/'relocations.json').read_text())
    for name, expected in relocation['protected_files'].items():
        require(digest(ROOT/name) == expected, f'Protected resource changed: {name}')
    pixels = json.loads((root/'pixel-equivalence.json').read_text())
    require(pixels['frames'] == 192 and pixels['all_decoded_pixels_equal'], 'Rendering equivalence failed.')
    browser = json.loads((root/'browser-results.json').read_text())['stats']
    require(browser['expected'] == 104 and browser['unexpected'] == browser['skipped'] == browser['flaky'] == 0,
            'Browser/API regressions did not pass.')
    suites = ET.parse(root/'processor-tests.xml').getroot().findall('testsuite')
    require(sum(int(s.attrib['tests']) for s in suites) == 160 and
            all(int(s.attrib['failures']) == int(s.attrib['errors']) == 0 for s in suites),
            'Processor regressions did not pass.')
    require(not json.loads((root/'document-links.json').read_text())['missing'], 'Documentation links are broken.')
    for record in relocation['source_records']:
        record['final_sha256'] = digest(ROOT/record['new'])
    write_report(root/'final-relocations.json', relocation)
    source_snapshot(root/'after-source')
    extra = [ROOT/'README.md', ROOT/'compose.yaml', ROOT/'.vscode/settings.json',
             *list((ROOT/'apps/web/config').rglob('*.ts')),
             *list((ROOT/'docs').rglob('*.md')), ROOT/'docs/development/relocations.json']
    for path in extra:
        destination = root/'after-source'/path.relative_to(ROOT)
        destination.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(path, destination)
    db = Archive()
    with db.connect() as conn:
        usage = conn.execute('SELECT budget_id,kind,count(*) AS requests FROM requests GROUP BY budget_id,kind ORDER BY budget_id,kind').fetchall()
        speech = conn.execute("SELECT count(*) AS requests,coalesce(sum(characters),0) AS characters FROM syntelle_speech.requests WHERE budget_id=%s",
                              ('authorized-dragon-motion-2026-10-19',)).fetchone()
    write_report(root/'audit.json', {'moved_files': len(relocation['moves']), 'protected_files_unchanged': len(relocation['protected_files']),
        'pixel_identical_frames': pixels['frames'], 'browser': browser, 'processor_tests': 160,
        'image_analysis_usage_observed': usage, 'speech_usage_observed': speech,
        'new_provider_requests': 0, 'database_migrations': 0, 'animation_changed': False,
        'owner_visual_acceptance': False, 'real_device_validation': False})
    archived = db.archive_tree('repository-organization-16', root)
    destination = ROOT/'.tools/organization-16-restored'/str(archived['snapshot_id'])
    count = db.restore(archived['snapshot_id'], destination)
    for path in root.rglob('*'):
        if not path.is_file() or '__pycache__' in path.parts or path.suffix in ('.pyc', '.tmp'):
            continue
        require(digest(path) == digest(destination/path.relative_to(root)), 'Restored cleanup evidence differs.')
    result = {'snapshot': archived, 'restored_files': count, 'all_bytes_verified': True}
    db.event('repository-organization-16', 'verified-restoration', result)
    write_report(ROOT/'.tools/organization-16-postgres-restore.json', result)
    print(json.dumps(result, indent=2))


if __name__ == '__main__':
    main()
