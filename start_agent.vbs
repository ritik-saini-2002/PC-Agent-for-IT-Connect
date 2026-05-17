' ═══════════════════════════════════════════════════════════
'  start_agent.vbs
'  Launches PC Command Agent v12 with NO console window
'  Double-click this file to start the agent silently
'
'  Place this .vbs file in the SAME folder as agent_v12.py
'  Then double-click start_agent.vbs
' ═══════════════════════════════════════════════════════════

Dim objShell, objFSO
Set objShell = CreateObject("WScript.Shell")
Set objFSO   = CreateObject("Scripting.FileSystemObject")

' ── Get the folder where this .vbs file lives ──────────────
Dim scriptDir
scriptDir = objFSO.GetParentFolderName(WScript.ScriptFullName)

' ── Full paths ─────────────────────────────────────────────
Dim agentPath, logPath
agentPath = scriptDir & "\agent_v12.py"
logPath   = scriptDir & "\startup_error.log"

' ── Check agent file exists before trying to launch ────────
If Not objFSO.FileExists(agentPath) Then
    MsgBox "ERROR: agent_v12.py not found at:" & vbCrLf & agentPath & vbCrLf & vbCrLf & _
           "Make sure start_agent.vbs and agent_v12.py are in the same folder.", _
           vbCritical, "PC Agent - File Not Found"
    WScript.Quit 1
End If

' ── Find pythonw.exe (no console window) ───────────────────
Dim pythonw
pythonw = ""

' Check if pythonw is in PATH
On Error Resume Next
Dim oExec
Set oExec = objShell.Exec("cmd /c where pythonw 2>nul")
If Err.Number = 0 Then
    Dim result
    result = oExec.StdOut.ReadAll()
    result = Trim(result)
    If result <> "" Then
        ' Take only the first line if multiple results
        Dim lines
        lines = Split(result, vbCrLf)
        pythonw = Trim(lines(0))
    End If
End If
On Error GoTo 0

' Fallback: common install locations
If pythonw = "" Or Not objFSO.FileExists(pythonw) Then
    Dim possiblePaths(6)
    possiblePaths(0) = objShell.ExpandEnvironmentStrings("%LOCALAPPDATA%") & "\Programs\Python\Python312\pythonw.exe"
    possiblePaths(1) = objShell.ExpandEnvironmentStrings("%LOCALAPPDATA%") & "\Programs\Python\Python311\pythonw.exe"
    possiblePaths(2) = objShell.ExpandEnvironmentStrings("%LOCALAPPDATA%") & "\Programs\Python\Python310\pythonw.exe"
    possiblePaths(3) = "C:\Python312\pythonw.exe"
    possiblePaths(4) = "C:\Python311\pythonw.exe"
    possiblePaths(5) = "C:\Python310\pythonw.exe"
    possiblePaths(6) = "C:\Python39\pythonw.exe"

    Dim i
    For i = 0 To 6
        If objFSO.FileExists(possiblePaths(i)) Then
            pythonw = possiblePaths(i)
            Exit For
        End If
    Next
End If

' Last resort: fall back to python.exe (may briefly flash a window)
If pythonw = "" Then
    On Error Resume Next
    Set oExec = objShell.Exec("cmd /c where python 2>nul")
    If Err.Number = 0 Then
        result = Trim(oExec.StdOut.ReadAll())
        If result <> "" Then
            lines = Split(result, vbCrLf)
            pythonw = Trim(lines(0))
        End If
    End If
    On Error GoTo 0
End If

If pythonw = "" Then
    MsgBox "ERROR: Python not found on this system." & vbCrLf & vbCrLf & _
           "Please install Python from https://www.python.org/downloads/" & vbCrLf & _
           "and make sure to check 'Add Python to PATH' during install.", _
           vbCritical, "PC Agent - Python Not Found"
    WScript.Quit 1
End If

' ── Check if agent is already running on port 5000 ─────────
Dim alreadyRunning
alreadyRunning = False
On Error Resume Next
Set oExec = objShell.Exec("cmd /c netstat -ano | findstr :5000")
If Err.Number = 0 Then
    Dim netOut
    netOut = oExec.StdOut.ReadAll()
    If InStr(netOut, ":5000") > 0 Then
        alreadyRunning = True
    End If
End If
On Error GoTo 0

If alreadyRunning Then
    MsgBox "PC Command Agent is already running on port 5000." & vbCrLf & vbCrLf & _
           "No action taken. If you want to restart it, stop the existing agent first.", _
           vbInformation, "PC Agent - Already Running"
    WScript.Quit 0
End If

' ── Launch the agent — hidden window, don't wait ───────────
' 0 = hidden window, False = fire and forget (won't block)
On Error Resume Next
objShell.Run """" & pythonw & """ """ & agentPath & """", 0, False
If Err.Number <> 0 Then
    MsgBox "ERROR: Failed to launch agent." & vbCrLf & vbCrLf & _
           "Command: " & pythonw & vbCrLf & _
           "Agent  : " & agentPath & vbCrLf & vbCrLf & _
           "Error  : " & Err.Description, _
           vbCritical, "PC Agent - Launch Failed"
    WScript.Quit 1
End If
On Error GoTo 0

' ── Small delay then verify it actually started ────────────
WScript.Sleep 4000

Dim started
started = False
On Error Resume Next
Set oExec = objShell.Exec("cmd /c netstat -ano | findstr :5000")
If Err.Number = 0 Then
    netOut = oExec.StdOut.ReadAll()
    If InStr(netOut, ":5000") > 0 Then
        started = True
    End If
End If
On Error GoTo 0

If Not started Then
    Dim errMsg
    errMsg = "Agent launched but port 5000 is NOT listening." & vbCrLf & vbCrLf & _
             "Possible reasons:" & vbCrLf & _
             "  - A required Python library is missing" & vbCrLf & _
             "  - agent_v12.py has a syntax error" & vbCrLf & _
             "  - Port 5000 is blocked by another app" & vbCrLf & vbCrLf & _
             "Run agent_manager.bat → option D (Debug) to see the real error."

    ' Also check if there is a startup_error.log to show
    If objFSO.FileExists(logPath) Then
        Dim oFile, logContent
        Set oFile = objFSO.OpenTextFile(logPath, 1)
        logContent = oFile.ReadAll()
        oFile.Close
        If Trim(logContent) <> "" Then
            errMsg = errMsg & vbCrLf & vbCrLf & "--- startup_error.log ---" & vbCrLf & Left(logContent, 500)
        End If
    End If

    MsgBox errMsg, vbExclamation, "PC Agent - May Not Have Started"
End If

' ── Cleanup ────────────────────────────────────────────────
Set objShell = Nothing
Set objFSO   = Nothing