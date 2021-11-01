go build -ldflags="-s -w" -o agent.run  main.go
~/go/bin/goupx agent.run

rm dist.tar.gz
cp agent.run dist
cp config.json dist

tar cvf dist.tar dist
gzip dist.tar
