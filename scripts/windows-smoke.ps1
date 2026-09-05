# SPDX-License-Identifier: GPL-3.0-or-later
#
# Read-only hardware smoke test for the Windows verification session
# (spec docs/superpowers/specs/2026-09-04-windows-packaging-design.md,
# "Verification bar" step 4). Targets Windows PowerShell 5.1 — the
# shell a clean Windows 11 install ships with, not pwsh 7 — because
# that is what the verification VM has. PowerShell is not installed on
# the macOS machine that wrote this script, so it was checked
# statically only (balanced braces, every external call captured, no
# Write-Host for data) before it ran. It HAS since been run, on the
# Windows 11 ARM64 VM session, 05/09/2026 (evidence/vm-14-smoke-run.txt;
# per-command captures under evidence/vm-smoke/20260905-101214/) — see
# docs/hardware-notes.md's "Windows (ARM64 VM) session — 05/09/2026".
#
# This script makes NO write to the radio and takes no parameter that
# would start one. The two writes this milestone makes — the GUI Send
# of a trial memory edit, and the CLI `write` that restores it — are
# made by a human, by hand, following scripts/README-vm-leg.md.
#
# Usage:
#   .\windows-smoke.ps1 [-RigProg <path to rigprog.exe>] [-OutDir <dir>] [-Model <name>]
#
# Captures, under <OutDir>\<timestamp>\: host OS build/architecture,
# the CP2105's Get-PnpDevice/Get-PnpDeviceProperty entries, the
# Win32_SerialPort and SERIALCOMM registry views, `rigprog version`,
# `rigprog ports`, a `rigprog probe` of every COM port found (expected:
# exactly one succeeds), and — on the one that answered — a
# read/export/import round trip compared byte-for-byte after
# normalising the read_at and generator fields.

#Requires -Version 5.1
[CmdletBinding()]
param(
    [string]$RigProg = "$env:ProgramFiles\Open Rig Programmer\rigprog.exe",
    [string]$OutDir = '.\evidence',
    [string]$Model = 'FT-710'
)

Set-StrictMode -Version Latest
# Native commands (rigprog.exe among them) write ordinary progress to
# stderr, and probing the wrong COM port is EXPECTED to fail — under
# 'Stop' that becomes a terminating error and aborts the whole capture
# at the first port that isn't the CAT port. 'Continue' lets the loop
# below try every port and record what each one did.
$ErrorActionPreference = 'Continue'

$stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
$session = Join-Path $OutDir $stamp
New-Item -ItemType Directory -Path $session -Force | Out-Null

$logPath = Join-Path $session 'smoke.log'

function Write-SmokeLog {
    param([string]$Message)
    $line = '[{0}] {1}' -f (Get-Date -Format 'HH:mm:ss'), $Message
    Write-Host $line
    Add-Content -Encoding utf8 -Path $logPath -Value $line
}

# Runs one native command and captures stdout+stderr merged as PLAIN
# TEXT — 5.1 renders a failed native call's stderr as a
# NativeCommandError record rather than a string, and writing that to
# a file verbatim is useless for a report, so ForEach-Object { "$_" }
# stringifies every line first — plus the exit code, to its own file
# under $session named after $Name.
function Invoke-Captured {
    param(
        [Parameter(Mandatory)][string]$Name,
        [Parameter(Mandatory)][string]$Exe,
        [string[]]$Arguments = @()
    )
    Write-SmokeLog "RUN: $Exe $($Arguments -join ' ')"
    $out = (& $Exe @Arguments 2>&1 | ForEach-Object { "$_" }) -join "`n"
    $rc = $LASTEXITCODE
    $outFile = Join-Path $session "$Name.out.txt"
    Set-Content -Encoding utf8 -Path $outFile -Value $out
    Add-Content -Encoding utf8 -Path $outFile -Value "`n--- exit code: $rc ---"
    Write-SmokeLog "DONE: $Name (exit $rc) -> $outFile"
    [PSCustomObject]@{ Name = $Name; Output = $out; ExitCode = $rc; File = $outFile }
}

Write-SmokeLog "Windows verification smoke script starting. RigProg=$RigProg Model=$Model Session=$session"

if (-not (Test-Path -LiteralPath $RigProg)) {
    Write-SmokeLog "FATAL: rigprog.exe not found at $RigProg"
    exit 1
}

# --- 1. Host facts ---------------------------------------------------
Write-SmokeLog 'Capturing Get-ComputerInfo subset (OS build, architecture)'
Get-ComputerInfo -Property OsName, OsArchitecture, OsBuildNumber, OsVersion, WindowsProductName |
    Format-List | Out-File -Encoding utf8 -FilePath (Join-Path $session 'computerinfo.txt')

# --- 2. CP2105 device + driver ----------------------------------------
# Wrapped in @(...): under StrictMode, a Where-Object/foreach pipeline
# that matches nothing collapses to $null and one that matches exactly
# one collapses to a bare scalar rather than a one-element array —
# either would trip a later foreach or .Count on the no-device
# contingency path this script exists to handle cleanly.
Write-SmokeLog "Capturing Get-PnpDevice for VID_10C4 (the FT-710's Silicon Labs CP2105)"
$cp2105 = @(Get-PnpDevice | Where-Object { $_.InstanceId -match 'VID_10C4' })
# Format-List, not Format-Table -AutoSize: Out-File in Windows
# PowerShell 5.1 defaults to the host width (80 with no console), and
# Format-Table truncates columns to fit — this is exactly the
# InstanceId/driver evidence the verification bar exists to collect.
$cp2105 | Format-List | Out-File -Encoding utf8 -FilePath (Join-Path $session 'pnpdevice.txt')

$driverProps = @(foreach ($dev in $cp2105) {
    Get-PnpDeviceProperty -InstanceId $dev.InstanceId -KeyName `
        'DEVPKEY_Device_DriverVersion', 'DEVPKEY_Device_DriverProvider', 'DEVPKEY_Device_DriverDesc' `
        -ErrorAction SilentlyContinue
})
$driverProps | Format-List | Out-File -Encoding utf8 -FilePath (Join-Path $session 'pnpdevice-driver.txt')

# --- 3. Serial port inventory -------------------------------------------
Write-SmokeLog 'Capturing Get-CimInstance Win32_SerialPort'
Get-CimInstance Win32_SerialPort | Format-List |
    Out-File -Encoding utf8 -FilePath (Join-Path $session 'win32-serialport.txt')

Write-SmokeLog 'Capturing HKLM:\HARDWARE\DEVICEMAP\SERIALCOMM'
Get-ItemProperty -Path 'HKLM:\HARDWARE\DEVICEMAP\SERIALCOMM' -ErrorAction SilentlyContinue |
    Format-List | Out-File -Encoding utf8 -FilePath (Join-Path $session 'serialcomm.txt')

# --- 4. rigprog version + ports ------------------------------------------
Invoke-Captured -Name 'version' -Exe $RigProg -Arguments @('version') | Out-Null
$portsResult = Invoke-Captured -Name 'ports' -Exe $RigProg -Arguments @('ports')

# rigprog's own port table is the thing under test here, so this pulls
# COMn tokens out of its printed text with a plain regex rather than
# relying on any structured rigprog output format. Wrapped in @(...):
# under StrictMode this pipeline collapses to $null with zero matches
# and to a bare scalar with exactly one, either of which would trip
# the .Count read below on the no-COM-ports contingency path.
$comPorts = @([regex]::Matches($portsResult.Output, 'COM\d+') |
    ForEach-Object { $_.Value } | Select-Object -Unique)
Write-SmokeLog "COM ports found in 'rigprog ports' output: $($comPorts -join ', ')"

if ($comPorts.Count -eq 0) {
    Write-SmokeLog 'FATAL: no COM ports found in rigprog ports output — nothing to probe'
    exit 1
}

# --- 5. Probe every COM port (read-only; expect exactly one success) ----
$answered = @()
foreach ($port in $comPorts) {
    $result = Invoke-Captured -Name "probe-$port" -Exe $RigProg -Arguments @('probe', '--port', $port, '--model', $Model)
    if ($result.ExitCode -eq 0) {
        $answered += $port
    }
}

Write-SmokeLog "Ports that answered probe: $($answered -join ', ') (expected: exactly one)"
if ($answered.Count -ne 1) {
    Write-SmokeLog "WARNING: expected exactly one port to answer probe; got $($answered.Count)."
}

if ($answered.Count -eq 0) {
    Write-SmokeLog 'FATAL: no COM port answered probe — stopping before the read/export/import round trip'
    exit 1
}
$catPort = $answered[0]
Write-SmokeLog "Using $catPort as the CAT port for the read/export/import round trip"

# --- 6. Read, export, import round trip (nothing here writes to the
#        radio — read, export and import all run without a port open,
#        except the one `read` call, which sends only read commands) ---
$baselineJson = Join-Path $session 'baseline.json'
$baselineCsv = Join-Path $session 'baseline.csv'
$roundtripJson = Join-Path $session 'roundtrip.json'

$readResult = Invoke-Captured -Name 'read' -Exe $RigProg -Arguments @('read', '--port', $catPort, '--model', $Model, '--out', $baselineJson)
if ($readResult.ExitCode -ne 0) {
    Write-SmokeLog 'FATAL: rigprog read failed — stopping before export/import'
    exit 1
}

Invoke-Captured -Name 'export' -Exe $RigProg -Arguments @('export', '--csv', $baselineCsv, $baselineJson) | Out-Null
# No --force: $roundtripJson is a fresh path under this run's timestamped
# $session directory and $baselineJson (--into) never equals it, so
# rigprog's overwrite guard can never fire here — passing --force would
# silently widen a guard this script has no need to widen.
Invoke-Captured -Name 'import' -Exe $RigProg -Arguments @('import', '--csv', $baselineCsv, '--into', $baselineJson, '--out', $roundtripJson, '--model', $Model) | Out-Null

# --- 7. Compare baseline.json and roundtrip.json, normalising read_at
# and generator ---
# read_at is a wall-clock timestamp set at read time; export and import
# never touch it, so this is expected to already be identical, but
# normalise it anyway (a regex replace on the raw text, not a
# JSON-aware reparse, so the rest of the file is compared exactly as
# written) so a clock-formatting quirk cannot masquerade as, or hide, a
# real discrepancy. generator is normalised the same way: `read` sets
# it via clone.Service (generatorID "open-rig-programmer/core/clone"),
# while `import` always overwrites it with the CLI's own
# "rigprog/<version>" stamp (cmd/rigprog/import.go, base.Generator =
# cliGeneratorID) — so a round trip through import always changes this
# field even when nothing else does. Confirmed the ONLY difference on
# the 05/09/2026 VM run (evidence/vm-smoke/20260905-101214/): the
# round-trip compare said DIFFERENT before this fix, purely because of
# the generator field, with every channel identical.
function Get-NormalisedJsonText {
    param([string]$Path)
    (Get-Content -LiteralPath $Path -Raw) `
        -replace '"read_at"\s*:\s*"[^"]*"', '"read_at":"NORMALISED"' `
        -replace '"generator"\s*:\s*"[^"]*"', '"generator":"NORMALISED"'
}

$baselineNorm = Get-NormalisedJsonText -Path $baselineJson
$roundtripNorm = Get-NormalisedJsonText -Path $roundtripJson
$identical = $baselineNorm -ceq $roundtripNorm

$compareReport = Join-Path $session 'compare-baseline-roundtrip.txt'
if ($identical) {
    "IDENTICAL after normalising read_at and generator: $baselineJson vs $roundtripJson" | Set-Content -Encoding utf8 -Path $compareReport
    Write-SmokeLog 'Export/import round trip: IDENTICAL (after normalising read_at and generator)'
} else {
    "DIFFERENT after normalising read_at and generator: $baselineJson vs $roundtripJson" | Set-Content -Encoding utf8 -Path $compareReport
    Write-SmokeLog 'Export/import round trip: DIFFERENT — see compare-baseline-roundtrip.txt. This is a FAIL: do not continue to the writes in scripts/README-vm-leg.md until this is understood.'
}

Write-SmokeLog "Smoke script complete. Evidence in: $session"
Write-SmokeLog 'This script made NO write to the radio. Continue with scripts/README-vm-leg.md for the two writes this milestone makes, by hand.'
