cd main/gated
GOOS=linux GOARCH=amd64 go build -o gated
cd ../../main/gate
go build -o gate
## GOOS=windows GOARCH=amd64 go build -o gate.exe