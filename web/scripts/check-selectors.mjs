// Fails the build if a zustand selector returns a freshly built array, which
// re-renders forever with useSyncExternalStore (React error #185). Derive
// such values with useMemo from a stable slice instead.
import { readdirSync, readFileSync, statSync } from 'node:fs'
import { join } from 'node:path'

const bad = []
const walk = (dir) => {
  for (const f of readdirSync(dir)) {
    const p = join(dir, f)
    if (statSync(p).isDirectory()) walk(p)
    else if (/\.tsx?$/.test(p)) {
      const src = readFileSync(p, 'utf8')
      const re = /useStore\(\(s\) =>\s*s\.[a-zA-Z.]+\.(filter|map|slice|concat|flatMap)\(/g
      let m
      while ((m = re.exec(src))) bad.push(`${p}: selector builds a new array via .${m[1]}()`)
    }
  }
}
walk(new URL('../src', import.meta.url).pathname)
if (bad.length) {
  console.error(bad.join('\n'))
  process.exit(1)
}
