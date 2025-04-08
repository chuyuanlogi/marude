@echo off
setlocal enabledelayedexpansion

set APP_NAME=marude
set SERVICE_NAME=marude
set FOLDER_NAME=C:\marude
set DISPLAY_NAME=marude
set DESC="marude"

net session >nul 2>&1
if %errorLevel% neq 0 (
    echo please run this batch file as administrator
    exit /b
)

if "%~1"=="" (
    call :help
    exit /b
)

if "%~2"=="client" (
    set APP_NAME=marude_client.exe
	set SERVICE_NAME="marude_client"
	set DISPLAY_NAME="marude client"
	set DESC="marude client service"
) else if "%~2"=="server" (
    set APP_NAME=marude_server.exe
	set SERVICE_NAME="marude_server"
	set DISPLAY_NAME="marude server"
	set DESC="marude server service"
) else if "%~2"=="extend" (
    set APP_NAME=marude_extend.exe
	set SERVICE_NAME="marude_extend"
	set DISPLAY_NAME="marude entend"
	set DESC="marude extend service"
)

if "%~1"=="install" (
    call :install
) else if "%~1"=="update" (
    call :update
) else if "%~1"=="start" (
    call :start
) else if "%~1"=="stop" (
    call :stop
) else if "%~1"=="restart" (
    call :restart
) else if "%~1"=="uninstall" (
    call :uninstall
) else if "%~1"=="status" (
    call :status
) else (
    echo invalide command: %~1
    call :help
    exit /b 1
)

exit /b

:install
    echo installing...
	if not exist %FOLDER_NAME% md %FOLDER_NAME%
	if not exist %ProgramData%/marude md %ProgramData%/marude
	copy /y %APP_NAME% %FOLDER_NAME%
	copy /y conf/*.sample %ProgramData%/marude
    sc create %SERVICE_NAME% binPath= %FOLDER_NAME%/%APP_NAME% DisplayName= %DISPLAY_NAME% start= auto
    sc description %SERVICE_NAME% %DESC%
    sc failure %SERVICE_NAME% reset= 86400 actions= restart/5000
    echo finished install
    goto :eof

:update
    echo updating...
	call :stop
	timeout /t 2 >nul
	copy /y %APP_NAME% %FOLDER_NAME%
	call :start
    echo finished update
    goto :eof

:start
    echo starting..
    sc start %SERVICE_NAME%
    goto :eof

:stop
    echo stop...
    sc stop %SERVICE_NAME%
    goto :eof

:restart
    call :stop
    timeout /t 2 >nul
    call :start
    goto :eof

:uninstall
    echo uninstalling...
    call :stop 2>nul
    sc delete %SERVICE_NAME%
	rd /s/q %ProgramData%/marude
	rd /s/q %FOLDER_NAME%
    echo finished uninstall
    goto :eof

:status
    sc query %SERVICE_NAME%
    goto :eof

:help
    echo usage: %~nx0 [command] [process]
    echo.
    echo   install     - install process as a service
    echo   update      - update service
    echo   start       - start service
    echo   stop        - stop service
    echo   restart     - restart servicd
    echo   uninstall   - uninstall service
    echo   status      - check the service status
    goto :eof

endlocal