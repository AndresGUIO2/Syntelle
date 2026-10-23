"""Archive/restore preparation evidence in local PostgreSQL without paid calls."""
import argparse
import json
from pathlib import Path

from app.preparation.archive import Archive
from app.preparation.faces import PreparationError


if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--directory',type=Path);parser.add_argument('--experiment')
    parser.add_argument('--restore',type=int);parser.add_argument('--destination',type=Path)
    args=parser.parse_args()
    try:
        db=Archive()
        if args.restore:
            if not args.destination:parser.error('--destination is required for restore')
            print(json.dumps({'restored_files':db.restore(args.restore,args.destination)}))
        elif args.directory:
            if not args.experiment:parser.error('--experiment is required for archive')
            print(json.dumps(db.archive_tree(args.experiment,args.directory)))
        print(json.dumps(db.summary()))
    except PreparationError as error:raise SystemExit(str(error)) from None
