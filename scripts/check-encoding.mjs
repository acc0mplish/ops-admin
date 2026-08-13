import fs from 'node:fs'
import path from 'node:path'

const root = path.resolve(process.argv[2] || '.')

const suspiciousPatterns = [
  '\uFFFD',
  '\u951F',
  '\u9435',
  '\u95F8',
  '\u95FA',
  '\u95F9',
  '\u95FB',
  '\u95C1',
  '\u6FDE',
  '\u5A75',
  '\u740C\u3128\u52CF\u93BB',
  '\u7F02\u51A9\u57B9\u7EEE',
  '\u5A62\u60F0\u7CBE\u7459'
]

const allowedExtensions = new Set(['.go', '.js', '.mjs', '.vue', '.md', '.yaml', '.yml'])
const ignoredDirs = new Set(['.git', '.agents', '.codex', '.gocache', 'node_modules', 'dist'])
const ignoredFiles = new Set(['check-encoding.mjs', 'PLATFORM_UX_REVIEW.md'])
const hits = []

function walk(dir) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    if (entry.isDirectory()) {
      if (!ignoredDirs.has(entry.name)) {
        walk(path.join(dir, entry.name))
      }
      continue
    }

    if (!entry.isFile() || ignoredFiles.has(entry.name)) {
      continue
    }

    const file = path.join(dir, entry.name)
    if (allowedExtensions.has(path.extname(file))) {
      scanFile(file)
    }
  }
}

function scanFile(file) {
  const content = fs.readFileSync(file, 'utf8')
  const lines = content.split(/\r?\n/)

  lines.forEach((line, index) => {
    const pattern = suspiciousPatterns.find((item) => line.includes(item))
    if (!pattern) {
      return
    }

    hits.push({
      file: path.relative(root, file),
      line: index + 1,
      pattern: pattern === '\uFFFD' ? 'replacement-character' : pattern,
      text: line.trim()
    })
  })
}

walk(root)

if (hits.length) {
  for (const hit of hits) {
    console.error(`${hit.file}:${hit.line} [${hit.pattern}] ${hit.text}`)
  }
  console.error(`\nFound ${hits.length} suspicious encoding issue(s). Please fix them before release.`)
  process.exit(1)
}

console.log('Encoding scan passed.')
