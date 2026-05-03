#!/usr/bin/env node

const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');
const os = require('os');

// Parse CLI arguments
const args = process.argv.slice(2);
let file = 'testdata/bench/big.css';
let iterations = 100;
let warmup = 5;

for (let i = 0; i < args.length; i++) {
  if (args[i] === '--file') {
    file = args[++i];
  } else if (args[i] === '--iterations') {
    iterations = parseInt(args[++i], 10);
  } else if (args[i] === '--warmup') {
    warmup = parseInt(args[++i], 10);
  }
}

// Resolve file path relative to project root
const projectRoot = path.resolve(__dirname, '..');
const filePath = path.resolve(projectRoot, file);

if (!fs.existsSync(filePath)) {
  console.error(`Error: file not found: ${filePath}`);
  process.exit(1);
}

// Build ftcm
console.log('Building ftcm...');
try {
  execSync('go build -o bin/ftcm ./cmd/ftcm', { cwd: projectRoot, stdio: 'inherit' });
} catch (e) {
  console.error('Failed to build ftcm');
  process.exit(1);
}

// Locate tcm (typed-css-modules)
const tcmPath = path.resolve(projectRoot, 'node_modules/.bin/tcm');
if (!fs.existsSync(tcmPath)) {
  console.error('Error: typed-css-modules not found. Run npm install');
  process.exit(1);
}

// Get absolute path to ftcm binary
const ftcmPath = path.resolve(projectRoot, 'bin/ftcm');

console.log(`Testing file: ${filePath}`);
console.log(`ftcm binary: ${ftcmPath}`);
console.log(`tcm binary: ${tcmPath}`);
console.log();

// Correctness check: ensure ftcm and tcm produce same output
console.log('Running correctness check...');
const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), 'ftcm-compare-'));
try {
  const tcmOutDir = path.join(tempDir, 'tcm-out');
  const ftcmOutDir = path.join(tempDir, 'ftcm-out');
  
  fs.mkdirSync(tcmOutDir, { recursive: true });
  fs.mkdirSync(ftcmOutDir, { recursive: true });

  // Run tcm
  execSync(`${tcmPath} "${filePath}" --out "${tcmOutDir}"`, { stdio: 'pipe' });

  // Run ftcm
  execSync(`${ftcmPath} "${filePath}" --outDir "${ftcmOutDir}"`, { stdio: 'pipe' });

  // Compare outputs (normalize line endings)
  const tcmFiles = fs.readdirSync(tcmOutDir);
  const ftcmFiles = fs.readdirSync(ftcmOutDir);

  if (tcmFiles.length !== ftcmFiles.length) {
    console.error(`❌ Output file count mismatch: tcm=${tcmFiles.length}, ftcm=${ftcmFiles.length}`);
    process.exit(1);
  }

  let mismatch = false;
  for (const file of tcmFiles) {
    const tcmOut = fs.readFileSync(path.join(tcmOutDir, file), 'utf8').replace(/\r\n/g, '\n');
    const ftcmOut = fs.readFileSync(path.join(ftcmOutDir, file), 'utf8').replace(/\r\n/g, '\n');

    if (tcmOut !== ftcmOut) {
      console.error(`❌ Output mismatch for ${file}`);
      console.error('tcm output:');
      console.error(tcmOut);
      console.error('ftcm output:');
      console.error(ftcmOut);
      mismatch = true;
    }
  }

  if (mismatch) {
    process.exit(1);
  }

  console.log('✓ Outputs match\n');
} finally {
  // Cleanup temp dir
  execSync(`rm -rf "${tempDir}"`);
}

// Warmup runs
console.log(`Running ${warmup} warmup iterations...`);
for (let i = 0; i < warmup; i++) {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'ftcm-warmup-'));
  try {
    execSync(`${ftcmPath} "${filePath}" --outDir "${tmpDir}"`, { stdio: 'pipe' });
    execSync(`${tcmPath} "${filePath}" --out "${tmpDir}"`, { stdio: 'pipe' });
  } finally {
    execSync(`rm -rf "${tmpDir}"`);
  }
}

console.log(`Running ${iterations} timed iterations...\n`);

const ftcmTimes = [];
const tcmTimes = [];

for (let i = 0; i < iterations; i++) {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'ftcm-bench-'));
  try {
    // Time ftcm
    const ftcmStart = process.hrtime.bigint();
    execSync(`${ftcmPath} "${filePath}" --outDir "${tmpDir}"`, { stdio: 'pipe' });
    const ftcmEnd = process.hrtime.bigint();
    ftcmTimes.push(Number(ftcmEnd - ftcmStart) / 1e6); // Convert to ms

    // Time tcm
    const tcmStart = process.hrtime.bigint();
    execSync(`${tcmPath} "${filePath}" --out "${tmpDir}"`, { stdio: 'pipe' });
    const tcmEnd = process.hrtime.bigint();
    tcmTimes.push(Number(tcmEnd - tcmStart) / 1e6); // Convert to ms

    if ((i + 1) % 20 === 0) {
      process.stdout.write(`${i + 1}/${iterations}\r`);
    }
  } finally {
    execSync(`rm -rf "${tmpDir}"`);
  }
}

console.log(`\n`);

// Compute statistics
const stats = (times) => {
  const sorted = times.slice().sort((a, b) => a - b);
  const sum = times.reduce((a, b) => a + b, 0);
  const mean = sum / times.length;
  const median = sorted[Math.floor(sorted.length / 2)];
  const min = sorted[0];
  const max = sorted[sorted.length - 1];
  const p95Idx = Math.floor(sorted.length * 0.95);
  const p95 = sorted[p95Idx];
  return { mean, median, min, max, p95 };
};

const ftcmStats = stats(ftcmTimes);
const tcmStats = stats(tcmTimes);
const ratio = tcmStats.mean / ftcmStats.mean;

console.log('Results (milliseconds):');
console.log('');
console.log('ftcm:');
console.log(`  Mean:   ${ftcmStats.mean.toFixed(3)}`);
console.log(`  Median: ${ftcmStats.median.toFixed(3)}`);
console.log(`  Min:    ${ftcmStats.min.toFixed(3)}`);
console.log(`  Max:    ${ftcmStats.max.toFixed(3)}`);
console.log(`  P95:    ${ftcmStats.p95.toFixed(3)}`);
console.log('');
console.log('tcm (typed-css-modules):');
console.log(`  Mean:   ${tcmStats.mean.toFixed(3)}`);
console.log(`  Median: ${tcmStats.median.toFixed(3)}`);
console.log(`  Min:    ${tcmStats.min.toFixed(3)}`);
console.log(`  Max:    ${tcmStats.max.toFixed(3)}`);
console.log(`  P95:    ${tcmStats.p95.toFixed(3)}`);
console.log('');
console.log(`Speedup: ${ratio.toFixed(2)}x (tcm/ftcm)`);
