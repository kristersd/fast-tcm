#!/usr/bin/env node
const { spawn } = require("child_process")
const path = require("path")
const fs = require("fs")

const platform = process.platform
const arch = process.arch

const binaryName = platform === "win32" ? "ftcm.exe" : "ftcm"

let binaryPath = path.join(__dirname, "bin", `${platform}-${arch}`, binaryName)

if (!fs.existsSync(binaryPath)) {
  if (platform === "darwin" && arch === "arm64") {
    // Apple Silicon Macs can run x64 binaries using Rosetta, so we provide the x64 version as a fallback
    binaryPath = path.join(__dirname, "bin", "darwin-x64", binaryName)
  } else if (platform === "linux" && arch === "arm64") {
    binaryPath = path.join(__dirname, "bin", "linux-x64", binaryName)
  }
}

if (!fs.existsSync(binaryPath)) {
  console.error(`❌ ftcm binary not found for ${platform}-${arch}`)
  console.error(`Expected at: ${binaryPath}`)
  process.exit(1)
}

const child = spawn(binaryPath, process.argv.slice(2), {
  stdio: "inherit",
})

child.on("exit", (code) => {
  process.exitCode = code ?? 0
})
