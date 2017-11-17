GOOS=linux GOARCH=amd64 go build -o linux_hanabi main/main.go
rsync -rvz static templates linux_hanabi root@aarmaan.me:/root/
rm linux_hanabi
ssh root@aarmaan.me '(chmod +x linux_hanabi; ./linux_hanabi --addr 0.0.0.0:8001)'
