from pathlib import Path

APP_DIR = Path(__file__).resolve().parents[1]
PROCESSOR_DIR = APP_DIR.parent
PREPARATION_DIR = APP_DIR / 'preparation'
ROOT = next((parent for parent in PROCESSOR_DIR.parents if (parent / 'compose.yaml').is_file()), PROCESSOR_DIR)
