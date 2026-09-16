"""Find the architecture diagram image on each template's official source page.

Usage: python3 scripts/refarch/images.py templates
Writes `image: <url>` into every template.yaml whose source page yields a
diagram; existing `image:` values are kept. Images are never downloaded into
the repo: the gallery loads them from the vendor site at view time.
"""
import os, re, sys, glob, html, urllib.request, urllib.parse, concurrent.futures as cf
import yaml

UA = {'User-Agent': 'Mozilla/5.0 (compatible; iagram-templates/1.0; +https://github.com/iagram/iagram)'}
BAD = re.compile(r'(logo|icon|avatar|badge|favicon|sprite|arrow|button|thumb|pixel|tracking|1x1|banner|hero|social|share|featured|warning|caution|note|important|tip|callout|alert|info\b|\.gif$)', re.I)
GOOD = re.compile(r'(architecture|diagram|arch|overview|topology|flow|reference|solution|design|figure)', re.I)

def fetch(url, timeout=25):
    req = urllib.request.Request(url, headers=UA)
    with urllib.request.urlopen(req, timeout=timeout) as r:
        return r.geturl(), r.read(2_000_000).decode('utf-8', 'replace')

MIN_BYTES = 12_000  # callout icons and spacers are far smaller than a diagram

def head_ok(url):
    for method, headers in (('HEAD', UA), ('GET', {**UA, 'Range': 'bytes=0-64'})):
        try:
            req = urllib.request.Request(url, headers=headers, method=method)
            with urllib.request.urlopen(req, timeout=20) as r:
                ct = r.headers.get('Content-Type', '')
                if not ct.startswith('image/'):
                    return False
                size = r.headers.get('Content-Range', '').split('/')[-1] or r.headers.get('Content-Length', '')
                if size.isdigit() and int(size) < MIN_BYTES and 'svg' not in ct:
                    return False
                return True
        except Exception:
            continue
    return False

IMG = re.compile(r'<img\b[^>]*>', re.I)
ATTR = re.compile(r'([a-zA-Z-]+)\s*=\s*("([^"]*)"|\'([^\']*)\')')

def candidates(base, page):
    out = []
    # limit to the main content when the page marks it
    m = re.search(r'<div[^>]+class="[^"]*blog-post-content[^"]*".*?</article>', page, re.S | re.I) or re.search(r'<(main|article)\b.*?</\1>', page, re.S | re.I)
    body = m.group(0) if m else page
    for i, tag in enumerate(IMG.findall(body)):
        attrs = {k.lower(): html.unescape(v3 if v3 is not None else v4 or '') for k, _, v3, v4 in ATTR.findall(tag)}
        src = attrs.get('data-src') or attrs.get('src') or ''
        if src.startswith('data:') or not src:
            continue
        src = urllib.parse.urljoin(base, src.split()[0])
        if not re.search(r'\.(png|jpe?g|svg|webp)(\?|$)', src, re.I):
            continue
        alt = attrs.get('alt', '')
        score = 0
        if BAD.search(src) or BAD.search(alt):
            score -= 5
        if GOOD.search(alt):
            score += 4
        if GOOD.search(src):
            score += 2
        try:
            w = int(re.sub(r'\D', '', attrs.get('width', '')) or 0)
            if w and w < 200:
                score -= 4
            if w >= 600:
                score += 2
        except ValueError:
            pass
        score -= i * 0.01  # earlier images first
        out.append((score, src))
    out.sort(key=lambda t: -t[0])
    return [u for s, u in out if s > -3]

def find(source):
    try:
        base, page = fetch(source)
    except Exception as e:
        return None, f'fetch failed: {e}'
    # Open Graph image is a good hint on blogs and Microsoft Learn
    og = re.search(r'<meta[^>]+property="og:image"[^>]+content="([^"]+)"', page, re.I) or re.search(r'<meta[^>]+content="([^"]+)"[^>]+property="og:image"', page, re.I)
    cands = candidates(base, page)
    if og and 'blogs' not in source and 'blog' not in source:
        ogu = urllib.parse.urljoin(base, html.unescape(og.group(1)))
        if not BAD.search(ogu) and 'learn.microsoft.com' not in source:
            cands.append(ogu)  # last resort only
    for u in cands[:5]:
        if head_ok(u):
            return u, None
    return None, 'no diagram image found'

REFRESH = '--refresh' in sys.argv

def process(path):
    spec = yaml.safe_load(open(path))
    if spec.get('image') and not REFRESH:
        return path, spec['image'], 'kept'
    url, err = find(spec['source'])
    text = open(path).read()
    text = re.sub(r'^image: .*\n', '', text, flags=re.M)
    if url:
        text = text.replace('\nsource: ', f'\nimage: {url}\nsource: ', 1)
    open(path, 'w').write(text)
    if not url:
        return path, None, err
    return path, url, 'added'

if __name__ == '__main__':
    root = [a for a in sys.argv[1:] if not a.startswith('--')][0]
    files = sorted(glob.glob(os.path.join(root, '*', '*', 'template.yaml')))
    ok = 0
    with cf.ThreadPoolExecutor(max_workers=8) as ex:
        for path, url, note in ex.map(process, files):
            slug = '/'.join(path.split(os.sep)[-3:-1])
            if url:
                ok += 1
            print(f'{slug}: {note}' + (f' {url}' if url and note == 'added' else ''))
    print(f'{ok}/{len(files)} templates have an image')
