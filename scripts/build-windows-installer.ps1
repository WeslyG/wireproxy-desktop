param(
    [Parameter(Mandatory)][ValidatePattern('^v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$', Options = 'None')][string]$Tag,
    [Parameter(Mandatory)][ValidateSet('amd64', 'arm64')][string]$Architecture,
    [Parameter(Mandatory)][string]$Binary,
    [Parameter(Mandatory)][string]$OutputDirectory
)

$ErrorActionPreference = 'Stop'
$binaryPath = (Resolve-Path -LiteralPath $Binary).Path
$outputPath = (New-Item -ItemType Directory -Force -Path $OutputDirectory).FullName
$spec = Join-Path $PSScriptRoot '../packaging/winget/WireproxyDesktop.iss'
$compiler = Get-Command ISCC.exe -ErrorAction SilentlyContinue
$compilerPath = if ($compiler) {
    $compiler.Source
} else {
    Join-Path ${env:ProgramFiles(x86)} 'Inno Setup 6/ISCC.exe'
}
if (!(Test-Path -LiteralPath $compilerPath -PathType Leaf)) {
    throw 'Install Inno Setup 6.3 or later to build the Windows installer.'
}

$outputName = "wireproxy-desktop-setup-windows-$Architecture-$Tag"
$version = $Tag -creplace '^v', ''
& $compilerPath "/DAppVersion=$version" "/DAppArch=$Architecture" "/DAppBinary=$binaryPath" "/O$outputPath" "/F$outputName" $spec
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
if (!(Test-Path -LiteralPath (Join-Path $outputPath "$outputName.exe") -PathType Leaf)) {
    throw 'Inno Setup did not produce the expected installer.'
}
