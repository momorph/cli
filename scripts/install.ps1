#
# MoMorph CLI Installer for Windows
# Usage: irm https://momorph.ai/cli/stable/install.ps1 | iex
#
# Environment variables:
#   VERSION           - Specific version to install (default: latest)
#   INSTALL_DIR       - Installation directory (default: $env:LOCALAPPDATA\MoMorph)
#

$ErrorActionPreference = "Stop"

# Configuration
$GitHubRepo = "momorph/cli"
$BinaryName = "momorph.exe"
$DefaultInstallDir = "$env:LOCALAPPDATA\MoMorph"

# Print colored message
function Write-Info {
    param([string]$Message)
    Write-Host "[INFO] " -ForegroundColor Blue -NoNewline
    Write-Host $Message
}

function Write-Success {
    param([string]$Message)
    Write-Host "[SUCCESS] " -ForegroundColor Green -NoNewline
    Write-Host $Message
}

function Write-Warning {
    param([string]$Message)
    Write-Host "[WARNING] " -ForegroundColor Yellow -NoNewline
    Write-Host $Message
}

function Write-Error {
    param([string]$Message)
    Write-Host "[ERROR] " -ForegroundColor Red -NoNewline
    Write-Host $Message
}

# Detect architecture
function Get-Architecture {
    $arch = [System.Environment]::GetEnvironmentVariable("PROCESSOR_ARCHITECTURE")
    switch ($arch) {
        "AMD64" { return "amd64" }
        "ARM64" { return "arm64" }
        "x86"   { return "386" }
        default {
            Write-Error "Unsupported architecture: $arch"
            exit 1
        }
    }
}

# Get latest version from GitHub API
function Get-LatestVersion {
    try {
        $response = Invoke-RestMethod -Uri "https://api.github.com/repos/$GitHubRepo/releases/latest" -UseBasicParsing
        return $response.tag_name
    }
    catch {
        Write-Error "Failed to fetch latest version: $_"
        exit 1
    }
}

# Get current installed version
function Get-CurrentVersion {
    $momorphPath = Get-Command momorph -ErrorAction SilentlyContinue
    if ($momorphPath) {
        try {
            $output = & momorph version 2>$null
            if ($output -match "Version:\s*(\d+\.\d+\.\d+)") {
                return "v$($Matches[1])"
            }
        }
        catch {
            return $null
        }
    }
    return $null
}

# Check if installed via Chocolatey
function Get-PackageManager {
    $momorphPath = Get-Command momorph -ErrorAction SilentlyContinue
    if (-not $momorphPath) {
        return "none"
    }

    $path = $momorphPath.Source
    if ($path -like "*\chocolatey\*") {
        return "chocolatey"
    }
    return "manual"
}

# Download and verify checksum
function Install-MoMorphCLI {
    param(
        [string]$Version,
        [string]$Arch,
        [string]$InstallDir
    )

    # Create temp directory
    $tempDir = Join-Path $env:TEMP "momorph-install-$(Get-Random)"
    New-Item -ItemType Directory -Path $tempDir -Force | Out-Null

    try {
        # Remove 'v' prefix for filename
        $versionNum = $Version.TrimStart('v')
        $filename = "momorph-cli_${versionNum}_windows_${Arch}.zip"
        $downloadUrl = "https://github.com/$GitHubRepo/releases/download/$Version/$filename"
        $checksumsUrl = "https://github.com/$GitHubRepo/releases/download/$Version/checksums.txt"

        Write-Info "Downloading MoMorph CLI $Version for windows/$Arch..."

        # Download binary
        $zipPath = Join-Path $tempDir $filename
        try {
            Invoke-WebRequest -Uri $downloadUrl -OutFile $zipPath -UseBasicParsing
        }
        catch {
            Write-Error "Failed to download: $downloadUrl"
            exit 1
        }

        # Download checksums
        $checksumsPath = Join-Path $tempDir "checksums.txt"
        try {
            Invoke-WebRequest -Uri $checksumsUrl -OutFile $checksumsPath -UseBasicParsing
        }
        catch {
            Write-Error "Failed to download checksums"
            exit 1
        }

        # Verify checksum
        Write-Info "Verifying checksum..."
        $checksums = Get-Content $checksumsPath
        $expectedChecksum = ($checksums | Where-Object { $_ -match $filename } | ForEach-Object { ($_ -split '\s+')[0] })

        if (-not $expectedChecksum) {
            Write-Error "Checksum not found for $filename"
            exit 1
        }

        $actualChecksum = (Get-FileHash -Path $zipPath -Algorithm SHA256).Hash.ToLower()

        if ($expectedChecksum -ne $actualChecksum) {
            Write-Error "Checksum verification failed!"
            Write-Error "Expected: $expectedChecksum"
            Write-Error "Actual:   $actualChecksum"
            exit 1
        }

        Write-Success "Checksum verified"

        # Extract binary
        Write-Info "Extracting..."
        Expand-Archive -Path $zipPath -DestinationPath $tempDir -Force

        # Create install directory if needed
        if (-not (Test-Path $InstallDir)) {
            Write-Info "Creating directory $InstallDir..."
            New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
        }

        # Install binary
        Write-Info "Installing to $InstallDir..."
        $sourcePath = Join-Path $tempDir $BinaryName
        $destPath = Join-Path $InstallDir $BinaryName
        Copy-Item -Path $sourcePath -Destination $destPath -Force
    }
    finally {
        # Cleanup
        if (Test-Path $tempDir) {
            Remove-Item -Path $tempDir -Recurse -Force -ErrorAction SilentlyContinue
        }
    }
}

# Add to PATH
function Add-ToPath {
    param([string]$Directory)

    $currentPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($currentPath -notlike "*$Directory*") {
        Write-Info "Adding $Directory to user PATH..."
        $newPath = "$Directory;$currentPath"
        [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
        $env:Path = "$Directory;$env:Path"
        return $true
    }
    return $false
}

# Verify installation
function Test-Installation {
    param(
        [string]$InstallDir,
        [string]$ExpectedVersion
    )

    $binaryPath = Join-Path $InstallDir $BinaryName

    if (Test-Path $binaryPath) {
        try {
            $output = & $binaryPath version 2>$null
            if ($output -match "Version:\s*(\d+\.\d+\.\d+)") {
                $installedVersion = "v$($Matches[1])"
            }
            else {
                $installedVersion = "unknown"
            }
        }
        catch {
            $installedVersion = "unknown"
        }

        Write-Success "MoMorph CLI installed successfully!"
        Write-Info "Version: $installedVersion"
        Write-Info "Location: $binaryPath"

        # Check PATH
        $momorphCmd = Get-Command momorph -ErrorAction SilentlyContinue
        if ($momorphCmd -and $momorphCmd.Source -ne $binaryPath) {
            Write-Host ""
            Write-Warning "Another momorph binary has higher PATH priority:"
            Write-Warning "  Active: $($momorphCmd.Source)"
            Write-Warning "  Installed: $binaryPath"
            Write-Host ""
            Write-Info "To use the newly installed version, either:"
            Write-Host "  1. Uninstall the other version (e.g., 'choco uninstall momorph-cli')"
            Write-Host "  2. Restart your terminal to refresh PATH"
            Write-Host "  3. Run directly: $binaryPath"
        }

        Write-Host ""
        Write-Info "Get started with:"
        Write-Host "  momorph login     # Authenticate with GitHub"
        Write-Host "  momorph init .    # Initialize a MoMorph project"
        Write-Host "  momorph --help    # Show help"
    }
    else {
        Write-Error "Installation verification failed"
        exit 1
    }
}

# Prompt for confirmation
function Confirm-Action {
    param(
        [string]$Prompt,
        [bool]$Default = $true
    )

    if ($Default) {
        $promptText = "$Prompt [Y/n] "
    }
    else {
        $promptText = "$Prompt [y/N] "
    }

    Write-Host $promptText -ForegroundColor Cyan -NoNewline

    # Check if running interactively
    if ([Environment]::UserInteractive -and [Console]::KeyAvailable -eq $false) {
        $response = Read-Host
        if ([string]::IsNullOrWhiteSpace($response)) {
            return $Default
        }
        return $response -match '^[yY]'
    }
    else {
        # Non-interactive, use default
        Write-Host ""
        return $Default
    }
}

# Main
function Main {
    Write-Host ""
    Write-Host "======================================" -ForegroundColor Blue
    Write-Host "       MoMorph CLI Installer          " -ForegroundColor White
    Write-Host "======================================" -ForegroundColor Blue
    Write-Host ""

    # Detect platform
    $arch = Get-Architecture
    Write-Info "Detected platform: windows/$arch"

    # Get target version
    if ($env:VERSION) {
        $targetVersion = $env:VERSION
        if (-not $targetVersion.StartsWith('v')) {
            $targetVersion = "v$targetVersion"
        }
        Write-Info "Target version: $targetVersion"
    }
    else {
        Write-Info "Fetching latest version..."
        $targetVersion = Get-LatestVersion
        Write-Info "Latest version: $targetVersion"
    }

    # Get current version
    $currentVersion = Get-CurrentVersion
    if ($currentVersion) {
        Write-Info "Current version: $currentVersion"
    }

    # Check if already up to date
    if ($currentVersion -eq $targetVersion) {
        Write-Success "MoMorph CLI is already up to date"
        exit 0
    }

    # Detect package manager installation
    $pkgManager = Get-PackageManager

    # Determine install directory
    $installDir = if ($env:INSTALL_DIR) { $env:INSTALL_DIR } else { $DefaultInstallDir }

    if ($pkgManager -eq "chocolatey") {
        Write-Host ""
        Write-Warning "MoMorph CLI $currentVersion is currently installed via Chocolatey."
        Write-Warning "Installing via this script will create a separate installation."
        Write-Host ""
        Write-Info "Recommended: Update via Chocolatey instead:"
        Write-Host "  choco upgrade momorph-cli"
        Write-Host ""

        if (-not (Confirm-Action "Do you want to continue with manual installation anyway?")) {
            Write-Info "Installation cancelled."
            exit 0
        }

        Write-Host ""
        Write-Info "To use the manually installed version after installation,"
        Write-Info "you may need to uninstall the Chocolatey version:"
        Write-Host "  choco uninstall momorph-cli"
        Write-Host ""
    }
    elseif ($currentVersion) {
        Write-Host ""
        if (-not (Confirm-Action "Upgrade MoMorph CLI from $currentVersion to $targetVersion?")) {
            Write-Info "Installation cancelled."
            exit 0
        }
        Write-Host ""
    }
    else {
        Write-Host ""
        if (-not (Confirm-Action "Install MoMorph CLI $targetVersion?")) {
            Write-Info "Installation cancelled."
            exit 0
        }
        Write-Host ""
    }

    # Download and install
    Install-MoMorphCLI -Version $targetVersion -Arch $arch -InstallDir $installDir

    # Add to PATH
    $pathAdded = Add-ToPath -Directory $installDir
    if ($pathAdded) {
        Write-Info "Added to PATH. Please restart your terminal for changes to take effect."
    }

    # Verify
    Test-Installation -InstallDir $installDir -ExpectedVersion $targetVersion
}

# Run main
Main
