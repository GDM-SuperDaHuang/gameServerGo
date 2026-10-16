@echo off
setlocal

:: protoc.exe path
set "PROTOC_BIN=D:\Program Files\protoc-36.1-win64\bin\protoc.exe"

:: protoc-gen-go must be in PATH
set "PATH=%PATH%;C:\Users\Administrator\go\bin"

echo Current dir: %cd%
echo.

if not exist "protobuf\proto" (
    echo [ERROR] protobuf\proto not found
    pause
    exit /b 1
)

if not exist "%PROTOC_BIN%" (
    echo [ERROR] protoc not found: %PROTOC_BIN%
    pause
    exit /b 1
)

where protoc-gen-go >nul 2>nul
if errorlevel 1 (
    echo [ERROR] protoc-gen-go not in PATH
    pause
    exit /b 1
)

if exist "protobuf\pbGo" (
    rmdir /s /q protobuf\pbGo
)
mkdir protobuf\pbGo

echo Generating Go files...
for %%f in (protobuf\proto\*.proto) do (
    "%PROTOC_BIN%" --proto_path=protobuf\proto --go_out=protobuf/pbGo --go_opt=paths=source_relative "%%f"
    if errorlevel 1 (
        echo [ERROR] protoc failed: %%f
        pause
        exit /b 1
    )
)

echo Done!
dir protobuf\pbGo /s /b
pause
endlocal
