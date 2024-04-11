@echo off
setlocal enabledelayedexpansion

set hugo_prefix=
set hugo_version=
if not "%~1"=="" (
    set hugo_prefix=.hugo
    set hugo_version=.%1
)

rem Relearn themes exampleSite
set "themeDir=.."
if not exist "%themeDir%\layouts\partials\version.txt" (
    rem other sites stored parallel to Relearn themes directory 
    set "themeDir=..\hugo-theme-relearn"
)
if not exist "%themeDir%\layouts\partials\version.txt" (
    rem other sites stored inside of repos directory parallel to Relearn theme directory
    set "themeDir=..\..\hugo-theme-relearn"
)
if not exist "%themeDir%\layouts\partials\version.txt" (
    rem regular theme users
    set "themeDir=themes\hugo-theme-relearn"
)
if not exist "%themeDir%\layouts\partials\version.txt" (
    rem theme users with custom theme name
    set "themeDir=themes\relearn"
)
if not exist "%themeDir%\layouts\partials\version.txt" (
    echo Relearn theme not found.
    set "version="
) else (
    set /p version=<"%themeDir%\layouts\partials\version.txt"
    set "version=.!version!"
)
echo %version%>"metrics%version%%hugo_prefix%%hugo_version%.log"

set config=--environment testing
if exist "config\testing" (
    rem seems we are in the themes exampleSite, no need to copy anything
) else if exist "config.toml" (
    set config=--config config.toml,%themeDir%\exampleSite\config\testing\hugo.toml
) else if exist "hugo.toml" (
    set config=--config hugo.toml,%themeDir%\exampleSite\config\testing\hugo.toml
) else if exist "config" (
    copy /e /i /y "%themeDir%\exampleSite\config\testing" "config\testing"
) else if exist "hugo" (
    copy /e /i /y "%themeDir%\config\testing" "hugo\testing"
)

echo on
hugo%hugo_version% %config% --printPathWarnings --templateMetrics --templateMetricsHints --cleanDestinationDir --logLevel info --destination "public%version%%hugo_prefix%%hugo_version%" >> "metrics%version%%hugo_prefix%%hugo_version%.log"
@echo off

set "start_dir=%CD%\public%version%%hugo_prefix%%hugo_version%"
set "output_file=dir%version%%hugo_prefix%%hugo_version%.log"
if exist "%output_file%" del "%output_file%"
for /r "%start_dir%" %%F in (*) do (
    set "file=%%F"
    echo !file:%start_dir%\=! >> "%output_file%"
)
move /Y "dir%version%%hugo_prefix%%hugo_version%.log" "public%version%%hugo_prefix%%hugo_version%\dir.log" 2>&1 >NUL

move /Y "metrics%version%%hugo_prefix%%hugo_version%.log" "public%version%%hugo_prefix%%hugo_version%\metrics.log" 2>&1 >NUL
For /F "UseBackQ Delims==" %%A In ("public%version%%hugo_prefix%%hugo_version%\metrics.log") Do Set "lastline=%%A"
echo %lastline%
