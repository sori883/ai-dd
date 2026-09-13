param(
    [Parameter(Mandatory=$true)][string]$Version,
    [Parameter(Mandatory=$true)][string]$ProjectDirectory
)
$ErrorActionPreference = 'Stop'
$tempDirectory = $null
$exitCode = 1
try {
    if ($Version -notmatch '^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$' -or $Version.Contains('..')) { throw 'Invalid release version' }
    $project = Get-Item -LiteralPath $ProjectDirectory
    if (-not $project.PSIsContainer -or $project.PSProvider.Name -ne 'FileSystem') { throw 'Project must be an existing filesystem directory' }
    $projectPath = $project.FullName
    $curl = Get-Command curl.exe -CommandType Application -TotalCount 1 -ErrorAction Stop
    $architecture = [System.Runtime.InteropServices.RuntimeInformation]::ProcessArchitecture.ToString()
    switch ($architecture) {
        'X64' { $target = 'windows_amd64' }
        'Arm64' { $target = 'windows_arm64' }
        default { throw "Unsupported process architecture: $architecture" }
    }
    Add-Type -AssemblyName System.IO.Compression
    $tempDirectory = Join-Path ([System.IO.Path]::GetTempPath()) ('ai-dd-bootstrap.' + [Guid]::NewGuid().ToString('N'))
    [System.IO.Directory]::CreateDirectory($tempDirectory) | Out-Null
    function Fetch-Asset([string]$Name, [long]$Limit) {
        $destination = Join-Path $tempDirectory $Name
        $url = 'https://github.com/sori883/ai-dd/releases/download/' + $Version + '/' + $Name
        $start = New-Object System.Diagnostics.ProcessStartInfo
        $start.FileName = $curl.Path
        # All arguments here are fixed ASCII or the validated release version.
        $start.Arguments = '--disable --proto =https --proto-redir =https --location --fail --silent --show-error --connect-timeout 30 --max-time 120 --max-filesize ' + $Limit + ' --output - ' + $url
        $start.UseShellExecute = $false
        $start.RedirectStandardOutput = $true
        $process = New-Object System.Diagnostics.Process
        $process.StartInfo = $start
        $output = $null
        $started = $false
        try {
            try { $started = $process.Start() } catch { throw ('Cannot start curl.exe: ' + $_.Exception.Message) }
            if (-not $started) { throw 'Cannot start curl.exe: Process.Start returned false' }
            $output = [System.IO.File]::Open($destination, [System.IO.FileMode]::CreateNew)
            $buffer = New-Object byte[] 65536
            [long]$total = 0
            while ($true) {
                $count = $process.StandardOutput.BaseStream.Read($buffer, 0, [int][Math]::Min($buffer.Length, $Limit - $total + 1))
                if ($count -eq 0) { break }
                $total += $count
                if ($total -gt $Limit) { throw 'Download exceeds size limit' }
                $output.Write($buffer, 0, $count)
            }
            $process.WaitForExit()
            if ($process.ExitCode -ne 0) { throw "Download failed: $Name" }
        } finally {
            if ($null -ne $output) { $output.Dispose() }
            if ($started -and -not $process.HasExited) { $process.Kill(); $process.WaitForExit() }
            $process.Dispose()
        }
    }
    Fetch-Asset 'SHA256SUMS' 4096
    $sums = [System.IO.File]::ReadAllText((Join-Path $tempDirectory 'SHA256SUMS'))
    $lines = $sums.Split([char]10)
    $targets = @('darwin_amd64','darwin_arm64','linux_amd64','linux_arm64','windows_amd64','windows_arm64')
    if ($lines.Length -ne 7 -or $lines[6] -ne '') { throw 'Invalid SHA256SUMS line count' }
    $expected = $null
    for ($i=0; $i -lt 6; $i++) {
        $suffix = '.tar.gz'; if ($i -ge 4) { $suffix = '.zip' }
        $name = 'ai-dd_' + $Version + '_' + $targets[$i] + $suffix
        if ($lines[$i] -cnotmatch ('^[0-9a-f]{64}  ' + [Regex]::Escape($name) + '$')) { throw 'Invalid SHA256SUMS entry' }
        if ($targets[$i] -eq $target) { $expected = $lines[$i].Substring(0,64) }
    }
    $archiveName = 'ai-dd_' + $Version + '_' + $target + '.zip'
    Fetch-Asset $archiveName 536870912
    $archivePath = Join-Path $tempDirectory $archiveName
    $actual = (Get-FileHash -LiteralPath $archivePath -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actual -cne $expected) { throw 'Archive checksum mismatch' }
    $stream = [System.IO.File]::OpenRead($archivePath)
    $zip = $null
    try {
        $zip = New-Object System.IO.Compression.ZipArchive($stream, [System.IO.Compression.ZipArchiveMode]::Read)
        $members = @($zip.Entries | Where-Object { $_.FullName -ceq 'aidlc-install.exe' })
        if ($members.Count -ne 1) { throw 'Installer must occur exactly once' }
        $member = $members[0]
        $unixType = ($member.ExternalAttributes -shr 16) -band 61440
        if (($unixType -ne 0 -and $unixType -ne 32768) -or ($member.ExternalAttributes -band 16) -ne 0) { throw 'Installer must be a regular file' }
        if ($member.Length -le 0 -or $member.Length -gt 67108864) { throw 'Installer exceeds size limit' }
        $installerPath = Join-Path $tempDirectory 'aidlc-install.exe'
        $inputStream = $member.Open()
        $outputStream = [System.IO.File]::Open($installerPath, [System.IO.FileMode]::CreateNew)
        try {
            $buffer = New-Object byte[] 65536
            [long]$total = 0
            while (($count = $inputStream.Read($buffer,0,$buffer.Length)) -gt 0) {
                $total += $count
                if ($total -gt 67108864 -or $total -gt $member.Length) { throw 'Installer exceeds size limit' }
                $outputStream.Write($buffer,0,$count)
            }
            if ($total -ne $member.Length) { throw 'Installer length mismatch' }
        } finally { $inputStream.Dispose(); $outputStream.Dispose() }
    } finally { if ($null -ne $zip) { $zip.Dispose() }; $stream.Dispose() }
    & $installerPath codex --release-version $Version --project-dir $projectPath --release-dir $tempDirectory
    $exitCode = $LASTEXITCODE
} catch {
    [Console]::Error.WriteLine('AI-DD: ' + $_.Exception.Message)
} finally {
    if ($null -ne $tempDirectory) { Remove-Item -LiteralPath $tempDirectory -Recurse -Force }
}
# A downloaded ScriptBlock must return to its caller; -File retains process status.
if ($MyInvocation.MyCommand -is [System.Management.Automation.ExternalScriptInfo]) {
    exit $exitCode
}
Set-Variable -Name LASTEXITCODE -Value $exitCode -Scope 1
return
