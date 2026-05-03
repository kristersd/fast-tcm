#!/usr/bin/env node

const fs = require("fs")
const path = require("path")
const { execSync } = require("child_process")

// Parse CLI arguments
const args = process.argv.slice(2)
let targetDir = "testdata/extensions"
let iterations = 50
let warmup = 5
let pattern = "**/*.pcss"
let keepTmp = false

for (let i = 0; i < args.length; i++) {
  if (args[i] === "--dir") {
    targetDir = args[++i]
  } else if (args[i] === "--iterations") {
    iterations = parseInt(args[++i], 10)
  } else if (args[i] === "--warmup") {
    warmup = parseInt(args[++i], 10)
  } else if (args[i] === "--pattern") {
    pattern = args[++i]
  } else if (args[i] === "--keepTmp") {
    keepTmp = true
  } else if (!args[i].startsWith("--")) {
    targetDir = args[i]
  }
}

// Resolve paths
const projectRoot = path.resolve(__dirname, "..")
const targetDirPath = path.resolve(projectRoot, targetDir)

if (!fs.existsSync(targetDirPath)) {
  console.error(`Error: directory not found: ${targetDirPath}`)
  process.exit(1)
}

// Locate tcm and ftcm (absolute paths needed when running with cwd=/)
const tcmPath = path.resolve(projectRoot, "node_modules/.bin/tcm")
if (!fs.existsSync(tcmPath)) {
  console.error("Error: typed-css-modules not found. Run npm install")
  process.exit(1)
}

const ftcmPath = path.resolve(projectRoot, "bin/ftcm")

// Build ftcm
console.log("Building ftcm...")
try {
  execSync("go build -o bin/ftcm ./cmd/ftcm", {
    cwd: projectRoot,
    stdio: "inherit",
  })
} catch (e) {
  console.error("Failed to build ftcm")
  process.exit(1)
}

console.log(`\nTarget directory: ${targetDirPath}`)
console.log(`Pattern: ${pattern}`)

// Deterministic temp output roots under /tmp
// We run tools with cwd=/ and relative outDir so tcm's path.join(rootDir, outDir)
// resolves to /tmp/ftcm-compare-dir/... correctly.
const tempRoot = "tmp/ftcm-compare-dir"
const absTempRoot = path.join("/", tempRoot)
console.log(`Temp output root: ${absTempRoot}${keepTmp ? " (will be kept)" : ""}`)
const tcmCheckDir = path.join(tempRoot, "check", "tcm")
const ftcmCheckDir = path.join(tempRoot, "check", "ftcm")
const tcmRunDir = path.join(tempRoot, "run", "tcm")
const ftcmRunDir = path.join(tempRoot, "run", "ftcm")

function clearDir(dir) {
  const absDir = path.join("/", dir)
  if (fs.existsSync(absDir)) {
    execSync(`rm -rf "${absDir}"`)
  }
  fs.mkdirSync(absDir, { recursive: true })
}

// Correctness check
console.log("\nRunning correctness check...")
clearDir(tcmCheckDir)
clearDir(ftcmCheckDir)

execSync(
  `"${tcmPath}" -p "${pattern}" -o "${tcmCheckDir}" "${targetDirPath}" >/dev/null 2>&1 || true`,
  { cwd: "/" },
)
execSync(
  `"${ftcmPath}" --pattern "${pattern}" --outDir "${ftcmCheckDir}" "${targetDirPath}" >/dev/null 2>&1 || true`,
  { cwd: "/" },
)

// Compare outputs from both output trees
let findings = { match: 0, mismatch: 0, tcmMissing: 0, ftcmMissing: 0 }

function collectRelPaths(dir, rootDir, set) {
  const absDir = path.join("/", dir)
  if (!fs.existsSync(absDir)) return
  const entries = fs.readdirSync(absDir)
  for (const entry of entries) {
    const fullPath = path.join(absDir, entry)
    const stat = fs.statSync(fullPath)
    if (stat.isDirectory()) {
      collectRelPaths(path.join(dir, entry), rootDir, set)
    } else if (entry.endsWith(".d.ts")) {
      set.add(path.relative(path.join("/", rootDir), fullPath))
    }
  }
}

const tcmFiles = new Set()
const ftcmFiles = new Set()
collectRelPaths(tcmCheckDir, tcmCheckDir, tcmFiles)
collectRelPaths(ftcmCheckDir, ftcmCheckDir, ftcmFiles)

const allFiles = new Set([...tcmFiles, ...ftcmFiles])

for (const rel of allFiles) {
  const tcmFile = path.join("/", tcmCheckDir, rel)
  const ftcmFile = path.join("/", ftcmCheckDir, rel)

  const tcmExists = fs.existsSync(tcmFile)
  const ftcmExists = fs.existsSync(ftcmFile)

  if (!tcmExists) {
    findings.tcmMissing++
    continue
  }
  if (!ftcmExists) {
    findings.ftcmMissing++
    continue
  }

  const tcmContent = fs.readFileSync(tcmFile, "utf8").replace(/\r\n/g, "\n")
  const ftcmContent = fs.readFileSync(ftcmFile, "utf8").replace(/\r\n/g, "\n")

  if (tcmContent === ftcmContent) {
    findings.match++
  } else {
    findings.mismatch++
    console.error(`Output mismatch: ${rel}`)
  }
}

console.log(
  `✓ Correctness: ${findings.match} match, ${findings.mismatch} differ, ${findings.tcmMissing} tcm missing, ${findings.ftcmMissing} ftcm missing`,
)
console.log()

if (
  findings.mismatch > 0 ||
  findings.tcmMissing > 0 ||
  findings.ftcmMissing > 0
) {
  console.error("Correctness check failed")
  process.exit(1)
}

// Statistics helper
const stats = (times) => {
  const sorted = times.slice().sort((a, b) => a - b)
  const sum = times.reduce((a, b) => a + b, 0)
  const mean = sum / times.length
  const median = sorted[Math.floor(sorted.length / 2)]
  const min = sorted[0]
  const max = sorted[sorted.length - 1]
  const p95Idx = Math.floor(sorted.length * 0.95)
  const p95 = sorted[p95Idx]
  return { mean, median, min, max, p95 }
}

// Warmup runs
console.log(`Running ${warmup} warmup iterations...`)
for (let i = 0; i < warmup; i++) {
  clearDir(ftcmRunDir)
  execSync(
    `"${ftcmPath}" --pattern "${pattern}" --outDir "${ftcmRunDir}" "${targetDirPath}" >/dev/null 2>&1 || true`,
    { cwd: "/" },
  )

  clearDir(tcmRunDir)
  execSync(
    `"${tcmPath}" -p "${pattern}" -o "${tcmRunDir}" "${targetDirPath}" >/dev/null 2>&1 || true`,
    { cwd: "/" },
  )
}

// Timed runs
console.log(`Running ${iterations} timed iterations...\n`)

const ftcmTimes = []
const tcmTimes = []

for (let i = 0; i < iterations; i++) {
  // Time ftcm
  clearDir(ftcmRunDir)
  const ftcmStart = process.hrtime.bigint()
  execSync(
    `"${ftcmPath}" --pattern "${pattern}" --outDir "${ftcmRunDir}" "${targetDirPath}" >/dev/null 2>&1 || true`,
    { cwd: "/" },
  )
  const ftcmEnd = process.hrtime.bigint()
  ftcmTimes.push(Number(ftcmEnd - ftcmStart) / 1e6)

  // Time tcm
  clearDir(tcmRunDir)
  const tcmStart = process.hrtime.bigint()
  execSync(
    `"${tcmPath}" -p "${pattern}" -o "${tcmRunDir}" "${targetDirPath}" >/dev/null 2>&1 || true`,
    { cwd: "/" },
  )
  const tcmEnd = process.hrtime.bigint()
  tcmTimes.push(Number(tcmEnd - tcmStart) / 1e6)

  if ((i + 1) % Math.ceil(iterations / 5) === 0) {
    process.stdout.write(`${i + 1}/${iterations}\r`)
  }
}

process.stdout.write("\r           \r") // Clear line

// Compute statistics
const ftcmStats = stats(ftcmTimes)
const tcmStats = stats(tcmTimes)
const ratioMedian = tcmStats.median / ftcmStats.median

console.log("═".repeat(60))
console.log("Timing Results (Directory-based comparison)")
console.log("═".repeat(60))
console.log()

console.log("ftcm (fast-tcm):")
console.log(`  Median: ${ftcmStats.median.toFixed(3)} ms`)
console.log(`  Mean:   ${ftcmStats.mean.toFixed(3)} ms`)
console.log(`  Min:    ${ftcmStats.min.toFixed(3)} ms`)
console.log(`  Max:    ${ftcmStats.max.toFixed(3)} ms`)
console.log()

console.log("tcm (typed-css-modules):")
console.log(`  Median: ${tcmStats.median.toFixed(3)} ms`)
console.log(`  Mean:   ${tcmStats.mean.toFixed(3)} ms`)
console.log(`  Min:    ${tcmStats.min.toFixed(3)} ms`)
console.log(`  Max:    ${tcmStats.max.toFixed(3)} ms`)
console.log()

console.log(`Speedup: ${ratioMedian.toFixed(2)}x (median-based, tcm/ftcm)`)
console.log(`Total runs: ${iterations} iterations`)

if (!keepTmp) {
  if (fs.existsSync(absTempRoot)) {
    execSync(`rm -rf "${absTempRoot}"`)
  }
}
