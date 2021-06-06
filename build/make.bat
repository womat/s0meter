set GOARCH=arm
set GOOS=linux
go build -o ..\bin\s0counter ..\cmd\s0counter.go ..\cmd\linux.go ..\cmd\mqtt.go

set GOARCH=386
set GOOS=windows
go build -o ..\bin\s0counter.exe ..\cmd\s0counter.go ..\cmd\windows.go ..\cmd\mqtt.go