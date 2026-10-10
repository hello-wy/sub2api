#!/usr/bin/env python3
"""Export frozen business_* tables using one PostgreSQL snapshot.
Use standard PGHOST/PGPORT/PGDATABASE/PGUSER/PGPASSFILE variables; no secrets in argv.
Restore the custom dump into a database with the matching historical schema.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess
import tempfile


def export(destination):
    destination = Path(destination)
    destination.mkdir(mode=0o700, parents=True, exist_ok=False)
    with tempfile.TemporaryFile(mode='w+') as errors:
        session = subprocess.Popen(['psql', '-X', '-qAt', '-v', 'ON_ERROR_STOP=1'],
                                   stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                   stderr=errors, text=True)
        def query(sql):
            session.stdin.write(sql + ';\n\\echo __ARCHIVE_END__\n')
            session.stdin.flush()
            lines = []
            for line in session.stdout:
                if line.rstrip() == '__ARCHIVE_END__':
                    return '\n'.join(lines)
                lines.append(line.rstrip())
            errors.seek(0)
            raise RuntimeError(errors.read() or 'PostgreSQL session closed')
        try:
            query('BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY')
            snapshot = query('SELECT pg_export_snapshot()')
            cutover = query('SELECT cutover_at FROM operating_legacy_archive WHERE id=1')
            if not cutover:
                raise RuntimeError('No legacy cutover record; refusing an unfrozen export')
            tables = json.loads(query("SELECT json_agg(tablename ORDER BY tablename) FROM pg_tables WHERE schemaname='public' AND left(tablename,9)='business_'"))
            counts = {}
            for table in tables:
                identifier = '"' + table.replace('"', '""') + '"'
                counts[table] = int(query('SELECT count(*) FROM public.' + identifier))
            dump = destination / 'legacy.dump'
            subprocess.run(['pg_dump', '--format=custom', '--no-owner', '--no-acl',
                            '--snapshot=' + snapshot, '--table=public.business_*',
                            '--file=' + str(dump)], check=True)
            os.chmod(dump, 0o600)
            manifest = {'cutover_at': cutover, 'snapshot': snapshot, 'tables': counts,
                        'sha256': hashlib.sha256(dump.read_bytes()).hexdigest(),
                        'restore': 'Restore into an isolated database with matching historical migrations; external dependencies are not included.'}
            (destination / 'manifest.json').write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + '\n')
            query('COMMIT')
        finally:
            session.stdin.close()
            session.wait(timeout=10)
    print(destination)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('destination', help='New directory, must not exist')
    export(parser.parse_args().destination)
