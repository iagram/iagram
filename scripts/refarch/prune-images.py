#!/usr/bin/env python3
"""Remove bundled reference pictures that are not the architecture diagram.

Input: one or more JSON Lines files produced by the reference reconciliation
(one object per template with at least "template": "<provider>/<slug>" and
"image_ok": true|false). For every template whose picture was judged not to
be its diagram, this deletes templates/<provider>/<slug>/diagram.*, drops the
template's `image:` line so the gallery shows the generated miniature, and
removes its credit line from templates/NOTICE.md.

    scripts/refarch/prune-images.py annot-*.jsonl
"""
import glob
import json
import os
import re
import sys

ROOT = os.path.join(os.path.dirname(os.path.abspath(__file__)), '..', '..')


def main(paths):
    bad = []
    for p in paths:
        for line in open(p):
            line = line.strip()
            if not line:
                continue
            try:
                rec = json.loads(line)
            except json.JSONDecodeError:
                print(f'skip unreadable line in {p}', file=sys.stderr)
                continue
            if rec.get('image_ok') is False:
                bad.append(rec['template'])
    notice_path = os.path.join(ROOT, 'templates', 'NOTICE.md')
    notice = open(notice_path).read()
    removed = 0
    for t in sorted(set(bad)):
        d = os.path.join(ROOT, 'templates', *t.split('/'))
        for f in glob.glob(os.path.join(d, 'diagram.*')):
            os.remove(f)
            removed += 1
        spec = os.path.join(d, 'template.yaml')
        if os.path.exists(spec):
            text = open(spec).read()
            text2 = re.sub(r'^image: .*\n', '', text, flags=re.M)
            if text2 != text:
                open(spec, 'w').write(text2)
        notice = '\n'.join(l for l in notice.split('\n') if f'/{t}' not in l and f' {t}' not in l)
    open(notice_path, 'w').write(notice)
    print(f'{len(set(bad))} templates flagged, {removed} pictures removed')


if __name__ == '__main__':
    main(sys.argv[1:])
