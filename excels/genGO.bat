@echo off
REM Go server Luban generation: Excel -> D:\Work\gameServerGo\excels (gen=Go code, data=json).
REM Usage: genGO.bat [GoOutputDir]
setlocal
cd /d %~dp0

set "CONF_ROOT=%~dp0"
if "%CONF_ROOT:~-1%"=="\" set "CONF_ROOT=%CONF_ROOT:~0,-1%"
set "WORKSPACE=%~dp0..\..\.."
set "LUBAN_DLL=%WORKSPACE%\Tools\LubanTools\Tools\Luban\Luban.dll"
REM 完整日志落盘，双击闪退后也能打开看；失败时控制台也会回显。
set "LOG_FILE=%CONF_ROOT%\genGO.log"
REM 校验步骤日志（只读校验，不落地）。
set "CHECK_LOG=%CONF_ROOT%\genGO.check.log"
REM 当前执行步骤，失败时用于定位。
set "FAIL_STEP=resolve_dotnet"
call "%~dp0resolve_dotnet.bat"
if errorlevel 1 (
    set "FAIL_LOG="
    goto :fail
)
if not defined DOTNET_EXE set "DOTNET_EXE=dotnet"

REM allow override: genGO.bat D:\path\to\excels
set "GO_OUT=D:\Work\gameServerGo\excels"
if not "%~1"=="" set "GO_OUT=%~1"
set "GO_CODE_DIR=%GO_OUT%\gen"
set "GO_DATA_DIR=%GO_OUT%\data"

set DOTNET_CLI_TELEMETRY_OPTOUT=1
set DOTNET_SKIP_FIRST_TIME_EXPERIENCE=true
set DOTNET_NOLOGO=1
set DOTNET_ROLL_FORWARD=LatestMajor

if not exist "%LUBAN_DLL%" (
    set "FAIL_STEP=check Luban.dll"
    echo [ERROR] Luban.dll not found: %LUBAN_DLL%
    set "FAIL_LOG="
    goto :fail
)
if not exist "%CONF_ROOT%\luban.conf" (
    set "FAIL_STEP=check luban.conf"
    echo [ERROR] luban.conf not found: %CONF_ROOT%\luban.conf
    set "FAIL_LOG="
    goto :fail
)
if not exist "%GO_CODE_DIR%" mkdir "%GO_CODE_DIR%"
if not exist "%GO_DATA_DIR%" mkdir "%GO_DATA_DIR%"

REM NOTE:
REM 1. target "go-server" 以 luban.conf 为准（当前 groups=["s"]），不要与 -t server 混淆。
REM 2. -x lubanGoModule is REQUIRED by Luban GoCodeTargetBase even for go-json
REM    (template itself does not use it); value = Go import path of the gen dir.
REM Step 1/2: 先校验再生成。只读校验（-f 强制加载全部数据，--validationFailAsError 让数据问题直接报错），
REM 不写任何输出文件，失败时 Luban 会打印 文件/Sheet/错误位置/字段，可直接定位到 Excel 单元格。
REM 本地 Luban 1.0.0 暂不支持 --strict/--errorFormat json，先用本命令；升级 Luban 后可再加。
REM 跳过校验：set GEN_GO_SKIP_CHECK=1
set "FAIL_STEP=luban validate(go-server)"
if "%GEN_GO_SKIP_CHECK%"=="1" (
    echo [INFO] [1/2] 跳过校验（GEN_GO_SKIP_CHECK=1）。
    goto :skip_check
)
echo [INFO] [1/2] 校验配置表，日志: %CHECK_LOG%
echo [INFO] "%DOTNET_EXE%" "%LUBAN_DLL%" --conf "%CONF_ROOT%\luban.conf" -t go-server -f --validationFailAsError
"%DOTNET_EXE%" "%LUBAN_DLL%" --conf "%CONF_ROOT%\luban.conf" -t go-server -f --validationFailAsError > "%CHECK_LOG%" 2>&1
set "CHECK_EXIT=%ERRORLEVEL%"
REM 先存退出码再 type（type 会重置 ERRORLEVEL）。
type "%CHECK_LOG%"
if "%CHECK_EXIT%"=="0" goto :check_pass
echo [ERROR] 配置表校验失败，定位看上面的 文件/错误位置/字段 行，完整日志: %CHECK_LOG%
set "EXITCODE=%CHECK_EXIT%"
set "FAIL_LOG=%CHECK_LOG%"
goto :fail
:check_pass
echo [INFO] 校验通过。
:skip_check
set "FAIL_STEP=luban go-server"
set "FAIL_LOG=%LOG_FILE%"
echo [INFO] [2/2] 执行 Luban 生成，完整日志: %LOG_FILE%
echo [INFO] "%DOTNET_EXE%" "%LUBAN_DLL%" -t go-server -c go-json -d json --conf "%CONF_ROOT%\luban.conf" -x outputCodeDir="%GO_CODE_DIR%" -x outputDataDir="%GO_DATA_DIR%" -x lubanGoModule=gameServer/excels/gen
"%DOTNET_EXE%" "%LUBAN_DLL%" -t go-server -c go-json -d json --conf "%CONF_ROOT%\luban.conf" -x outputCodeDir="%GO_CODE_DIR%" -x outputDataDir="%GO_DATA_DIR%" -x lubanGoModule=gameServer/excels/gen > "%LOG_FILE%" 2>&1
set "LUBAN_EXIT=%ERRORLEVEL%"
REM 无论成功失败都回显日志，失败时可直接看到表名/字段/Excel行号。
type "%LOG_FILE%"
if not "%LUBAN_EXIT%"=="0" (
    echo [ERROR] Luban Go generation failed.
    set "EXITCODE=%LUBAN_EXIT%"
    goto :fail
)

REM gofmt (optional, only if Go toolchain is installed)
where go >nul 2>&1
if %errorlevel% equ 0 (
    gofmt -w "%GO_CODE_DIR%"
) else (
    echo [WARN] 'go' not in PATH, skip gofmt.
)

echo [SUCCESS] Go excels generated: %GO_OUT%
echo   code: %GO_CODE_DIR%
echo   data: %GO_DATA_DIR%
exit /b 0

REM 错误统一出口：双击运行时停留窗口以便查看日志。
REM 被其他脚本/自动化调用时可设置 GEN_GO_NO_PAUSE=1 跳过 pause，避免挂起。
:fail
REM Luban失败时EXITCODE已提前设置，这里只在未设置时取ERRORLEVEL。
if not defined EXITCODE set "EXITCODE=%ERRORLEVEL%"
if "%EXITCODE%"=="0" set "EXITCODE=1"
echo.
echo [FAILED] 失败步骤: %FAIL_STEP%，退出码=%EXITCODE%
if not defined FAIL_LOG (
    echo [FAILED] （此步骤无日志文件，看上方 [ERROR] 行即可。）
    goto :pause_exit
)
echo [FAILED] 完整日志: %FAIL_LOG%
if exist "%FAIL_LOG%" (
    echo.
    echo [HINT] 关键错误行（表名 / 字段 / Excel位置，取自日志关键字）:
    REM 只用 ASCII 关键字，避免 findstr 中文编码问题；type 全文里仍可见中文行。
    findstr /i /c:"error" /c:"exception" /c:"fail" /c:".xlsx" /c:"Sheet" /c:"Err:" /c:"cell:" /c:"=>" "%FAIL_LOG%" 2>nul
    echo.
    echo [HINT] 若上行为空，请直接看上面 type 出来的完整日志，或用记事本打开 %FAIL_LOG% 搜索 error。
)
:pause_exit
echo.
echo [FAILED] genGO.bat 执行失败，按任意键关闭窗口...
if not "%GEN_GO_NO_PAUSE%"=="1" pause >nul
exit /b %EXITCODE%
