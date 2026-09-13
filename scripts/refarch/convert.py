"""Convert research-format reference architectures into iagram template specs.

Usage: python3 convert.py <provider> <input.yaml> <templates dir> <api base>
Queries the running iagram server for catalog classification (curated ids,
attachments, links, allowed parents, required props).
"""
import json, re, sys, urllib.request, urllib.error, os
import yaml

provider, src, outdir, api = sys.argv[1:5]
PFX = {'aws': 'aws', 'gcp': 'google', 'azure': 'azurerm'}[provider]

def get(path):
    try:
        with urllib.request.urlopen(api + path) as r:
            return json.load(r)
    except urllib.error.HTTPError:
        return None

catalog = get('/api/catalog')
entries = {e['id']: e for e in catalog['entries']}
curated_by_tf = {}
for e in catalog['entries']:
    tf = ((e.get('terraform') or {}).get('import') or {}).get('resource')
    if e['provider'] == provider and tf:
        curated_by_tf[tf] = e['id']
links = {l['resource']: l for l in catalog.get('links', []) if l['provider'] == provider}
entry_cache = {}
def entry(eid):
    if eid in entries: return entries[eid]
    if eid not in entry_cache: entry_cache[eid] = get('/api/resources/' + eid)
    return entry_cache[eid]

def slug(s):
    s = re.sub(r'[^a-z0-9]+', '-', str(s).lower()).strip('-')
    return s or 'x'

ROOT_KINDS = {'aws': ('account', 'region'), 'gcp': ('project', None), 'azure': ('subscription', 'resource_group')}
PLACEHOLDER = {'project_id': 'my-project-123456', 'subscription_id': '00000000-0000-0000-0000-000000000000', 'domain': 'example.com', 'location': 'westeurope', 'region': 'eu-west-1'}

def required_props(eid, given):
    e = entry(eid)
    if not e: return given
    props = (e.get('props') or {}).get('properties') or {}
    out = dict(given)
    for k in (e.get('props') or {}).get('required') or []:
        if k in out: continue
        sch = props.get(k, {})
        if sch.get('default') is not None: continue
        if sch.get('enum'): out[k] = sch['enum'][0]; continue
        if k in PLACEHOLDER: out[k] = PLACEHOLDER[k]; continue
        if sch.get('type') == 'string': out[k] = 'changeme'
        elif sch.get('type') == 'object': out[k] = {}
        elif sch.get('type') == 'array': out[k] = []
        elif sch.get('type') in ('number', 'integer'): out[k] = 1
        elif sch.get('type') == 'boolean': out[k] = True
    return out

def sprops_az(s):
    az = str(s.get('az', 'a'))
    if az in list('abcdef'): return az
    return 'a' if az in ('outpost', 'outposts', 'onprem') else '-' + az.lstrip('-')

def convert(arch):
    nodes, edges, warn = [], [], []
    ids = {}          # ref key -> node id
    node_type = {}    # id -> element id
    node_parent = {}
    def add(nid, typ, name, parent=None, props=None, caption=None):
        nid = slug(nid)
        base, i = nid, 2
        while nid in node_type: nid = f'{base}-{i}'; i += 1
        props = required_props(typ, props or {})
        n = {'id': nid, 'type': typ, 'name': str(name)}
        if parent: n['parent'] = parent
        if props: n['props'] = props
        if caption: n['caption'] = caption
        nodes.append(n); node_type[nid] = typ; node_parent[nid] = parent
        return nid
    first_region = None
    first_root = None
    account_by_name = {}
    def ensure_account(name):
        if name in account_by_name: return account_by_name[name]
        if provider == 'aws':
            aid = add(f'acct-{name}', 'aws.account', name)
        elif provider == 'gcp':
            aid = add(f'prj-{name}', 'gcp.project', name, props={'project_id': slug(name) + '-123456'})
        else:
            aid = add(f'sub-{name}', 'azure.subscription', name)
        account_by_name[name] = aid
        ids[f'{ROOT_KINDS[provider][0]}:{name}'] = aid
        return aid
    def add_region(acct, rname, label=None):
        nonlocal first_region
        if provider == 'aws':
            rid = add(f'reg-{rname}', 'aws.region', rname, acct, {'region': rname}, caption=label)
        elif provider == 'gcp':
            # projects carry the region; no separate box
            for n in nodes:
                if n['id'] == acct: n.setdefault('props', {})['region'] = rname if rname != 'global' else 'europe-west1'
            rid = acct
        else:
            rid = add(f'rg-{rname}', 'azure.resource_group', rname, acct, {'location': rname}, caption=label)
        if first_region is None: first_region = rid
        ids[f'region:{rname}'] = rid
        return rid
    for c in arch.get('containers') or []:
        if 'datacenter' in c:
            name = c['datacenter'] if isinstance(c['datacenter'], str) else 'on-premises'
            did = add(f'dc-{name}', 'common.datacenter', name)
            ids['datacenter'] = did; ids[f'datacenter:{name}'] = did
            continue
        if 'group' in c and provider == 'gcp':
            gid = add(f'grp-{c["group"]}', 'common.group', c['group'])
            ids[f'group:{c["group"]}'] = gid
            for f in c.get('folders') or []:
                fid = add(f, 'gcp.res.google_folder', f, gid, {'display_name': f, 'parent': 'organizations/000000000000'})
                ids[f] = fid
            continue
        rootname = c.get('account') or c.get('project') or c.get('subscription') or 'prod'
        has_ous = provider == 'aws' and any(isinstance(g, dict) and 'accounts' in g for g in c.get('groups') or [])
        if has_ous:
            org = ids.get('org') or add('org', 'aws.organization', 'organization', None, {'root_id': 'r-abcd'})
            ids['org'] = org
            acct = account_by_name.get(rootname) or add(f'acct-{rootname}', 'aws.account', rootname, org)
            account_by_name[rootname] = acct; ids[f'account:{rootname}'] = acct
        else:
            acct = ensure_account(rootname)
        if first_root is None: first_root = acct
        for g in c.get('groups') or []:
            if isinstance(g, dict) and 'accounts' in g:
                gid = add(f'ou-{g["label"]}', 'aws.organizational_unit', re.sub(r'\s*OU$', '', g['label']), ids['org'])
                ids[f'group:{g["label"]}'] = gid
                for a in g['accounts']:
                    aid = add(f'acct-{a}', 'aws.account', a, gid)
                    ids[f'account:{a}'] = aid; account_by_name[a] = aid
        rname = c.get('region') or (c.get('resource_group') or {}).get('location') if isinstance(c.get('resource_group'), dict) else c.get('region')
        if provider == 'azure':
            rg = c.get('resource_group') or {'name': f'rg-{rootname}', 'location': 'westeurope'}
            rid = add(f'rg-{rg["name"]}', 'azure.resource_group', rg['name'], acct, {'location': rg.get('location', 'westeurope')})
            if first_region is None: first_region = rid
            ids['region'] = ids.get('region') or rid
            ids[f'resource_group:{rg["name"]}'] = rid
        else:
            rid = add_region(acct, rname or PLACEHOLDER['region'], c.get('label'))
            ids.setdefault('region', rid)
        for rg in c.get('resource_groups') or []:
            rid2 = add(f'rg-{rg["name"]}', 'azure.resource_group', rg['name'], acct, {'location': rg.get('location', 'westeurope')})
            ids[f'resource_group:{rg["name"]}'] = rid2
        for g in c.get('groups') or []:
            if isinstance(g, dict) and 'label' in g and 'accounts' not in g:
                gid = add(f'grp-{g["label"]}', 'common.group', g['label'], acct if provider == 'azure' and not c.get('resource_group') else rid)
                ids[f'group:{g["label"]}'] = gid
        for v in c.get('vpcs') or c.get('vnets') or []:
            vtype = {'aws': 'aws.vpc', 'gcp': 'gcp.vpc', 'azure': 'azure.vnet'}[provider]
            vprops = {'cidr': v['cidr']} if v.get('cidr') else {}
            vid = add(f'vpc-{v["name"]}', vtype, v['name'], rid, vprops)
            ids[f'vpc:{v["name"]}'] = vid; ids[f'vnet:{v["name"]}'] = vid; ids[v['name']] = vid
            azs = {}
            for s in v.get('subnets') or []:
                parent = vid
                if provider == 'aws' and s.get('az'):
                    if s['az'] not in azs:
                        azs[s['az']] = add(f'az-{v["name"]}-{s["az"]}', 'aws.availability_zone', str(s['az']).lstrip('-'), vid, {'zone': sprops_az(s)})
                    parent = azs[s['az']]
                stype = {'aws': 'aws.subnet', 'gcp': 'gcp.subnetwork', 'azure': 'azure.subnet'}[provider]
                sprops = {'cidr': s.get('cidr', '10.0.0.0/24')}
                if provider == 'aws':
                    az = str(s.get('az', 'a'))
                    if az not in list('abcdef'):
                        az = 'a' if az in ('outpost', 'outposts', 'onprem') else ('-' + az.lstrip('-'))
                    sprops.update({'az': az, 'public': bool(s.get('public'))})
                if provider == 'gcp' and s.get('region'): sprops['region'] = s['region']
                sid = add(f'sub-{s["name"]}', stype, s['name'], parent, sprops)
                ids[f'subnet:{s["name"]}'] = sid
    if first_root is None:
        first_root = ensure_account('prod'); first_region = add_region(first_root, PLACEHOLDER['region']); ids['region'] = first_region
    # actors
    for a in arch.get('actors') or []:
        if a == 'datacenter':
            if 'datacenter' not in ids: ids['datacenter'] = add('dc-on-premises', 'common.datacenter', 'on-premises')
            continue
        aid = add(a, f'common.{a}', a)
        ids[a] = aid
    def resolve_container(ref):
        if ref is None: return first_region
        if ref in ids: return ids[ref]
        if ref == 'region': return first_region
        if ref == 'account' or ref == 'project' or ref == 'subscription': return first_root
        if ref.startswith('datacenter'): return ids.get('datacenter') or first_region
        return None
    def logical_type(cid):
        while cid:
            t = node_type[cid]
            if not entry(t).get('transparent'): return t
            cid = node_parent.get(cid)
        return 'root'
    implicit = {}
    def region_of(cid):
        cur = cid
        while cur and node_type[cur] not in ('aws.region', 'gcp.project', 'azure.resource_group'):
            cur = node_parent.get(cur)
        return cur or first_region
    def implicit_network(cid, want):
        # an element needs a VPC/subnet the diagram did not draw: add one per region
        reg = region_of(cid)
        key = (reg, want)
        if key in implicit: return implicit[key]
        vtype = {'aws': 'aws.vpc', 'gcp': 'gcp.vpc', 'azure': 'azure.vnet'}[provider]
        stype = {'aws': 'aws.subnet', 'gcp': 'gcp.subnetwork', 'azure': 'azure.subnet'}[provider]
        vid = implicit.get((reg, vtype))
        if not vid:
            used = {n.get('props', {}).get('cidr') for n in nodes}
            third = 200
            while f'10.{third}.0.0/16' in used: third += 1
            implicit['third'] = third
            vid = add(f'vpc-{reg}', vtype, 'main', reg, {'cidr': f'10.{third}.0.0/16'} if provider != 'gcp' else {})
            implicit[(reg, vtype)] = vid
            warn.append(f'added an implicit {vtype} in {reg}')
        if want == vtype: return vid
        if provider == 'aws':
            for i, z in enumerate('ab'):
                azid = add(f'az-{reg}-{z}', 'aws.availability_zone', z, vid, {'zone': z})
                sid = add(f'sub-{reg}-{z}', 'aws.subnet', f'private-{z}', azid, {'cidr': f'10.{implicit.get("third", 200)}.{i}.0/24', 'az': z, 'public': False})
                implicit.setdefault((reg, stype), sid)
        else:
            sprops = {'cidr': f'10.{implicit.get("third", 200)}.0.0/24'}
            if provider == 'gcp': sprops['region'] = 'europe-west1'
            implicit[(reg, stype)] = add(f'sub-{reg}', stype, 'main', vid, sprops)
        return implicit[(reg, stype)]
    def place(eid, cid):
        e = entry(eid)
        allowed = set(e.get('allowed_parents') or [])
        cur = cid
        while cur:
            if '*' in allowed or logical_type(cur) in allowed or e.get('attachment'): return cur
            cur = node_parent.get(cur)
        for want in (f'{provider}.subnet', 'gcp.subnetwork', f'{provider}.vpc', 'azure.vnet'):
            if want in allowed:
                return implicit_network(cid, want)
        return first_region if (logical_type(first_region) in allowed) else first_root
    link_nodes = {}
    for el in arch.get('elements') or []:
        tf = el['tf']
        eid = curated_by_tf.get(tf) or f'{provider}.res.{tf}'
        e = entry(eid)
        if not e:
            warn.append(f'unknown type {tf} ({el["name"]}) dropped'); continue
        if e.get('link'):
            link_nodes[el['name']] = tf; continue
        if e.get('attachment'):
            warn.append(f'{tf} ({el["name"]}) is configuration of another element; dropped'); continue
        if e.get('component'):
            # a member of a cluster box: place it inside an owner drawn in this architecture
            owners = [nid for nid, t in node_type.items() if t in (e.get('allowed_parents') or [])]
            if not owners:
                warn.append(f'{tf} ({el["name"]}) is a component of a cluster that is not drawn; dropped'); continue
            nid = add(el['name'], eid, el['name'], owners[0]); ids[el['name']] = nid; continue
        cid = resolve_container(el.get('in'))
        if cid is None:
            warn.append(f'{el["name"]}: unknown container {el.get("in")}; placed in region'); cid = first_region
        if entry(node_type[cid]).get('kind') != 'container':
            cid = node_parent.get(cid) or first_region
        cid = place(eid, cid)
        nid = add(el['name'], eid, el['name'], cid)
        ids[el['name']] = nid
    if provider == 'aws':
        for vid, vt in list(node_type.items()):
            if vt != 'aws.vpc': continue
            azs = {}
            for sid, st in node_type.items():
                if st == 'aws.subnet':
                    cur = node_parent.get(sid)
                    while cur and cur != vid: cur = node_parent.get(cur)
                    if cur == vid:
                        azs.setdefault(next(n for n in nodes if n['id'] == sid)['props'].get('az', 'a'), node_parent.get(sid))
            if not azs and not any(node_parent.get(x) == vid for x in node_type): continue
            for z in 'abc':
                if len(azs) >= 2: break
                if z in azs: continue
                parent = vid
                if any(node_type[p] == 'aws.availability_zone' for p in azs.values()):
                    parent = add(f'az-{vid}-{z}', 'aws.availability_zone', z, vid, {'zone': z})
                vcidr = next(n for n in nodes if n['id'] == vid)['props'].get('cidr', '10.0.0.0/16')
                base = '.'.join(vcidr.split('.')[:2])
                add(f'sub-{vid}-{z}', 'aws.subnet', f'private-{z}', parent, {'cidr': f'{base}.25{ord(z)-97}.0/24', 'az': z, 'public': False})
                azs[z] = parent
                warn.append(f'added a second-AZ subnet to VPC {vid}')
    def endpoint(ref):
        if ref in ids: return ids[ref]
        if ref == 'datacenter': return ids.get('datacenter')
        if ref.startswith(('vpc:', 'vnet:', 'subnet:', 'account:', 'project:', 'group:', 'region:')): return ids.get(ref)
        return None
    steps = {}
    flows = arch.get('flows') or []
    # link nodes referenced by flows: pair sources and targets
    handled = set()
    for lname, tf in link_nodes.items():
        srcs = [f for f in flows if f.get('to') == lname]
        dsts = [f for f in flows if f.get('from') == lname]
        for f in srcs + dsts: handled.add(id(f))
        if srcs and dsts:
            flows.append({'from': srcs[0]['from'], 'to': dsts[0]['to'], 'link': tf, 'label': srcs[0].get('label'), 'step': srcs[0].get('step')})
    for f in flows:
        if id(f) in handled: continue
        a, b = endpoint(str(f['from'])), endpoint(str(f['to']))
        if not a or not b:
            warn.append(f'flow {f.get("from")} -> {f.get("to")} dropped (unknown endpoint)'); continue
        ed = {'from': a, 'to': b}
        if f.get('label'): ed['label'] = str(f['label'])
        if f.get('step') is not None:
            ed['step'] = str(f['step'])
            steps.setdefault(ed['step'], f"{f['from']} → {f['to']}" + (f": {f['label']}" if f.get('label') else ''))
        if f.get('link'):
            l = links.get(f['link'])
            ta, tb = node_type[a], node_type[b]
            ok = l and ((ta in l['from']['elements'] and tb in l['to']['elements']) or (tb in l['from']['elements'] and ta in l['to']['elements']))
            if ok:
                ed['link'] = f['link']
            else:
                ed.setdefault('label', f['link'].split('_', 1)[1].replace('_', ' '))
                warn.append(f'link {f["link"]} does not join {ta} and {tb}; drawn as an arrow')
        edges.append(ed)
    spec = {'title': arch['title'], 'category': arch['category'], 'tags': arch.get('tags') or [], 'source': arch['source'],
            'description': arch['description'], 'nodes': nodes, 'edges': edges}
    if steps: spec['steps'] = [{'n': k, 'text': v} for k, v in sorted(steps.items(), key=lambda kv: (len(kv[0]), kv[0]))]
    return spec, warn

class Dumper(yaml.SafeDumper):
    pass
def repr_dict(d, data):
    # flow style for node/edge rows
    return d.represent_mapping('tag:yaml.org,2002:map', data, flow_style=all(not isinstance(v, (dict, list)) or (isinstance(v, dict) and all(not isinstance(x, (dict, list)) for x in v.values())) for v in data.values()) and 'nodes' not in data)
Dumper.add_representer(dict, repr_dict)

from collections import Counter
dropped_types = Counter(); unknown_types = Counter(); bad_links = Counter()
archs = yaml.safe_load(open(src))
seen_slugs = set(os.listdir(os.path.join(outdir, provider))) if os.path.isdir(os.path.join(outdir, provider)) else set()
total = 0
for arch in archs:
    if arch['slug'] in seen_slugs and not os.path.exists(os.path.join(outdir, provider, arch['slug'], '.from-' + os.path.basename(src))):
        print(f"{provider}/{arch['slug']}: slug already exists (another file); skipped"); continue
    spec, warn = convert(arch)
    for w in warn:
        m = re.match(r'(\S+) \(.*\) is configuration of another element', w)
        if m: dropped_types[m.group(1)] += 1
        m = re.match(r'unknown type (\S+)', w)
        if m: unknown_types[m.group(1)] += 1
        m = re.match(r'link (\S+) does not join', w)
        if m: bad_links[m.group(1)] += 1
    total += 1
    d = os.path.join(outdir, provider, arch['slug'])
    os.makedirs(d, exist_ok=True)
    with open(os.path.join(d, 'template.yaml'), 'w') as f:
        f.write(f"# Generated from the official reference ({arch['source']}); edit freely.\n")
        yaml.dump(spec, f, Dumper=Dumper, sort_keys=False, allow_unicode=True, width=200)
    open(os.path.join(d, '.from-' + os.path.basename(src)), 'w').close()
    print(f"{provider}/{arch['slug']}: {len(spec['nodes'])} nodes, {len(spec['edges'])} edges" + (f"; {len(warn)} warnings" if warn else ''))
    for w in warn: print('   -', w)
rep = os.path.join(os.path.dirname(os.path.abspath(src)), 'report-' + os.path.basename(src).replace('.yaml', '.txt'))
with open(rep, 'w') as f:
    f.write(f"{total} templates from {os.path.basename(src)}\n\nDropped as configuration (candidates for first_level):\n")
    for t, n in dropped_types.most_common(): f.write(f"  {n:3d}  {t}\n")
    f.write("\nUnknown Terraform types (not in the provider schema):\n")
    for t, n in unknown_types.most_common(): f.write(f"  {n:3d}  {t}\n")
    f.write("\nLinks that did not join their ends (drawn as arrows):\n")
    for t, n in bad_links.most_common(): f.write(f"  {n:3d}  {t}\n")
print('report:', rep)
