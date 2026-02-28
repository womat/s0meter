set GOARCH=arm
set GOOS=linux
go build -o ..\bin\s0meter ..\cmd\s0meter.go

set GOARCH=386
set GOOS=windows
go build -o ..\bin\s0meter.exe ..\cmd\s0meter.go